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
	"testing"

	v1 "k8s.io/api/core/v1"
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
