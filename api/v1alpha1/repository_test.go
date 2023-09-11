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
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		t.Run(fmt.Sprintf("test: %s", testCase.description), func(t *testing.T) {
			if !reflect.DeepEqual(testCase.repo.NamespacedName(), testCase.expected) {
				t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, testCase.repo.NamespacedName())
			}
		})
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
		t.Run(fmt.Sprintf("test: %s", testCase.description), func(t *testing.T) {
			result := IsPullStrategySame(testCase.pullA, testCase.pullB)
			if !reflect.DeepEqual(result, testCase.expected) {
				t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expected, result)
			}
		})
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
		t.Run(fmt.Sprintf("test: %s", testCase.description), func(t *testing.T) {
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
		})
	}
}

func TestRepository_IsOCI(t *testing.T) {
	type fields struct {
		Spec RepositorySpec
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "is oci",
			fields: fields{
				Spec: RepositorySpec{
					URL: "oci://registry-1.docker.io/bitnamicharts",
				},
			},
			want: true,
		},
		{
			name: "not oci",
			fields: fields{
				Spec: RepositorySpec{
					URL: "http://registry-1.docker.io/bitnamicharts",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{
				Spec: tt.fields.Spec,
			}
			if got := repo.IsOCI(); got != tt.want {
				t.Errorf("IsOCI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRatingEnabled(t *testing.T) {
	os.Setenv(RatingEnableEnv, "true")
	if !RatingEnabled() {
		t.Fatalf("Test Failed. expect true get false")
	}
	os.Unsetenv(RatingEnableEnv)
	if RatingEnabled() {
		t.Fatalf("Test Failed. expect false get true")
	}
}

func TestGetRatingServiceAccount(t *testing.T) {
	setSA := "serviceaccount"
	os.Setenv(RatingServiceAccountEnv, setSA)
	if r := GetRatingServiceAccount(); r != setSA {
		t.Fatalf("Test Failed. expect '%s' get '%s'", setSA, r)
	}
	os.Unsetenv(RatingServiceAccountEnv)
	if r := GetRatingServiceAccount(); r != DefaultRatingServiaceAccount {
		t.Fatalf("Test Failed. expect '%s' get '%s'", DefaultRatingServiaceAccount, r)
	}
}

func TestGetRatingClusterRole(t *testing.T) {
	setRole := "clustrrole"
	os.Setenv(RatingClusterRoleEnv, setRole)
	if r := GetRatingClusterRole(); r != setRole {
		t.Fatalf("Test Failed. expec '%s' get '%s'", setRole, r)
	}
	os.Unsetenv(RatingClusterRoleEnv)
	if r := GetRatingClusterRole(); r != DefaultRatingClusterRole {
		t.Fatalf("Test Failed. expec '%s' get '%s'", DefaultRatingClusterRole, r)
	}
}

func TestGetRatingClusterRoleBinding(t *testing.T) {
	setRolebinding := "clusterrolebinding"
	os.Setenv(RatingClusterRoleBindingEnv, setRolebinding)
	if r := GetRatingClusterRoleBinding(); r != setRolebinding {
		t.Fatalf("Test Failed. expect '%s' get '%s'", setRolebinding, r)
	}
	os.Unsetenv(RatingClusterRoleBindingEnv)
	if r := GetRatingClusterRoleBinding(); r != DefaultRatingClusterRoleBinding {
		t.Fatalf("Test Failed. expect '%s' get '%s'", DefaultRatingClusterRoleBinding, r)
	}
}

func TestAddSubjectToClusterRoleBinding(t *testing.T) {
	c := fake.NewClientBuilder()
	namespace := "default"
	expectSubject := []v1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      GetRatingServiceAccount(),
			Namespace: namespace,
		},
	}

	addSubjectCLB := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultRatingClusterRoleBinding,
		},
	}

	subjectCLB := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Subjects: expectSubject,
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	c.WithObjects(addSubjectCLB, subjectCLB)
	c.WithScheme(scheme)
	// first test not enable
	client := c.Build()
	if err := AddSubjectToClusterRoleBinding(context.TODO(), client, namespace); err != nil {
		t.Fatalf("Test Failed. rating is not enabled and no error should be returned.")
	}
	// set rating
	os.Setenv(RatingEnableEnv, "true")
	if err := AddSubjectToClusterRoleBinding(context.TODO(), client, namespace); err != nil {
		t.Fatalf("Test Failed. rating is enabled, but serviceaccount is not set properly. with error: %v", err)
	}
	// checkt clusterrolebinding
	clb := &v1.ClusterRoleBinding{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: GetRatingClusterRoleBinding()}, clb); err != nil {
		t.Fatalf("Test Failed. after setting clusterrolebinding %s's subject, getobject erorr: %v", GetRatingClusterRoleBinding(), err)
	}
	if !reflect.DeepEqual(clb.Subjects, expectSubject) {
		t.Fatalf("Test Failed. for clusterrolebinding %s, the expected subject is %v but got %v", GetRatingClusterRoleBinding(), expectSubject, clb.Subjects)
	}

	// set clusterrolebinding to default
	os.Setenv(RatingClusterRoleBindingEnv, "default")
	if err := AddSubjectToClusterRoleBinding(context.TODO(), client, namespace); err != nil {
		t.Fatalf("Test Failed. rating is enabled, but serviceaccount is not set properly. with error: %v", err)
	}
	clb = &v1.ClusterRoleBinding{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: GetRatingClusterRoleBinding()}, clb); err != nil {
		t.Fatalf("Test Failed. after setting clusterrolebinding %s's subject, getobject erorr: %v", GetRatingClusterRoleBinding(), err)
	}
	if !reflect.DeepEqual(clb.Subjects, expectSubject) {
		t.Fatalf("Test Failed. for clusterrolebinding %s, the expected subject is %v but got %v", GetRatingClusterRoleBinding(), expectSubject, clb.Subjects)
	}

	os.Unsetenv(RatingClusterRoleBindingEnv)
	os.Unsetenv(RatingEnableEnv)
}

func TestRemoveSubjectFromClusterRoleBinding(t *testing.T) {
	c := fake.NewClientBuilder()
	namespace := "default"

	clb1 := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultRatingClusterRoleBinding,
		},
	}

	clb2 := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Subjects: []v1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      GetRatingServiceAccount(),
				Namespace: namespace,
			},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	c.WithObjects(clb1, clb2)
	c.WithScheme(scheme)
	// first test not enable
	client := c.Build()
	if err := RemoveSubjectFromClusterRoleBinding(context.TODO(), client, namespace); err != nil {
		t.Fatalf("Test Failed. rating is not enabled and no error should be returned.")
	}

	os.Setenv(RatingEnableEnv, "true")
	// remove serviceaccount from clusterrolebinding rating-clusterrolebinding
	if err := RemoveSubjectFromClusterRoleBinding(context.TODO(), client, namespace); err != nil {
		t.Fatalf("Test Failed. rating is enabled, but the serviceaccount is not removed normally. with error: %v", err)
	}
	// checkt clusterrolebinding
	clb := &v1.ClusterRoleBinding{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: GetRatingClusterRoleBinding()}, clb); err != nil {
		t.Fatalf("Test Failed. after setting clusterrolebinding %s's subject, getobject erorr: %v", GetRatingClusterRoleBinding(), err)
	}
	if len(clb.Subjects) != 0 {
		t.Fatalf("Test Failed. for clusterrolebinding %s, the expected subject is nil but got %v", GetRatingClusterRoleBinding(), clb.Subjects)
	}

	os.Setenv(RatingClusterRoleBindingEnv, "default")
	// remove serviceaccount from clustrrolebinding default
	if err := RemoveSubjectFromClusterRoleBinding(context.TODO(), client, namespace); err != nil {
		t.Fatalf("Test Failed. rating is enabled, but the serviceaccount is not removed normally. with error: %v", err)
	}
	clb = &v1.ClusterRoleBinding{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: GetRatingClusterRoleBinding()}, clb); err != nil {
		t.Fatalf("Test Failed. after setting clusterrolebinding %s's subject, getobject erorr: %v", GetRatingClusterRoleBinding(), err)
	}
	if len(clb.Subjects) != 0 {
		t.Fatalf("Test Failed. for clusterrolebinding %s, the expected subject is nil but got %v", GetRatingClusterRoleBinding(), clb.Subjects)
	}
	os.Unsetenv(RatingClusterRoleBindingEnv)
	os.Unsetenv(RatingEnableEnv)
}
