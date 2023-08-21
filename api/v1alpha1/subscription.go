/*
Copyright 2023 The Kubebb Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SubscriptionNameLabel = Group + "/subscription-name"
)

// IsAuto returns true if InstallMethod is Auto, case-insensitivity.
func (i *InstallMethod) IsAuto() bool {
	return strings.EqualFold(string(*i), string(InstallMethodAuto))
}

// ConditionType for Subscription
const (
	// SubscriptionTypeReady indicates that the subscription is ready to use
	SubscriptionTypeReady = TypeReady
	// SubscriptionTypeSourceSynced indicates that the component and the repository are synced
	SubscriptionTypeSourceSynced ConditionType = "SourceSynced"
	// SubscriptionTypePlanSynce indicates that the componentplan is synced
	SubscriptionTypePlanSynce ConditionType = "PlanSynced"
)

// Condition resons for Subscription
const (
	SubscriptionReasonAvailable        = ReasonAvailable
	SubscriptionReasonUnavailable      = ReasonUnavailable
	SubscriptionReasonReconcileSuccess = ReasonReconcileSuccess
	SubscriptionReasonReconcileError   = ReasonReconcileError
)

// SubscriptionAvailable returns a condition that indicates the subscription is
// currently observed to be available for use.
func SubscriptionAvailable() Condition {
	return Condition{
		Type:               SubscriptionTypeReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             SubscriptionReasonAvailable,
	}
}

// SubscriptionUnavailable returns a condition that indicates the subscription is
// currently observed to be unavailable for use.
func SubscriptionUnavailable() Condition {
	return Condition{
		Type:               SubscriptionTypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             SubscriptionReasonUnavailable,
	}
}

// SubscriptionReconcileSuccess returns a condition indicating that controller successfully
// completed the most recent reconciliation of the subscription.
func SubscriptionReconcileSuccess(ct ConditionType) Condition {
	return Condition{
		Type:               ct,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             SubscriptionReasonReconcileSuccess,
	}
}

// SubscriptionReconcileError returns a condition indicating that controller encountered an
// error while reconciling the subscription. This could mean controller was
// unable to update the resource to reflect its desired state, or that
// controller was unable to determine the current actual state of the subscription.
func SubscriptionReconcileError(ct ConditionType, err error) Condition {
	return Condition{
		Type:               ct,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             SubscriptionReasonReconcileError,
		Message:            err.Error(),
	}
}

// MostRecentScheduleTime returns:
//   - CronJob's creation time,
//   - the most recent time a Job should be created or nil, if that's after now,
//   - boolean indicating an excessive number of missed schedules,
//   - error in an edge case where the schedule specification is grammatically correct,
//     but logically doesn't make sense (31st day for months with only 30 days, for example).
//
// inspire by https://github.com/kubernetes/kubernetes/blob/2fe38f93e53201b0c9e58aa6e3d37b8a61d2ca23/pkg/controller/cronjob/utils.go#L81
func (r *Subscription) MostRecentScheduleTime(now time.Time, schedule cron.Schedule) (time.Time, *time.Time, bool, error) {
	earliestTime := r.ObjectMeta.CreationTimestamp.Time
	t1 := schedule.Next(earliestTime)
	t2 := schedule.Next(t1)
	if now.Before(t1) {
		// now  t1
		return earliestTime, nil, false, nil
	}
	if now.Before(t2) {
		// t1 now t2
		return earliestTime, &t1, false, nil
	}

	// It is possible for cron.ParseStandard("59 23 31 2 *") to return an invalid schedule
	// minute - 59, hour - 23, dom - 31, month - 2, and dow is optional, clearly 31 is invalid
	// In this case the timeBetweenTwoSchedules will be 0, and we error out the invalid schedule
	timeBetweenTwoSchedules := int64(t2.Sub(t1).Round(time.Second).Seconds())
	if timeBetweenTwoSchedules < 1 {
		return earliestTime, nil, false, fmt.Errorf("time difference between two schedules is less than 1 second")
	}
	// this logic used for calculating number of missed schedules does a rough
	// approximation, by calculating a diff between two schedules (t1 and t2),
	// and counting how many of these will fit in between last schedule and now
	timeElapsed := int64(now.Sub(t1).Seconds())
	numberOfMissedSchedules := (timeElapsed / timeBetweenTwoSchedules) + 1

	var mostRecentTime time.Time
	// to get the most recent time accurate for regular schedules and the ones
	// specified with @every form, we first need to calculate the potential earliest
	// time by multiplying the initial number of missed schedules by its interval,
	// this is critical to ensure @every starts at the correct time, this explains
	// the numberOfMissedSchedules-1, the additional -1 serves there to go back
	// in time one more time unit, and let the cron library calculate a proper
	// schedule, for case where the schedule is not consistent, for example
	// something like  30 6-16/4 * * 1-5
	potentialEarliest := t1.Add(time.Duration((numberOfMissedSchedules-1-1)*timeBetweenTwoSchedules) * time.Second)
	for t := schedule.Next(potentialEarliest); !t.After(now); t = schedule.Next(t) {
		mostRecentTime = t
	}

	// An object might miss several starts. For example, if
	// controller gets wedged on friday at 5:01pm when everyone has
	// gone home, and someone comes in on tuesday AM and discovers
	// the problem and restarts the controller, then all the hourly
	// jobs, more than 80 of them for one hourly cronJob, should
	// all start running with no further intervention (if the cronJob
	// allows concurrency and late starts).
	//
	// However, if there is a bug somewhere, or incorrect clock
	// on controller's server or apiservers (for setting creationTimestamp)
	// then there could be so many missed start times (it could be off
	// by decades or more), that it would eat up all the CPU and memory
	// of this controller. In that case, we want to not try to list
	// all the missed start times.
	//
	// I've somewhat arbitrarily picked 100, as more than 80,
	// but less than "lots".
	tooManyMissed := numberOfMissedSchedules > 100

	if mostRecentTime.IsZero() {
		return earliestTime, nil, tooManyMissed, nil
	}
	return earliestTime, &mostRecentTime, tooManyMissed, nil
}
