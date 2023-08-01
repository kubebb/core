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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"istio.io/istio/operator/pkg/compare"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/utils/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// IgnoreNotFound returns the supplied error, or nil if the error indicates a
// Kubernetes resource was not found.
func IgnoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
// inspire by github.com/argoproj/gitops-engine/pkg/utils/kube
func SplitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	ymls, err := SplitYAMLToString(yamlData)
	if err != nil {
		return nil, err
	}
	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// SplitYAMLToString splits a YAML file into strings. Returns list of yamls
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
// inspire by github.com/argoproj/gitops-engine/pkg/utils/kube
func SplitYAMLToString(yamlData []byte) ([]string, error) {
	// Similar way to what kubectl does
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/resource/visitor.go#L573-L600
	// Ideally k8s.io/cli-runtime/pkg/resource.Builder should be used instead of this method.
	// E.g. Builder does list unpacking and flattening and this code does not.
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []string
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		objs = append(objs, string(ext.Raw))
	}
	return objs, nil
}

func ResourceDiffStr(ctx context.Context, source, exist *unstructured.Unstructured, ignorePaths []string, c client.Client) (string, error) {
	newOne := source.DeepCopy()
	newOne.SetResourceVersion(exist.GetResourceVersion())
	if err := c.Patch(ctx, newOne, client.MergeFrom(exist), client.DryRunAll); err != nil {
		return "", err
	}
	existYaml, err := yaml.Marshal(OmitManagedFields(exist))
	if err != nil {
		return "", err
	}
	newYaml, err := yaml.Marshal(OmitManagedFields(newOne))
	if err != nil {
		return "", err
	}
	return compare.YAMLCmpWithIgnore(string(existYaml), string(newYaml), ignorePaths, ""), nil
}

func OmitManagedFields(o runtime.Object) runtime.Object {
	a, err := meta.Accessor(o)
	if err != nil {
		// The object is not a `metav1.Object`, ignore it.
		return o
	}
	a.SetManagedFields(nil)
	return o
}

func GetCronJobImage(obj *unstructured.Unstructured) (image []string) {
	cj := batchv1.CronJob{}
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &cj)
	image = ParseContainerImage(cj.Spec.JobTemplate.Spec.Template.Spec.Containers)
	image = append(image, ParseContainerImage(cj.Spec.JobTemplate.Spec.Template.Spec.InitContainers)...)
	return image
}

func GetJobImage(obj *unstructured.Unstructured) (image []string) {
	job := batchv1.Job{}
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &job)
	image = ParseContainerImage(job.Spec.Template.Spec.Containers)
	image = append(image, ParseContainerImage(job.Spec.Template.Spec.InitContainers)...)
	return image
}

func GetStatefulSetImage(obj *unstructured.Unstructured) (image []string) {
	sts := appsv1.StatefulSet{}
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &sts)
	image = ParseContainerImage(sts.Spec.Template.Spec.Containers)
	image = append(image, ParseContainerImage(sts.Spec.Template.Spec.InitContainers)...)
	return image
}

func GetDeploymentImage(obj *unstructured.Unstructured) (image []string) {
	deploy := appsv1.Deployment{}
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
	image = ParseContainerImage(deploy.Spec.Template.Spec.Containers)
	image = append(image, ParseContainerImage(deploy.Spec.Template.Spec.InitContainers)...)
	return image
}

func GetPodImage(obj *unstructured.Unstructured) (image []string) {
	pod := corev1.Pod{}
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod)
	image = ParseContainerImage(pod.Spec.Containers)
	image = append(image, ParseContainerImage(pod.Spec.InitContainers)...)
	return image
}

func ParseContainerImage(containers []corev1.Container) (image []string) {
	for _, container := range containers {
		image = append(image, container.Image)
	}
	return image
}

const InClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func GetNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(InClusterNamespacePath); os.IsNotExist(err) {
		operatorNamespace := os.Getenv("POD_NAMESPACE")
		if operatorNamespace == "" {
			return "", fmt.Errorf("not in cluster and env POD_NAMESPACE not found")
		}
		return operatorNamespace, nil
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(InClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

var (
	operatorUser string
)

func GetOperatorUser() string {
	return operatorUser
}

func SetOperatorUser(ctx context.Context, c client.Client) (string, error) {
	name := env.GetString("POD_NAME", "")
	if name == "" {
		return "", fmt.Errorf("env POD_NAME not set")
	}
	ns, err := GetNamespace()
	if err != nil {
		return "", err
	}
	pod := &corev1.Pod{}
	pod.Name = name
	pod.Namespace = ns
	if err := c.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		return "", err
	}
	sa := pod.Spec.ServiceAccountName
	if sa == "" {
		sa = pod.Spec.DeprecatedServiceAccount
	}
	operatorUser = serviceaccount.MakeUsername(ns, sa)
	return operatorUser, nil
}
