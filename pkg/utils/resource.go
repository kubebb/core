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
	"sort"

	"github.com/go-logr/logr"
	"github.com/kubebb/core/api/v1alpha1"
	"istio.io/istio/operator/pkg/compare"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
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

// IsCRD returns true if the supplied object is a CRD
// inspire by github.com/argoproj/gitops-engine/pkg/utils/kube
func IsCRD(u *unstructured.Unstructured) bool {
	gvk := u.GroupVersionKind()
	return gvk.Kind == "CustomResourceDefinition" && gvk.Group == "apiextensions.k8s.io"
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

// IsNullList checks if the object is a "List" type where items is null instead of an empty list.
// Handles a corner case where obj.IsList() returns false when a manifest is like:
// ---
// apiVersion: v1
// items: null
// kind: ConfigMapList
// copy from https://github.com/argoproj/argo-cd/blob/50b2f03657026a0987e4910eca4778e8950e6d87/reposerver/repository/repository.go#L1525
func IsNullList(obj *unstructured.Unstructured) bool {
	if _, ok := obj.Object["spec"]; ok {
		return false
	}
	if _, ok := obj.Object["status"]; ok {
		return false
	}
	field, ok := obj.Object["items"]
	if !ok {
		return false
	}
	return field == nil
}

func ResourceDiffStr(ctx context.Context, source, exist *unstructured.Unstructured, ignorePaths []string, c client.Client) (string, error) {
	newOne := source.DeepCopy()
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

// ComponentPlanDiffIgnorePaths is the list of paths to ignore when comparing
// These fields will almost certainly change when componentplan is updated, and displaying these
// changes will only result in more invalid information, so they need to be ignored
var ComponentPlanDiffIgnorePaths = []string{
	"metadata.generation",
	"metadata.resourceVersion",
	"metadata.labels." + v1alpha1.ComponentPlanKey,
	"spec.template.metadata.labels." + v1alpha1.ComponentPlanKey,
	"metadata.labels.helm.sh/chart",
	"spec.template.metadata.labels.helm.sh/chart",
}

// GetResourcesAndImages get resource slices, image lists from manifests
func GetResourcesAndImages(ctx context.Context, logger logr.Logger, c client.Client, manifests []string) (resources []v1alpha1.Resource, images []string, err error) {
	resources = make([]v1alpha1.Resource, len(manifests))
	for i, manifest := range manifests {
		obj := &unstructured.Unstructured{}
		if err = json.Unmarshal([]byte(manifest), obj); err != nil {
			return nil, nil, err
		}
		has := &unstructured.Unstructured{}
		has.SetKind(obj.GetKind())
		has.SetAPIVersion(obj.GetAPIVersion())
		err = c.Get(ctx, client.ObjectKeyFromObject(obj), has)
		var isNew bool
		if err != nil && apierrors.IsNotFound(err) {
			isNew = true
		} else if err != nil {
			return nil, nil, err
		}
		r := v1alpha1.Resource{
			Kind:       obj.GetKind(),
			Name:       obj.GetName(),
			APIVersion: obj.GetAPIVersion(),
		}
		if isNew {
			r.NewCreated = &isNew
		} else {
			diff, err := ResourceDiffStr(ctx, obj, has, ComponentPlanDiffIgnorePaths, c)
			if err != nil {
				logger.Error(err, "failed to get diff", "obj", klog.KObj(obj))
				diffMsg := "diff with exist"
				r.SpecDiffwithExist = &diffMsg
			} else if diff == "" {
				ignore := "no spec diff, but some field like resourceVersion will update"
				r.SpecDiffwithExist = &ignore
			} else {
				r.SpecDiffwithExist = &diff
			}
		}
		resources[i] = r
		gvk := obj.GroupVersionKind()
		switch gvk.Group {
		case "":
			switch gvk.Kind {
			case "Pod":
				images = append(images, GetPodImage(obj)...)
			}
		case "apps":
			switch gvk.Kind {
			case "Deployment":
				images = append(images, GetDeploymentImage(obj)...)
			case "StatefulSet":
				images = append(images, GetStatefulSetImage(obj)...)
			}
		case "batch":
			switch gvk.Kind {
			case "Job":
				images = append(images, GetJobImage(obj)...)
			case "CronJob":
				images = append(images, GetCronJobImage(obj)...)
			}
		}
	}
	imageMap := make(map[string]bool)
	for _, i := range images {
		imageMap[i] = true
	}
	images = make([]string, 0, len(imageMap))
	for k := range imageMap {
		images = append(images, k)
	}
	sort.Strings(images)
	return resources, images, nil
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
