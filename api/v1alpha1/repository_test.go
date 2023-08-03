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
	"reflect"
	"testing"
)

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
