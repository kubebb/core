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

package helm

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Install installs a helm chart to the cluster
func Install(ctx context.Context, client client.Client, planName string, manifests []string) (err error) {
	for _, manifest := range manifests {
		obj := &unstructured.Unstructured{}
		if err = json.Unmarshal([]byte(manifest), obj); err != nil {
			return err
		}
		if !utils.IsCRD(obj) {
			if err = corev1alpha1.AddComponentPlanLabel(obj, planName); err != nil {
				return err
			}
		}
		_, err = controllerutil.CreateOrUpdate(ctx, client, obj, func() error {
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// UnInstallByManifest remove a helm chart from the cluster
func UnInstallByManifest(ctx context.Context, client client.Client, manifests []string) (err error) {
	for _, manifest := range manifests {
		obj := &unstructured.Unstructured{}
		if err = json.Unmarshal([]byte(manifest), obj); err != nil {
			return err
		}
		err = client.Delete(ctx, obj)
		if utils.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

// UnInstallByResources remove a helm chart from the cluster
func UnInstallByResources(ctx context.Context, c client.Client, ns, planName string, resources []corev1alpha1.Resource) (err error) {
	for _, resource := range resources {
		obj := &unstructured.Unstructured{}
		obj.SetKind(resource.Kind)
		obj.SetName(resource.Name)
		obj.SetAPIVersion(resource.APIVersion)
		obj.SetNamespace(ns)
		err = c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if utils.IgnoreNotFound(err) != nil {
			return err
		}
		labels := obj.GetLabels()
		if labels == nil {
			continue
		}
		if labels[corev1alpha1.ComponentPlanKey] != planName {
			continue
		}
		err = c.Delete(ctx, obj)
		if utils.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

// GetManifests get helm templates
func GetManifests(ctx context.Context, logger logr.Logger, name, namespace, chart, version, repoName, repoUrl string, set, setString, setFile, SetJSON, SetLiteral []string, skipCrd, isOCI bool) (data []string, err error) {
	Objs, err := getHelmTemplate(ctx, logger, name, namespace, chart, version, repoName, repoUrl, set, setString, setFile, SetJSON, SetLiteral, skipCrd, isOCI)
	if err != nil {
		return nil, err
	}
	manifests := make([]*unstructured.Unstructured, 0)
	for _, obj := range Objs {
		if obj == nil {
			continue
		}

		var targets []*unstructured.Unstructured
		if obj.IsList() {
			err = obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					targets = append(targets, unstructuredObj)
					return nil
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else if utils.IsNullList(obj) {
			// noop
		} else {
			targets = []*unstructured.Unstructured{obj}
		}

		manifests = append(manifests, targets...)
	}
	data = make([]string, len(manifests))
	for i, m := range manifests {
		manifestStr, err := json.Marshal(m.Object)
		if err != nil {
			return nil, err
		}
		data[i] = string(manifestStr)
	}
	return data, nil
}

func getHelmTemplate(ctx context.Context, logger logr.Logger, name, namespace, chart, version, repoName, repoUrl string, set, setString, setFile, SetJSON, SetLiteral []string, skipCrd, isOCI bool) ([]*unstructured.Unstructured, error) {
	h := NewHelm("", isOCI)
	out, err := h.repoAdd(ctx, repoName, repoUrl)
	if err != nil {
		return nil, err
	}
	logger.V(5).Info("helm repo add", "output", out)
	out, err = h.repoUpdate(ctx, repoName)
	if err != nil {
		return nil, err
	}
	logger.V(5).Info("helm repo update", "output", out)
	out, err = h.template(ctx, name, namespace, chart, version, set, setString, setFile, SetJSON, SetLiteral, skipCrd)
	if err != nil {
		if !isMissingDependencyErr(err) {
			return nil, err
		}
		// FIXME check this
		if err = cleanupChartLockFile("TODO"); err != nil {
			return nil, err
		}

		_, err := h.dependencyBuild(ctx)
		if err != nil {
			return nil, err
		}

		out, err = h.template(ctx, name, namespace, chart, version, set, setString, setFile, SetJSON, SetLiteral, skipCrd)
		if err != nil {
			return nil, err
		}
	}
	var _out string
	if len(out) > 20 {
		_out = out[:20]
	} else {
		_out = out
	}
	logger.V(5).Info("helm template", "output first 20", _out)
	return utils.SplitYAML([]byte(out))
}
