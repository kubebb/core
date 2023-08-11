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
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestConditionEqual tests Condition.Equal
func TestConditionEqual(t *testing.T) {
	testCases := []struct {
		description string
		name        string
		conditionA  Condition
		conditionB  Condition

		expected bool
	}{
		{
			description: "Two equivalent conditions",
			conditionA: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},
			conditionB: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},

			expected: true,
		},
		{
			description: "Two conditions with different types",
			conditionA: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},
			conditionB: Condition{
				Type:   TypeSynced,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},

			expected: false,
		},
		{
			description: "Two conditions with different status",
			conditionA: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},
			conditionB: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionFalse,
				Reason: ReasonAvailable,
			},

			expected: false,
		},
		{
			description: "Two conditions with different reasons",
			conditionA: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},
			conditionB: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonDeleting,
			},

			expected: false,
		},
		{
			description: "Two conditions with different time",
			conditionA: Condition{
				Type:               TypeReady,
				Status:             corev1.ConditionTrue,
				Reason:             ReasonAvailable,
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
			conditionB: Condition{
				Type:               TypeReady,
				Status:             corev1.ConditionTrue,
				Reason:             ReasonAvailable,
				LastTransitionTime: metav1.NewTime(time.Now()),
			},

			expected: false,
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.conditionA.Equal(testCase.conditionB), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.conditionA.Equal(testCase.conditionB))
		}
	}
}

// TestWithMessage tests Condition.WithMessage
func TestWithMessage(t *testing.T) {
	testCases := []struct {
		description string
		name        string
		condition   Condition
		msg         string

		expected Condition
	}{
		{
			description: "add a normal message",
			condition: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},
			msg: "A message",

			expected: Condition{
				Type:    TypeReady,
				Status:  corev1.ConditionTrue,
				Reason:  ReasonAvailable,
				Message: "A message",
			},
		},
		{
			description: "add an empty message",
			condition: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionTrue,
				Reason: ReasonAvailable,
			},
			msg: "",

			expected: Condition{
				Type:    TypeReady,
				Status:  corev1.ConditionTrue,
				Reason:  ReasonAvailable,
				Message: "",
			},
		},
		{
			description: "override a message",
			condition: Condition{
				Type:    TypeReady,
				Status:  corev1.ConditionTrue,
				Reason:  ReasonAvailable,
				Message: "A message",
			},
			msg: "A new message",

			expected: Condition{
				Type:    TypeReady,
				Status:  corev1.ConditionTrue,
				Reason:  ReasonAvailable,
				Message: "A new message",
			},
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.condition.WithMessage(testCase.msg), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.condition.WithMessage(testCase.msg))
		}
	}
}

// TestNewConditionedStatus tests NewConditionedStatus
func TestNewConditionedStatus(t *testing.T) {
	testCases := []struct {
		description string
		name        string
		conditions  []Condition

		expected *ConditionedStatus
	}{
		{
			description: "With a normal list of conditions",
			conditions: []Condition{
				Condition{
					Type:   TypeReady,
					Status: corev1.ConditionTrue,
					Reason: ReasonAvailable,
				},
				Condition{
					Type:   TypeSynced,
					Status: corev1.ConditionFalse,
					Reason: ReasonCreating,
				},
				Condition{
					Type:   TypeFailedSync,
					Status: corev1.ConditionFalse,
					Reason: ReasonDeleting,
				},
			},

			expected: &ConditionedStatus{
				[]Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonAvailable,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
		},
		{
			description: "With a list of conditions, containing duplicate types",
			conditions: []Condition{
				Condition{
					Type:   TypeReady,
					Status: corev1.ConditionTrue,
					Reason: ReasonAvailable,
				},
				Condition{
					Type:   TypeReady,
					Status: corev1.ConditionFalse,
					Reason: ReasonCreating,
				},
				Condition{
					Type:   TypeFailedSync,
					Status: corev1.ConditionFalse,
					Reason: ReasonDeleting,
				},
			},

			expected: &ConditionedStatus{
				[]Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
		},
		{
			description: "With list being nil",
			conditions:  nil,

			expected: &ConditionedStatus{
				Conditions: nil,
			},
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(NewConditionedStatus(testCase.conditions...), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, NewConditionedStatus(testCase.conditions...))
		}
	}
}

// TestGetCondition tests ConditionedStatus.GetCondition
func TestGetCondition(t *testing.T) {
	testCases := []struct {
		description       string
		conditionedStatus *ConditionedStatus
		conditionType     ConditionType

		expected Condition
	}{
		{
			description: "condition type exists",
			conditionedStatus: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditionType: TypeReady,

			expected: Condition{
				Type:   TypeReady,
				Status: corev1.ConditionFalse,
				Reason: ReasonCreating,
			},
		},
		{
			description: "condition type does not exist",
			conditionedStatus: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditionType: TypeSynced,

			expected: Condition{
				Type:   TypeSynced,
				Status: corev1.ConditionUnknown,
			},
		},
		{
			description: "condition type is a random string",
			conditionedStatus: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditionType: "abc",

			expected: Condition{
				Type:   "abc",
				Status: corev1.ConditionUnknown,
			},
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.conditionedStatus.GetCondition(testCase.conditionType), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.conditionedStatus.GetCondition(testCase.conditionType))
		}
	}
}

// TestSetConditions tests ConditionedStatus.SetConditions
func TestSetConditions(t *testing.T) {
	testCases := []struct {
		description       string
		conditionedStatus ConditionedStatus
		conditions        []Condition

		expected ConditionedStatus
	}{
		{
			description: "not changing anything",
			conditionedStatus: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditions: []Condition{
				Condition{
					Type:   TypeFailedSync,
					Status: corev1.ConditionFalse,
					Reason: ReasonDeleting,
				},
			},

			expected: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
		},
		{
			description: "changing a condition",
			conditionedStatus: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditions: []Condition{
				Condition{
					Type:   TypeReady,
					Status: corev1.ConditionTrue,
					Reason: ReasonCreating,
				},
			},

			expected: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
		},
		{
			description: "adding a condition",
			conditionedStatus: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditions: []Condition{
				Condition{
					Type:   TypeSynced,
					Status: corev1.ConditionFalse,
					Reason: ReasonCreating,
				},
			},

			expected: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
		},
		{
			description: "adding a condition and changing another one",
			conditionedStatus: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
				},
			},
			conditions: []Condition{
				Condition{
					Type:   TypeReady,
					Status: corev1.ConditionTrue,
					Reason: ReasonCreating,
				},
				Condition{
					Type:   TypeSynced,
					Status: corev1.ConditionFalse,
					Reason: ReasonCreating,
				},
			},

			expected: ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		testCase.conditionedStatus.SetConditions(testCase.conditions...)
		if !reflect.DeepEqual(testCase.conditionedStatus, testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.conditionedStatus)
		}
	}
}

// TestConditionedStatusEqual tests ConditionedStatus.Equal
func TestConditionedStatusEqual(t *testing.T) {
	testCases := []struct {
		description string
		csA         *ConditionedStatus
		csB         *ConditionedStatus

		expected bool
	}{
		{
			description: "Two equivalent conditioned status",
			csA: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
			csB: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},

			expected: true,
		},
		{
			description: "Two conditioned status with different conditions",
			csA: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
			csB: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionTrue,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},

			expected: false,
		},
		{
			description: "Two conditioned status with different number of conditions",
			csA: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
			csB: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},

			expected: false,
		},
		{
			description: "One conditioned status has a list being nil",
			csA: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
			csB: &ConditionedStatus{
				Conditions: nil,
			},

			expected: false,
		},
		{
			description: "Both conditioned status have a list being nil",
			csA: &ConditionedStatus{
				Conditions: nil,
			},
			csB: &ConditionedStatus{
				Conditions: nil,
			},

			expected: true,
		},
		{
			description: "One conditioned status is nil",
			csA: &ConditionedStatus{
				Conditions: []Condition{
					Condition{
						Type:   TypeReady,
						Status: corev1.ConditionTrue,
						Reason: ReasonCreating,
					},
					Condition{
						Type:   TypeFailedSync,
						Status: corev1.ConditionFalse,
						Reason: ReasonDeleting,
					},
					Condition{
						Type:   TypeSynced,
						Status: corev1.ConditionFalse,
						Reason: ReasonCreating,
					},
				},
			},
			csB: nil,

			expected: false,
		},
		{
			description: "Both conditioned status are nil",
			csA:         nil,
			csB:         nil,

			expected: true,
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.csA.Equal(testCase.csB), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.csA.Equal(testCase.csB))
		}
	}
}
