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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"reflect"
	"testing"
)

// TestNamespacedName tests Repository.NamespacedName
func TestNamespacedName(t *testing.T) {
	testCases := []struct {
		description string
		name        string
		repo        *Repository

		expected string
	}{
		{
			description: "test bitnami namespaced name",
			name:        "test",
			repo: &Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repository-bitnami-sample",
					Namespace: "kubebb-system",
				},
			},
			expected: "kubebb-system.repository-bitnami-sample",
		},
		{
			description: "test empty namespaced name",
			name:        "test",
			repo: &Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "",
				},
			},
			expected: ".",
		},
		{
			description: "test empty name",
			name:        "test",
			repo: &Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "kubebb-system",
				},
			},
			expected: "kubebb-system.",
		},
		{
			description: "test empty namespace",
			name:        "test",
			repo: &Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repository-bitnami-sample",
					Namespace: "",
				},
			},
			expected: ".repository-bitnami-sample",
		},
	}
	for _, testCase := range testCases {
		if !reflect.DeepEqual(testCase.repo.NamespacedName(), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.repo.NamespacedName())
		}
	}
}

// TestIsPullStrategySame tests IsPullStrategySame
func TestIsPullStrategySame(t *testing.T) {
	testCases := []struct {
		description string
		pullA       *PullStategy
		pullB       *PullStategy

		expected bool
	}{
		{
			description: "two equivalent pull strategies",
			pullA: &PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},
			pullB: &PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},

			expected: true,
		},
		{
			description: "two pull strategies with different retries",
			pullA: &PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},
			pullB: &PullStategy{
				IntervalSeconds: 120,
				Retry:           7,
			},

			expected: false,
		},
		{
			description: "two pull strategies with different interval seconds",
			pullA: &PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},
			pullB: &PullStategy{
				IntervalSeconds: 240,
				Retry:           5,
			},

			expected: false,
		},
		{
			description: "two pull strategies with different values",
			pullA: &PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},
			pullB: &PullStategy{
				IntervalSeconds: 240,
				Retry:           7,
			},

			expected: false,
		},
		{
			description: "one pull strategy is nil",
			pullA: &PullStategy{
				IntervalSeconds: 120,
				TimeoutSeconds:  600,
			},
			pullB: nil,

			expected: false,
		},
		{
			description: "both pull strategies are nil",
			pullA:       nil,
			pullB:       nil,

			expected: true,
		},
	}
	for _, testCase := range testCases {
		result := IsPullStrategySame(testCase.pullA, testCase.pullB)
		if !reflect.DeepEqual(result, testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, result)
		}
	}
}

// TestGetImageOverridePath tests GetImageOverridePath
func TestGetImageOverridePath(t *testing.T) {
	testCases := []struct {
		description string
		env         string

		expected []string
	}{
		{
			description: "Test with an empty environment",
			env:         "",

			expected: []string{"spec/containers/image",
				"spec/initContainers/image",
				"spec/template/spec/containers/image",
				"spec/template/spec/initContainers/image"},
		},
		{
			description: "Test with a nonempty environment",
			env:         "spec/initContainers/images:spec/notExisting/path:spec/andAnother/one",

			expected: []string{"spec/initContainers/images",
				"spec/notExisting/path",
				"spec/andAnother/one",
			},
		},
	}
	for _, testCase := range testCases {
		env := os.Getenv("IMAGEOVERRIDE_PATH")
		if len(testCase.env) != 0 {
			os.Setenv("IMAGEOVERRIDE_PATH", testCase.env)
		}
		if !reflect.DeepEqual(GetImageOverridePath(), testCase.expected) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, GetImageOverridePath())
		}
		if len(testCase.env) != 0 {
			os.Setenv("IMAGEOVERRIDE_PATH", env)
		}
	}
}
