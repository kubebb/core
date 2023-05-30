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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type condStatusTestCase struct {
	status, exp ConditionedStatus
	l           int
	cond        Condition
}

func TestUpdateCondwithFixedLen(t *testing.T) {
	now := metav1.Now()
	testCases := []condStatusTestCase{
		// l = 0
		{
			status: ConditionedStatus{},
			l:      0,
			exp: ConditionedStatus{
				Conditions: []Condition{{}},
			},
		},
		// l = 1 && conditions = 0
		{
			status: ConditionedStatus{},
			l:      1,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "",
				Message:            "",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
		},
		// l = 1 && conditions = 1
		{
			status: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
			l: 1,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "1",
				Message:            "1",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "1",
						Message:            "1",
					},
				},
			},
		},
		// l = 1, conditions = 2
		{
			status: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "1",
						Message:            "1",
					},
					{
						Status:             v1.ConditionFalse,
						Type:               TypeSynced,
						LastTransitionTime: now,
						Reason:             "2",
						Message:            "2",
					},
				},
			},
			l: 1,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "",
				Message:            "",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
		},

		// l = 2 && conditions = 0
		{
			status: ConditionedStatus{},
			l:      2,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "",
				Message:            "",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
		},

		// l = 2 && conditions = 1
		// 6
		{
			status: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
			l: 2,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "1",
				Message:            "1",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "1",
						Message:            "1",
					},
				},
			},
		},
		// l = 2, conditions = 2
		{
			status: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "1",
						Message:            "1",
					},
					{
						Status:             v1.ConditionFalse,
						Type:               TypeSynced,
						LastTransitionTime: now,
						Reason:             "2",
						Message:            "2",
					},
				},
			},
			l: 2,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "",
				Message:            "",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeSynced,
						LastTransitionTime: now,
						Reason:             "2",
						Message:            "2",
					},
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
		},
		// l =2 && conditions = 3
		{
			status: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionFalse,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "1",
						Message:            "1",
					},
					{
						Status:             v1.ConditionFalse,
						Type:               TypeSynced,
						LastTransitionTime: now,
						Reason:             "2",
						Message:            "2",
					},
					{
						Status:             v1.ConditionTrue,
						Type:               TypeFailedSync,
						LastTransitionTime: now,
						Reason:             "3",
						Message:            "3",
					},
				},
			},
			l: 2,
			cond: Condition{
				Status:             v1.ConditionTrue,
				Type:               TypeReady,
				LastTransitionTime: now,
				Reason:             "",
				Message:            "",
			},
			exp: ConditionedStatus{
				Conditions: []Condition{
					{
						Status:             v1.ConditionTrue,
						Type:               TypeFailedSync,
						LastTransitionTime: now,
						Reason:             "3",
						Message:            "3",
					},
					{
						Status:             v1.ConditionTrue,
						Type:               TypeReady,
						LastTransitionTime: now,
						Reason:             "",
						Message:            "",
					},
				},
			},
		},
	}
	for i, tc := range testCases {
		UpdateCondWithFixedLen(tc.l, &tc.status, tc.cond)
		if !reflect.DeepEqual(tc.exp, tc.status) {
			t.Fatalf("[%d] expect %v get %v", i, tc.exp, tc.status)
		}
	}
}

func TestMatch(t *testing.T) {
	testCases := []struct {
		description string
		name        string
		filterCond  map[string]FilterCond
		version     string
		deprecated  bool
		expected    bool
	}{
		{
			description: "filterCond is empty",
			name:        "test",
			filterCond:  nil,
			version:     "1.0.0",
			deprecated:  false,
			expected:    true,
		},
		{
			description: "name doesn't exist",
			name:        "test",
			filterCond:  map[string]FilterCond{"other": {}},
			version:     "1.0.0",
			deprecated:  false,
			expected:    true,
		},
		{
			description: "cond.Deprecated is false and deprecated is true",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Deprecated: false}},
			version:     "1.0.0",
			deprecated:  true,
			expected:    false,
		},
		{
			description: "version exists in cond.Versions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Versions: []string{"1.0.0"}}},
			version:     "1.0.0",
			deprecated:  false,
			expected:    true,
		},
		{
			description: "regular match success",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Regexp: `^1\.*0\.0$`}},
			version:     "1.0.0",
			deprecated:  false,
			expected:    true,
		},
		{
			description: "regular match failed",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Regexp: `^1\.*0\.0$`}},
			version:     "2.0.0",
			deprecated:  false,
			expected:    false,
		},
		{
			description: "version is larger than the value of the than field",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {VersionConstraint: ">= 1.0.0"}},
			version:     "2.0.0",
			deprecated:  false,
			expected:    true,
		},
		{
			description: "version is smaller than the value of the than field",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {VersionConstraint: ">= 2.0.0"}},
			version:     "1.0.0",
			deprecated:  false,
			expected:    false,
		},
		{
			description: "version is larger than the value of the than field and the less field is true",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {VersionConstraint: "<= 2.0.0"}},
			version:     "3.0.0",
			deprecated:  false,
			expected:    false,
		},
		{
			description: "version is smaller than the value of the than field and the less field is true",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {VersionConstraint: "<= 3.0.0"}},
			version:     "2.0.0",
			deprecated:  false,
			expected:    true,
		},
	}
	for _, testCase := range testCases {
		actual := Match(testCase.filterCond, Filter{Name: testCase.name, Version: testCase.version, Deprecated: testCase.deprecated})
		if actual != testCase.expected {
			t.Errorf("Test Failed: %s, expected: %t, actual: %t", testCase.description, testCase.expected, actual)
		}
	}
}

func TestIsCondSame(t *testing.T) {
	testCases := []struct {
		c1, c2 FilterCond
		exp    bool
	}{
		{
			c1:  FilterCond{},
			c2:  FilterCond{},
			exp: true,
		},
		{
			c1:  FilterCond{Deprecated: true},
			c2:  FilterCond{Deprecated: false},
			exp: false,
		},
		{
			c1:  FilterCond{VersionConstraint: ">= 1.0.0"},
			c2:  FilterCond{VersionConstraint: ">= 1.0.0"},
			exp: true,
		},
		{
			c1:  FilterCond{Versions: []string{"v1", "v1", "v2"}},
			c2:  FilterCond{Versions: []string{"v2"}},
			exp: false,
		},
		{
			c1:  FilterCond{Versions: []string{"v1", "v1", "v2"}},
			c2:  FilterCond{Versions: []string{"v1", "v2"}},
			exp: true,
		},
		{
			c1:  FilterCond{Versions: []string{"v1", "v1"}},
			c2:  FilterCond{Versions: []string{"v1"}},
			exp: true,
		},
	}
	for _, tc := range testCases {
		if r := IsCondSame(tc.c1, tc.c2); r != tc.exp {
			t.Fatalf("expect %v get %v", tc.exp, r)
		}
	}
}
