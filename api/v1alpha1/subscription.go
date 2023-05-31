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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// SubscriptionUnavaliable returns a condition that indicates the subscription is
// currently observed to be unavaliable for use.
func SubscriptionUnavaliable() Condition {
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
