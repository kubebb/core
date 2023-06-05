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

package utils

import (
	"reflect"
	"testing"
)

func TestAddString(t *testing.T) {
	type testCase struct {
		source []string
		add    string
		exp    []string
	}
	for _, tc := range []testCase{
		{nil, "abc", []string{"abc"}},
		{[]string{}, "abc", []string{"abc"}},
		{[]string{"abc"}, "abc", []string{"abc"}},
		{[]string{"abc"}, "def", []string{"abc", "def"}},
	} {
		if r := AddString(tc.source, tc.add); !reflect.DeepEqual(tc.exp, r) {
			t.Fatalf("expect %v get %v", tc.exp, r)
		}
	}
}

func TestRemoveString(t *testing.T) {
	type testCase struct {
		source []string
		remove string
		exp    []string
	}
	for _, tc := range []testCase{
		{nil, "abc", nil},
		{[]string{}, "abc", []string{}},
		{[]string{"abc"}, "def", []string{"abc"}},
		{[]string{"abc", "abc"}, "abc", []string{}},
		{[]string{"abc", "def"}, "def", []string{"abc"}},
		{[]string{"abc", "def", "abc"}, "abc", []string{"def"}},
		{[]string{"abc", "def"}, "l", []string{"abc", "def"}},
	} {
		if r := RemoveString(tc.source, tc.remove); !reflect.DeepEqual(tc.exp, r) {
			t.Fatalf("expect %v get %v", tc.exp, r)
		}
	}
}

func TestContainString(t *testing.T) {
	type testCase struct {
		source []string
		f      string
		exp    bool
	}
	for _, tc := range []testCase{
		{nil, "", false},
		{[]string{}, "abc", false},
		{[]string{"abc"}, "def", false},
		{[]string{"abc"}, "abc", true},
		{[]string{"abc", "def"}, "l", false},
	} {
		if r := ContainString(tc.source, tc.f); r != tc.exp {
			t.Fatalf("expect %v get %v", tc.exp, r)
		}
	}
}

func TestAddOrSwapString(t *testing.T) {
	type testCase struct {
		source  []string
		str     string
		return1 bool
		return2 []string
	}
	for _, tc := range []testCase{
		{nil, "a", true, []string{"a"}},
		{[]string{"a"}, "b", true, []string{"a", "b"}},
		{[]string{"a", "b"}, "a", true, []string{"b", "a"}},
		{[]string{"a", "b", "a"}, "a", false, []string{"a", "b", "a"}},
	} {
		if swap, list := AddOrSwapString(tc.source, tc.str); swap != tc.return1 || !reflect.DeepEqual(list, tc.return2) {
			t.Fatalf("expect %v %v get %v %v with input source: %v, str: %v", tc.return1, tc.return2, swap, list, tc.source, tc.str)
		}
	}
}
