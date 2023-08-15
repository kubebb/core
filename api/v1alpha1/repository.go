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
	"strings"

	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	Username = "username"
	Password = "password"
	CAData   = "cadata"
	CertData = "certdata"
	KeyData  = "keydata"

	ComponentRepositoryLabel = "kubebb.component.repository"
	RepositoryTypeLabel      = "kubebb.repository.type"

	RatingServiceAccountEnv     = "RATING_SERVICEACCOUNT"
	RatingClusterRoleEnv        = "RATING_CLUSTERROLE"
	RatingClusterRoleBindingEnv = "RATING_CLUSTERROLEBINDING"
	RatingEnableEnv             = "RATING_ENABLE"

	DefaultRatingServiaceAccount    = "rating-serviceaccount"
	DefaultRatingClusterRole        = "rating-clusterrole"
	DefaultRatingClusterRoleBinding = "rating-clusterrolebinding"
)

// NamespacedName return the namespaced name of the repository in string format
func (repo *Repository) NamespacedName() string {
	return fmt.Sprintf("%s.%s", repo.GetNamespace(), repo.GetName())
}

// IsPullStrategySame Determine whether the contents of two structures are the same
func IsPullStrategySame(a, b *PullStategy) bool {
	if a != nil && b != nil {
		return *a == *b
	}

	return a == nil && b == nil
}

// ImageOverridePath is the manifest path to detect kustomize image overrides
// can be replaced by environment variables IMAGEOVERRIDE_PATH, for example IMAGEOVERRIDE_PATH=spec/template/spec/initContainers/image:spec/initContainers/image
var ImageOverridePath = []string{"spec/containers/image", "spec/initContainers/image", "spec/template/spec/containers/image", "spec/template/spec/initContainers/image"}

func GetImageOverridePath() []string {
	v := os.Getenv("IMAGEOVERRIDE_PATH")
	if len(v) == 0 {
		return ImageOverridePath
	}
	return strings.Split(v, ":")
}

func EnsureRatingResources() {
	if !RatingEnabled() {
		return
	}

	cfg := config.GetConfigOrDie()
	c := kubernetes.NewForConfigOrDie(cfg)
	clusterRoleName := GetRatingClusterRole()
	if _, err := c.RbacV1().ClusterRoles().Get(context.Background(), clusterRoleName, metav1.GetOptions{}); err != nil {
		panic(err)
	}

	clusterRolebingName := GetRatingClusterRoleBinding()
	if _, err := c.RbacV1().ClusterRoleBindings().Get(context.Background(), clusterRolebingName, metav1.GetOptions{}); err != nil {
		panic(err)
	}
}

func RatingEnabled() bool {
	r, _ := env.GetBool(RatingEnableEnv, false)
	return r
}

func GetRatingServiceAccount() string {
	return env.GetString(RatingServiceAccountEnv, DefaultRatingServiaceAccount)
}

func GetRatingClusterRole() string {
	return env.GetString(RatingClusterRoleEnv, DefaultRatingClusterRole)
}

func GetRatingClusterRoleBinding() string {
	return env.GetString(RatingClusterRoleBindingEnv, DefaultRatingClusterRoleBinding)
}

func AddSubjectToClusterRoleBinding(ctx context.Context, c client.Client, namespace string) error {
	if !RatingEnabled() {
		return nil
	}

	clusterRoleBinding := GetRatingClusterRoleBinding()
	serviceAccount := GetRatingServiceAccount()
	crb := v1.ClusterRoleBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: clusterRoleBinding}, &crb); err != nil {
		return err
	}

	add := true
	for _, sub := range crb.Subjects {
		if sub.Kind == "ServiceAccount" && sub.Name == serviceAccount && sub.Namespace == namespace {
			add = false
			break
		}
	}
	if add {
		crb.Subjects = append(crb.Subjects, v1.Subject{Kind: "ServiceAccount", Name: serviceAccount, Namespace: namespace})
		return c.Update(ctx, &crb)
	}
	return nil
}

func RemoveSubjectFromClusterRoleBinding(ctx context.Context, c client.Client, namespace string) error {
	if !RatingEnabled() {
		return nil
	}

	clusterRoleBinding := GetRatingClusterRoleBinding()
	serviceAccount := GetRatingServiceAccount()
	crb := v1.ClusterRoleBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: clusterRoleBinding}, &crb); err != nil {
		return err
	}

	index, length := 0, len(crb.Subjects)
	for idx := 0; idx < length; idx++ {
		if crb.Subjects[idx].Kind == "ServiceAccount" && crb.Subjects[idx].Name == serviceAccount && crb.Subjects[idx].Namespace == namespace {
			continue
		}
		crb.Subjects[index] = crb.Subjects[idx]
		index++
	}
	if index != length {
		crb.Subjects = crb.Subjects[:index]
		return c.Update(ctx, &crb)
	}

	return nil
}
