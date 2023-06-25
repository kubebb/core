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

package repoimage

import (
	"strings"

	"github.com/kubebb/core/api/v1alpha1"
	"sigs.k8s.io/kustomize/api/filters/filtersutil"

	"sigs.k8s.io/kustomize/api/image"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// imageTagUpdater is an implementation of the kio.Filter interface
// that will update the value of the yaml node based on the provided
// ImageTag if the current value matches the format of an image reference.
type updater struct {
	Kind            string                   `yaml:"kind,omitempty"`
	ImageOverride   []v1alpha1.ImageOverride `yaml:"imageOverride,omitempty"`
	trackableSetter filtersutil.TrackableSetter
}

func (u updater) SetImageValue(rn *yaml.RNode) error {
	if err := yaml.ErrorIfInvalid(rn, yaml.ScalarNode); err != nil {
		return err
	}

	value := rn.YNode().Value
	if !image.IsImageMatched(value, ".*?") {
		return nil
	}
	registry, path, remainers, err := Split(value)
	if err != nil {
		// ignore err
		return nil
	}
	var newRegistry, newPath string
	for _, o := range u.ImageOverride {
		if registry != o.Registry {
			continue
		}
		if o.NewRegistry != "" {
			newRegistry = o.NewRegistry
		}
		if o.PathOverride == nil {
			continue
		}
		if o.PathOverride.Path == path {
			newPath = o.PathOverride.NewPath
		}
	}
	if newRegistry != "" || newPath != "" {
		v := []string{registry}
		if newRegistry != "" {
			v[0] = newRegistry
		}
		if newPath != "" {
			v = append(v, newPath)
		} else if path != "" {
			v = append(v, path)
		}
		v = append(v, remainers)
		return u.trackableSetter.SetScalar(strings.Join(v, "/"))(rn)
	}
	return nil
}

func (u updater) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	if err := u.SetImageValue(rn); err != nil {
		return nil, err
	}
	return rn, nil
}
