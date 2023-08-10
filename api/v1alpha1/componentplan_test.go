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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGenerateComponentPlanName for GenerateComponentPlanName
func TestGenerateComponentPlanName(t *testing.T) {
	testCases := []struct {
		sub     *Subscription
		version string

		expected string
	}{
		{
			sub: &Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Nginx",
				},
			},
			version: "15.1.0",

			expected: "sub.Nginx.15.1.0",
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(GenerateComponentPlanName(testCase.sub, testCase.version), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, GenerateComponentPlanName(testCase.sub, testCase.version))
		}
	}
}

// TestGenerateComponentPlanManifestConfigMapName for GenerateComponentPlanManifestConfigMapName
func TestGenerateComponentPlanManifestConfigMapName(t *testing.T) {
	testCases := []struct {
		plan *ComponentPlan

		expected string
	}{
		{
			plan: &ComponentPlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Nginx",
				},
			},

			expected: "manifest.Nginx",
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(GenerateComponentPlanManifestConfigMapName(testCase.plan), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, GenerateComponentPlanManifestConfigMapName(testCase.plan))
		}
	}
}

// IsEquivCondition returns whether two conditions are equivalent
func IsEquivCondition(conditionA Condition, conditionB Condition) bool {
	return conditionA.Type == conditionB.Type &&
		conditionA.Status == conditionB.Status &&
		conditionA.Reason == conditionB.Reason
}

// TestComponentPlanSucceeded for ComponentPlanSucceeded
func TestComponentPlanSucceeded(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeSucceeded,
				Status: corev1.ConditionTrue,
				Reason: "",
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanSucceeded(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanSucceeded())
		}
	}
}

// TestComponentPlanInitSucceeded for ComponentPlanInitSucceeded
func TestComponentPlanInitSucceeded(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{

			expected: Condition{
				Type:   ComponentPlanTypeSucceeded,
				Status: corev1.ConditionFalse,
				Reason: "",
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanInitSucceeded(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanInitSucceeded())
		}
	}
}

// TestComponentPlanFailed for ComponentPlanFailed
func TestComponentPlanFailed(t *testing.T) {
	testCases := []struct {
		err error

		expected Condition
	}{
		{
			err: errors.New("component plan failed"),

			expected: Condition{
				Type:   ComponentPlanTypeSucceeded,
				Status: corev1.ConditionFalse,
				Reason: "",
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanFailed(errors.New("component plan failed")), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanFailed(errors.New("component plan failed")))
		}
	}
}

// TestComponentPlanApproved for ComponentPlanApproved
func TestComponentPlanApproved(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeApproved,
				Status: corev1.ConditionTrue,
				Reason: "",
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanApproved(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanApproved())
		}
	}
}

// TestComponentPlanUnapproved for ComponentPlanUnapproved
func TestComponentPlanUnapproved(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeApproved,
				Status: corev1.ConditionFalse,
				Reason: "",
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUnapproved(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUnapproved())
		}
	}
}

// TestComponentPlanInstallSuccess for ComponentPlanInstallSuccess
func TestComponentPlanInstallSuccess(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionTrue,
				Reason: ComponentPlanReasonInstallSuccess,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanInstallSuccess(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanInstallSuccess())
		}
	}
}

// TestComponentPlanInstallFailed for ComponentPlanInstallFailed
func TestComponentPlanInstallFailed(t *testing.T) {
	testCases := []struct {
		err error

		expected Condition
	}{
		{
			err: errors.New("component plan install failed"),

			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonInstallFailed,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanInstallFailed(errors.New("component plan install failed")), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanInstallFailed(errors.New("component plan install failed")))
		}
	}
}

// TestComponentPlanInstalling for ComponentPlanInstalling
func TestComponentPlanInstalling(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{

			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonInstalling,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanInstalling(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanInstalling())
		}
	}
}

// TestComponentPlanUninstallSuccess for ComponentPlanUninstallSuccess
func TestComponentPlanUninstallSuccess(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{

			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionTrue,
				Reason: ComponentPlanReasonUninstallSuccess,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUninstallSuccess(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUninstallSuccess())
		}
	}
}

// TestComponentPlanUninstallFailed for ComponentPlanUninstallFailed
func TestComponentPlanUninstallFailed(t *testing.T) {
	testCases := []struct {
		err error

		expected Condition
	}{
		{
			err: errors.New("component plan uninstall failed"),

			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonUninstallFailed,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUninstallFailed(errors.New("component plan uninstall failed")), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUninstallFailed(errors.New("component plan uninstall failed")))
		}
	}
}

// TestComponentPlanUninstalling for ComponentPlanUninstalling
func TestComponentPlanUninstalling(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonUninstalling,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUninstalling(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUninstalling())
		}
	}
}

// TestComponentPlanUpgradeSuccess for ComponentPlanUpgradeSuccess
func TestComponentPlanUpgradeSuccess(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionTrue,
				Reason: ComponentPlanReasonUpgradeSuccess,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUpgradeSuccess(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUpgradeSuccess())
		}
	}
}

// TestComponentPlanUpgradeFailed for ComponentPlanUpgradeFailed
func TestComponentPlanUpgradeFailed(t *testing.T) {
	testCases := []struct {
		err error

		expected Condition
	}{
		{
			err: errors.New("component plan upgrade failed"),

			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonUpgradeFailed,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUpgradeFailed(errors.New("component plan upgrade failed")), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUpgradeFailed(errors.New("component plan upgrade failed")))
		}
	}
}

// TestComponentPlanUpgrading for ComponentPlanUpgrading
func TestComponentPlanUpgrading(t *testing.T) {
	testCases := []struct {
		expected Condition
	}{
		{
			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonUpgrading,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanUpgrading(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanUpgrading())
		}
	}
}

// TestComponentPlanWaitDo for ComponentPlanWaitDo
func TestComponentPlanWaitDo(t *testing.T) {
	testCases := []struct {
		err error

		expected Condition
	}{
		{
			err: errors.New("component plan wait do"),

			expected: Condition{
				Type:   ComponentPlanTypeActioned,
				Status: corev1.ConditionFalse,
				Reason: ComponentPlanReasonWaitDo,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(ComponentPlanWaitDo(errors.New("component plan wait do")), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, ComponentPlanWaitDo(errors.New("component plan wait do")))
		}
	}
}

// TestComponentPlanCondition for componentPlanCondition
func TestComponentPlanCondition(t *testing.T) {
	testCases := []struct {
		description string
		ct          ConditionType
		reason      ConditionReason
		status      corev1.ConditionStatus
		err         error

		expected Condition
	}{
		{
			description: "Test the untested condition: status=\"\"",
			ct:          ComponentPlanTypeSucceeded,
			reason:      ComponentPlanReasonUpgradeSuccess,
			status:      "",
			err:         nil,

			expected: Condition{
				Type:   ComponentPlanTypeSucceeded,
				Reason: ComponentPlanReasonUpgradeSuccess,
				Status: corev1.ConditionUnknown,
			},
		},
	}
	for _, testCase := range testCases {
		if !IsEquivCondition(componentPlanCondition(testCase.ct, testCase.reason, testCase.status, testCase.err), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, componentPlanCondition(testCase.ct, testCase.reason, testCase.status, testCase.err))
		}
	}
}

// TestInitCondition for ComponentPlan.InitCondition
func TestInitCondition(t *testing.T) {
	testCases := []struct {
		description string
		cp          ComponentPlan

		expected []Condition
	}{
		{
			description: "Component plan approved",
			cp: ComponentPlan{
				Spec: ComponentPlanSpec{
					Approved: true,
				},
			},

			expected: []Condition{
				{
					Type:   ComponentPlanTypeApproved,
					Status: corev1.ConditionTrue,
					Reason: "",
				},
				{
					Type:   ComponentPlanTypeActioned,
					Status: corev1.ConditionFalse,
					Reason: ComponentPlanReasonWaitDo,
				}, {
					Type:   ComponentPlanTypeSucceeded,
					Status: corev1.ConditionFalse,
					Reason: "",
				},
			},
		},
		{
			description: "Component plan unapproved",
			cp: ComponentPlan{
				Spec: ComponentPlanSpec{
					Approved: false,
				},
			},

			expected: []Condition{
				{
					Type:   ComponentPlanTypeApproved,
					Status: corev1.ConditionFalse,
					Reason: "",
				},
				{
					Type:   ComponentPlanTypeActioned,
					Status: corev1.ConditionFalse,
					Reason: ComponentPlanReasonWaitDo,
				}, {
					Type:   ComponentPlanTypeSucceeded,
					Status: corev1.ConditionFalse,
					Reason: "",
				},
			},
		},
	}
	for _, testCase := range testCases {
		result := testCase.cp.InitCondition()
		for i, condition := range result {
			if !IsEquivCondition(condition, testCase.expected[i]) {
				t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, condition)
			}
		}
	}
}

// TestIsActionedReason for ComponentPlan.IsActionedReason
func TestIsActionedReason(t *testing.T) {
	testCases := []struct {
		description     string
		plan            *ComponentPlan
		ConditionReason ConditionReason

		expected bool
	}{
		{
			description: "The actioned reason is correct",
			plan: &ComponentPlan{
				Status: ComponentPlanStatus{
					ConditionedStatus: ConditionedStatus{
						Conditions: []Condition{
							{
								Type:               ComponentPlanTypeActioned,
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Now(),
								Reason:             ReasonAvailable,
							},
						},
					},
				},
			},
			ConditionReason: ReasonAvailable,

			expected: true,
		},
		{
			description: "Get in actioned reason",
			plan: &ComponentPlan{
				Status: ComponentPlanStatus{
					ConditionedStatus: ConditionedStatus{
						Conditions: []Condition{
							{
								Type:               ComponentPlanTypeActioned,
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Now(),
								Reason:             ReasonAvailable,
							},
						},
					},
				},
			},
			ConditionReason: ReasonCreating,

			expected: false,
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.plan.IsActionedReason(testCase.ConditionReason), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, testCase.plan.GetReleaseName())
		}
	}
}

// TestGetReleaseName for ComponentPlan.GetReleaseName
func TestGetReleaseName(t *testing.T) {
	testCases := []struct {
		plan *ComponentPlan

		expected string
	}{
		{
			plan: &ComponentPlan{
				Spec: ComponentPlanSpec{
					Config: Config{
						Name: "Nginx",
					},
				},
			},

			expected: "Nginx",
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.plan.GetReleaseName(), testCase.expected) {
			t.Fatalf("Test Failed, expected: %v, actual: %v", testCase.expected, testCase.plan.GetReleaseName())
		}
	}
}
