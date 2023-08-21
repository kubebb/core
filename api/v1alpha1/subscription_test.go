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
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestIsAuto for InstallMethod.IsAuto
func TestIsAuto(t *testing.T) {
	im := InstallMethod("other")
	if im.IsAuto() {
		t.Fatal("Test Failed, expect false get true")
	}
	im = InstallMethodAuto
	if !im.IsAuto() {
		t.Fatal("Test Failed, expect true get false")
	}
}

func TestSubscriptionAvailable(t *testing.T) {
	expCond := Condition{
		Type:   SubscriptionTypeReady,
		Status: v1.ConditionTrue,
		Reason: SubscriptionReasonAvailable,
	}
	if r := SubscriptionAvailable(); r.Type != expCond.Type || r.Status != expCond.Status || r.Reason != expCond.Reason {
		t.Fatalf("Test Failed, expect %v get %v", expCond, r)
	}
}

func TestSubscriptionReconcileSuccess(t *testing.T) {
	cond := Condition{
		Type: SubscriptionTypeReady,
	}
	expCond := Condition{
		Type:   cond.Type,
		Status: v1.ConditionTrue,
		Reason: SubscriptionReasonReconcileSuccess,
	}
	if r := SubscriptionReconcileSuccess(cond.Type); r.Type != expCond.Type || r.Status != expCond.Status || r.Reason != expCond.Reason {
		t.Fatalf("Test Failed, expect %v get %v", expCond, r)
	}
}

func TestSubscriptionReconcileError(t *testing.T) {
	cond := Condition{
		Type: SubscriptionTypeReady,
	}

	err := errors.New("index out of range")
	expCond := Condition{
		Type:    cond.Type,
		Status:  v1.ConditionFalse,
		Reason:  SubscriptionReasonReconcileError,
		Message: err.Error(),
	}
	if r := SubscriptionReconcileError(cond.Type, err); r.Type != expCond.Type || r.Status != expCond.Status || r.Reason != expCond.Reason || r.Message != expCond.Message {
		t.Fatalf("Test Failed, expect %v get %v", expCond, r)
	}
}

// inspire by https://github.com/kubernetes/kubernetes/blob/2fe38f93e53201b0c9e58aa6e3d37b8a61d2ca23/pkg/controller/cronjob/utils_test.go#L353
func TestMostRecentScheduleTime(t *testing.T) {
	metav1TopOfTheHour := metav1.NewTime(*topOfTheHour())

	tests := []struct {
		name                  string
		sub                   *Subscription
		includeSDS            bool
		now                   time.Time
		expectedEarliestTime  time.Time
		expectedRecentTime    *time.Time
		expectedTooManyMissed bool
		wantErr               bool
	}{
		{
			name: "now before next schedule",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1TopOfTheHour,
				},
				Spec: SubscriptionSpec{
					Schedule: "0 * * * *",
				},
			},
			now:                  topOfTheHour().Add(30 * time.Second),
			expectedRecentTime:   nil,
			expectedEarliestTime: *topOfTheHour(),
		},
		{
			name: "now just after next schedule",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1TopOfTheHour,
				},
				Spec: SubscriptionSpec{
					Schedule: "0 * * * *",
				},
			},
			now:                  topOfTheHour().Add(61 * time.Minute),
			expectedRecentTime:   deltaTimeAfterTopOfTheHour(60 * time.Minute),
			expectedEarliestTime: *topOfTheHour(),
		},
		{
			name: "missed 5 schedules",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(*deltaTimeAfterTopOfTheHour(10 * time.Second)),
				},
				Spec: SubscriptionSpec{
					Schedule: "0 * * * *",
				},
			},
			now:                  *deltaTimeAfterTopOfTheHour(301 * time.Minute),
			expectedRecentTime:   deltaTimeAfterTopOfTheHour(300 * time.Minute),
			expectedEarliestTime: *deltaTimeAfterTopOfTheHour(10 * time.Second),
		},
		{
			name: "complex schedule",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1TopOfTheHour,
				},
				Spec: SubscriptionSpec{
					Schedule: "30 6-16/4 * * 1-5",
				},
			},
			now:                  *deltaTimeAfterTopOfTheHour(24*time.Hour + 31*time.Minute),
			expectedRecentTime:   deltaTimeAfterTopOfTheHour(24*time.Hour + 30*time.Minute),
			expectedEarliestTime: *topOfTheHour(),
		},
		{
			name: "another complex schedule",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1TopOfTheHour,
				},
				Spec: SubscriptionSpec{
					Schedule: "30 10,11,12 * * 1-5",
				},
			},
			now:                  *deltaTimeAfterTopOfTheHour(30*time.Hour + 30*time.Minute),
			expectedRecentTime:   nil,
			expectedEarliestTime: *topOfTheHour(),
		},
		{
			name: "complex schedule with longer diff between executions",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1TopOfTheHour,
				},
				Spec: SubscriptionSpec{
					Schedule: "30 6-16/4 * * 1-5",
				},
			},
			now:                  *deltaTimeAfterTopOfTheHour(96*time.Hour + 31*time.Minute),
			expectedRecentTime:   deltaTimeAfterTopOfTheHour(96*time.Hour + 30*time.Minute),
			expectedEarliestTime: *topOfTheHour(),
		},
		{
			name: "complex schedule with shorter diff between executions",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1TopOfTheHour,
				},
				Spec: SubscriptionSpec{
					Schedule: "30 6-16/4 * * 1-5",
				},
			},
			now:                  *deltaTimeAfterTopOfTheHour(24*time.Hour + 31*time.Minute),
			expectedRecentTime:   deltaTimeAfterTopOfTheHour(24*time.Hour + 30*time.Minute),
			expectedEarliestTime: *topOfTheHour(),
		},
		{
			name: "@every schedule",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(*deltaTimeAfterTopOfTheHour(-59 * time.Minute)),
				},
				Spec: SubscriptionSpec{
					Schedule: "@every 1h",
				},
			},
			now:                   *deltaTimeAfterTopOfTheHour(7 * 24 * time.Hour),
			expectedRecentTime:    deltaTimeAfterTopOfTheHour((6 * 24 * time.Hour) + 23*time.Hour + 1*time.Minute),
			expectedEarliestTime:  *deltaTimeAfterTopOfTheHour(-59 * time.Minute),
			expectedTooManyMissed: true,
		},
		{
			name: "rogue cronjob",
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(*deltaTimeAfterTopOfTheHour(10 * time.Second)),
				},
				Spec: SubscriptionSpec{
					Schedule: "59 23 31 2 *",
				},
			},
			now:                *deltaTimeAfterTopOfTheHour(1 * time.Hour),
			expectedRecentTime: nil,
			wantErr:            true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := cron.ParseStandard(tt.sub.Spec.Schedule)
			if err != nil {
				t.Errorf("error setting up the test, %s", err)
			}
			gotEarliestTime, gotRecentTime, gotTooManyMissed, err := tt.sub.MostRecentScheduleTime(tt.now, sched)
			if tt.wantErr {
				if err == nil {
					t.Error("mostRecentScheduleTime() got no error when expected one")
				}
				return
			}
			if !tt.wantErr && err != nil {
				t.Error("mostRecentScheduleTime() got error when none expected")
			}
			if gotEarliestTime.IsZero() {
				t.Errorf("earliestTime should never be 0, want %v", tt.expectedEarliestTime)
			}
			if !gotEarliestTime.Equal(tt.expectedEarliestTime) {
				t.Errorf("expectedEarliestTime - got %v, want %v", gotEarliestTime, tt.expectedEarliestTime)
			}
			if !reflect.DeepEqual(gotRecentTime, tt.expectedRecentTime) {
				t.Errorf("expectedRecentTime - got %v, want %v", gotRecentTime, tt.expectedRecentTime)
			}
			if gotTooManyMissed != tt.expectedTooManyMissed {
				t.Errorf("expectedNumberOfMisses - got %v, want %v", gotTooManyMissed, tt.expectedTooManyMissed)
			}
		})
	}
}

func topOfTheHour() *time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T10:00:00Z")
	if err != nil {
		panic("test setup error")
	}
	return &T1
}

func deltaTimeAfterTopOfTheHour(duration time.Duration) *time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T10:00:00Z")
	if err != nil {
		panic("test setup error")
	}
	t := T1.Add(duration)
	return &t
}
