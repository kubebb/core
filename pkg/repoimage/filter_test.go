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
	"testing"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	filtertest "sigs.k8s.io/kustomize/api/testutils/filtertest"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestFilter(t *testing.T) {
	mutationTrackerStub := filtertest.MutationTrackerStub{}
	testCases := map[string]struct {
		input                string
		expectedOutput       string
		filter               Filter
		fsSlice              types.FsSlice
		setValueCallback     func(key, value, tag string, node *yaml.RNode)
		expectedSetValueArgs []filtertest.SetValueArg
	}{
		"ignore CustomResourceDefinition": {
			input: `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: whatever
spec:
  containers:
  - image: whatever
`,
			expectedOutput: `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: whatever
spec:
  containers:
  - image: whatever
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:     "docker.io",
						NewRegistry:  "docker.cc",
						PathOverride: nil,
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers/image",
				},
			},
		},

		"legacy multiple images in containers": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: nginx:1.2.1
  - image: nginx:2.1.2
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: docker.cc/library/nginx:1.2.1
  - image: docker.cc/library/nginx:2.1.2
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:     "docker.io",
						NewRegistry:  "docker.cc",
						PathOverride: nil,
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers/image",
				},
			},
		},
		"legacy both containers and initContainers": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: nginx:1.2.1
  - image: tomcat:1.2.3
  initContainers:
  - image: nginx:1.2.1
  - image: apache:1.2.3
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: docker.cc/library/nginx:1.2.1
  - image: docker.cc/library/tomcat:1.2.3
  initContainers:
  - image: docker.cc/library/nginx:1.2.1
  - image: docker.cc/library/apache:1.2.3
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{

					{
						Registry:     "docker.io",
						NewRegistry:  "docker.cc",
						PathOverride: nil,
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers/image",
				},
				{
					Path: "spec/initContainers/image",
				},
			},
		},
		"legacy updates at multiple depths": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: nginx:1.2.1
  - image: tomcat:1.2.3
  template:
    spec:
      initContainers:
      - image: nginx:1.2.1
      - image: apache:1.2.3
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: docker.cc/library/nginx:1.2.1
  - image: docker.cc/library/tomcat:1.2.3
  template:
    spec:
      initContainers:
      - image: docker.cc/library/nginx:1.2.1
      - image: docker.cc/library/apache:1.2.3
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{

					{
						Registry:     "docker.io",
						NewRegistry:  "docker.cc",
						PathOverride: nil,
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers/image",
				},
				{
					Path: "spec/template/spec/initContainers/image",
				},
			},
		},
		"update with path": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  image: docker.io/kubebb/nginx:1.2.1
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  image: docker.cc/kubebb-replace/nginx:1.2.1
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:    "docker.io",
						NewRegistry: "docker.cc",
						PathOverride: &corev1alpha1.PathOverride{
							Path:    "kubebb",
							NewPath: "kubebb-replace",
						},
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/image",
				},
			},
		},

		"update multiple paths and registry": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: nginx:1.2.1
  - image: quay.io/kubevirt/cdi-apiserver:v1.46.0
  - image: localhost:5000/kubevirt/cdi-apiserver:v1.46.0
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: quay.io/kubevirt/nginx:1.2.1
  - image: docker.io/library/cdi-apiserver:v1.46.0
  - image: localhost:5000/kubevirt/cdi-apiserver:v1.46.0
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:    "docker.io",
						NewRegistry: "quay.io",
						PathOverride: &corev1alpha1.PathOverride{
							Path:    "library",
							NewPath: "kubevirt",
						},
					},
					{
						Registry:    "quay.io",
						NewRegistry: "docker.io",
						PathOverride: &corev1alpha1.PathOverride{
							Path:    "kubevirt",
							NewPath: "library",
						},
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers/image",
				},
			},
		},
		"multiple matches in sequence": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: nginx:1.2.1
  - image: a.docker.io/library/nginx:1.2.1
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - image: docker.cc/library/nginx:1.2.1
  - image: a.docker.io/library/nginx:1.2.1
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:    "docker.io",
						NewRegistry: "docker.cc",
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers/image",
				},
			},
		},

		"emptyContainers": {
			input: `
group: apps
apiVersion: v1
kind: Deployment
metadata:
  name: deploy1
spec:
  containers:
`,
			expectedOutput: `
group: apps
apiVersion: v1
kind: Deployment
metadata:
  name: deploy1
spec:
  containers: []
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:    "docker.io",
						NewRegistry: "docker.cc",
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/containers[]/image",
					//					CreateIfNotPresent: true,
				},
			},
		},
		"mutation tracker": {
			input: `
group: apps
apiVersion: v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
      - image: nginx:latest
        name: nginx-latest
      - image: docker.cc/abc/foobar:1
        name: replaced-with-digest
      - image: docker.cc/abc/postgres:1.8.0
        name: postgresdb
      initContainers:
      - image: nginx
        name: nginx-notag
      - image: nginx@sha256:03c1151dfb9695f66e31c93008bc74d7d9870ef29739f6f36a261652b5d266a6
        name: nginx-sha256
      - image: docker.cc/abc/alpine:1.8.0
        name: init-alpine
`,
			expectedOutput: `
group: apps
apiVersion: v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: docker.cc/library/nginx:1.7.9
        name: nginx-tagged
      - image: docker.cc/library/nginx:latest
        name: nginx-latest
      - image: docker.cc/abc/foobar:1
        name: replaced-with-digest
      - image: docker.cc/abc/postgres:1.8.0
        name: postgresdb
      initContainers:
      - image: docker.cc/library/nginx:latest
        name: nginx-notag
      - image: docker.cc/library/nginx@sha256:03c1151dfb9695f66e31c93008bc74d7d9870ef29739f6f36a261652b5d266a6
        name: nginx-sha256
      - image: docker.cc/abc/alpine:1.8.0
        name: init-alpine
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:    "docker.io",
						NewRegistry: "docker.cc",
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/template/spec/containers[]/image",
				},
				{
					Path: "spec/template/spec/initContainers[]/image",
				},
			},
			setValueCallback: mutationTrackerStub.MutationTracker,
			expectedSetValueArgs: []filtertest.SetValueArg{
				{
					Value:    "docker.cc/library/nginx:1.7.9",
					NodePath: []string{"spec", "template", "spec", "containers", "image"},
				},
				{
					Value:    "docker.cc/library/nginx:latest",
					NodePath: []string{"spec", "template", "spec", "containers", "image"},
				},
				{
					Value:    "docker.cc/library/nginx:latest",
					NodePath: []string{"spec", "template", "spec", "initContainers", "image"},
				},
				{
					Value:    "docker.cc/library/nginx@sha256:03c1151dfb9695f66e31c93008bc74d7d9870ef29739f6f36a261652b5d266a6",
					NodePath: []string{"spec", "template", "spec", "initContainers", "image"},
				},
			},
		},
		"image with tag and digest new name": {
			input: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  image: nginx:1.2.1@sha256:46d5b90a7f4e9996351ad893a26bcbd27216676ad4d5316088ce351fb2c2c3dd
`,
			expectedOutput: `
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  image: docker.cc/library/nginx:1.2.1@sha256:46d5b90a7f4e9996351ad893a26bcbd27216676ad4d5316088ce351fb2c2c3dd
`,
			filter: Filter{
				ImageOverride: []corev1alpha1.ImageOverride{
					{
						Registry:    "docker.io",
						NewRegistry: "docker.cc",
					},
				},
			},
			fsSlice: []types.FieldSpec{
				{
					Path: "spec/image",
				},
			},
		},
	}

	for tn, tc := range testCases {
		mutationTrackerStub.Reset()
		t.Run(tn, func(t *testing.T) {
			filter := tc.filter
			filter.WithMutationTracker(tc.setValueCallback)
			filter.FsSlice = tc.fsSlice
			if !assert.Equal(t,
				strings.TrimSpace(tc.expectedOutput),
				strings.TrimSpace(filtertest.RunFilter(t, tc.input, filter))) {
				t.FailNow()
			}
			assert.Equal(t, tc.expectedSetValueArgs, mutationTrackerStub.SetValueArgs())
		})
	}
}
