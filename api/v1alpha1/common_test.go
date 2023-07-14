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

	"helm.sh/helm/v3/pkg/chart"
	hrepo "helm.sh/helm/v3/pkg/repo"
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
		versions    []*hrepo.ChartVersion

		keep     bool
		expected []int
	}{
		{
			description: "filterCond is empty and should retain all",
			name:        "test",
			filterCond:  nil,
			versions:    []*hrepo.ChartVersion{{Metadata: &chart.Metadata{Version: "1.0.0"}}},
			keep:        true,
			expected:    []int{0},
		},
		{
			description: "the filter is not empty, but the chart package is not in the filter, so keep it",
			name:        "test",
			filterCond:  map[string]FilterCond{"other": {}},
			keep:        true,
			expected:    nil,
		},
		{
			description: "there is no version filtering information to determine if the chart package is retained or not",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Deprecated: false, Operation: FilterOpKeep}},
			versions:    []*hrepo.ChartVersion{{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}}},
			keep:        true,
			expected:    []int{0},
		},
		{
			description: "there is no version filtering information to determine if the chart package is retained or not",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Deprecated: false, Operation: FilterOpIgnore}},
			versions:    []*hrepo.ChartVersion{{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}}},
			keep:        false,
			expected:    nil,
		},
		// keep,accurate
		{
			description: "accurately matches versions. operation: keep, deprecated: true",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{
				Versions: []string{"1.0.0", "2.0.0"},
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{0, 1},
		},
		{
			description: "accurately matches versions. operation: keep, deprecated: true, no conditions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "accurately matches versions. operation: keep, deprecated: false",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				Versions: []string{"1.0.0", "2.0.0"},
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{1},
		},
		{
			description: "accurately matches versions. operation: keep, deprecated: false, no conditions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},

		// keep,regexp
		{
			description: "regexp matches versions. operation: keep, deprecated: true",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{0, 1, 2},
		},
		{
			description: "regexp matches versions. operation: keep, deprecated: false",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{1, 2},
		},
		{
			description: "regexp matches versions. operation: keep, deprecated: false, all deprecated",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "regexp matches versions. operation: keep, deprecated: false, version too big",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "11.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "21.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "31.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},

		// keep, semvar
		{
			description: "semvar matches versions. operation: keep, deprecated: true",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{
				VersionConstraint: ">= 2.0.0",
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{1, 2},
		},
		{
			description: "semvar matches versions. operation: keep, deprecated: false",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionConstraint: ">= 2.0.0",
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{2},
		},
		{
			description: "semvar matches versions. operation: keep, deprecated: false, all deprecated",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionConstraint: ">= 1.0.0",
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "semvar matches versions. operation: keep, deprecated: false, no conditions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Operation: FilterOpKeep, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},

		// ignore, accurately
		{
			description: "accurately matches versions. operation: ignore, deprecated: true",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{
				Versions: []string{"1.0.0", "2.0.0"},
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{2},
		},
		{
			description: "accurately matches versions. operation: ignore, deprecated: true, no conditions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{0, 1, 2},
		},
		{
			description: "accurately matches versions. operation: ignore, deprecated: false",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				Versions: []string{"1.0.0", "2.0.0"},
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{2},
		},
		{
			description: "accurately matches versions. operation: ignore, deprecated: true, no conditions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},

		// ignore, regexp
		{
			description: "regexp matches versions. operation: ignore, deprecated: true",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "regexp matches versions. operation: ignore, deprecated: false",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: false}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "regexp matches versions. operation: ignore, deprecated: false, all deprecated",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "regexp matches versions. operation: ignore, deprecated: true, too big version",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionRegexp: `^\d\.*0\.0$`,
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "11.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "21.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "31.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},

		// ignore, semvar
		{
			description: "semvar matches versions. operation: ignore, deprecated: true",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: true, VersionedFilterCond: &VersionedFilterCond{
				VersionConstraint: ">= 2.0.0",
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     true,
			expected: []int{0},
		},
		{
			description: "semvar matches versions. operation: ignore, deprecated: false",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionConstraint: ">= 2.0.0",
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: false}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "semvar matches versions. operation: ignore, deprecated: true, all deprecated",
			name:        "test",
			filterCond: map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{
				VersionConstraint: ">= 1.0.0",
			}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},
		{
			description: "semvar matches versions. operation: ignore, deprecated: true, no conditions",
			name:        "test",
			filterCond:  map[string]FilterCond{"test": {Operation: FilterOpIgnore, Deprecated: false, VersionedFilterCond: &VersionedFilterCond{}}},
			versions: []*hrepo.ChartVersion{
				{Metadata: &chart.Metadata{Version: "1.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "2.0.0", Deprecated: true}},
				{Metadata: &chart.Metadata{Version: "3.0.0", Deprecated: true}},
			},
			keep:     false,
			expected: nil,
		},
	}
	for _, testCase := range testCases {
		indices, keep := Match(testCase.filterCond, Filter{Name: testCase.name, Versions: testCase.versions})
		if !reflect.DeepEqual(indices, testCase.expected) || keep != testCase.keep {
			t.Errorf("Test Failed: %s, expected: %v %v, actual: %v %v", testCase.description, testCase.expected, testCase.keep, indices, keep)
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
			c1:  FilterCond{VersionedFilterCond: &VersionedFilterCond{VersionConstraint: ">= 1.0.0"}},
			c2:  FilterCond{VersionedFilterCond: &VersionedFilterCond{VersionConstraint: ">= 1.0.0"}},
			exp: true,
		},
		{
			c1:  FilterCond{VersionedFilterCond: &VersionedFilterCond{Versions: []string{"v1", "v1", "v2"}}},
			c2:  FilterCond{VersionedFilterCond: &VersionedFilterCond{Versions: []string{"v2"}}},
			exp: false,
		},
		{
			c1:  FilterCond{VersionedFilterCond: &VersionedFilterCond{Versions: []string{"v1", "v1", "v2"}}},
			c2:  FilterCond{VersionedFilterCond: &VersionedFilterCond{Versions: []string{"v1", "v2"}}},
			exp: true,
		},
		{
			c1:  FilterCond{VersionedFilterCond: &VersionedFilterCond{Versions: []string{"v1", "v1"}}},
			c2:  FilterCond{VersionedFilterCond: &VersionedFilterCond{Versions: []string{"v1"}}},
			exp: true,
		},
	}
	for i, tc := range testCases {
		if r := IsCondSame(tc.c1, tc.c2); r != tc.exp {
			t.Fatalf("expect %v get %v c1: %d", tc.exp, r, i)
		}
	}
}
