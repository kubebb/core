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
	"bytes"
	"log"
	"os"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

func ExampleFilter() {
	err := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: bytes.NewBufferString(`
apiVersion: example.com/v1
kind: Foo
metadata:
  name: instance
spec:
  containers:
  - name: FooBar
    image: nginx:1.2.1
---
apiVersion: example.com/v1
kind: Bar
metadata:
  name: instance
spec:
  containers:
  - name: BarFoo
    image: nginx:2.1.2 
`)}},
		Filters: []kio.Filter{Filter{
			ImageOverride: []corev1alpha1.ImageOverride{
				{
					Registry:    "docker.io",
					NewRegistry: "docker.cc",
				},
			},
			FsSlice: []types.FieldSpec{
				{
					Path: "spec/containers[]/image",
				},
			},
		}},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}.Execute()
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// apiVersion: example.com/v1
	// kind: Foo
	// metadata:
	//   name: instance
	// spec:
	//   containers:
	//   - name: FooBar
	//     image: docker.cc/library/nginx:1.2.1
	// ---
	// apiVersion: example.com/v1
	// kind: Bar
	// metadata:
	//   name: instance
	// spec:
	//   containers:
	//   - name: BarFoo
	//     image: docker.cc/library/nginx:2.1.2
}
