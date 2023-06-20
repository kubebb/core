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
	"os"
	"strings"
)

const (
	Username = "username"
	Password = "password"
	CAData   = "cadata"
	CertData = "certdata"
	KeyData  = "keydata"

	ComponentRepositoryLabel = "kubebb.component.repository"
)

// IsPullStrategySame Determine whether the contents of two structures are the same
func IsPullStrategySame(a, b *PullStategy) bool {
	if a != nil && b != nil {
		return *a == *b
	}

	return a == nil && b == nil
}

// ImageOverridePath is the manifest path to detect kustomize image overrides
// can be replaced by environment variables IMAGEOVERRIDE_PATH, for example IMAGEOVERRIDE_PATH=spec/template/spec/initContainers/image:spec/initContainers/image
var ImageOverridePath = []string{"spec/containers/image", "spec/initContainers/image"}

func GetImageOverridePath() []string {
	v := os.Getenv("IMAGEOVERRIDE_PATH")
	if len(v) == 0 {
		return ImageOverridePath
	}
	return strings.Split(v, ":")
}
