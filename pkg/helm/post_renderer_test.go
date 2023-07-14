/*
 * Copyright 2023 The Kubebb Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package helm

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"istio.io/istio/operator/pkg/compare"
	kustomize "sigs.k8s.io/kustomize/api/types"
)

const (
	nginxPod = `apiVersion: v1
kind: Pod
metadata:
  labels:
    app: nginx
  name: nginx
  namespace: default
spec:
  containers:
  - image: nginx
    imagePullPolicy: IfNotPresent
    name: nginx
`

	controllerDeploy = `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: controller-manager
  namespace: baas-system
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
      name: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
        name: controller-manager
    spec:
      containers:
      - image: 172.22.50.223/bestchains-dev/fabric-operator:7776e71
        name: operator
`
	finalGotImage = `apiVersion: v1
kind: Pod
metadata:
  labels:
    app: nginx
  name: nginx
  namespace: default
spec:
  containers:
  - image: docker.cc/library/nginx:v2
    imagePullPolicy: IfNotPresent
    name: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: controller-manager
  namespace: baas-system
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
      name: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
        name: controller-manager
    spec:
      containers:
      - image: 172.22.50.223/bestchains-dev/fabric-operator:7776e71
        name: operator
`
)

func Test_postRenderer_Run(t *testing.T) {
	testBuf := bytes.NewBuffer([]byte(nginxPod + "\n---\n" + controllerDeploy))
	//controllerDeployBuf := bytes.NewBuffer([]byte(controllerDeploy))
	type fields struct {
		repoOverride []corev1alpha1.ImageOverride
		images       []kustomize.Image
	}
	type args struct {
		renderedManifests *bytes.Buffer
	}
	tests := []struct {
		name                  string
		fields                fields
		args                  args
		wantModifiedManifests *bytes.Buffer
		wantErr               bool
	}{
		{
			name: "empty config, empty input",
			fields: fields{
				repoOverride: nil,
				images:       nil,
			},
			args: args{
				renderedManifests: bytes.NewBuffer(nil),
			},
			wantModifiedManifests: bytes.NewBuffer(nil),
			wantErr:               false,
		},
		{
			name: "empty override will not change input",
			fields: fields{
				repoOverride: nil,
				images:       nil,
			},
			args: args{
				renderedManifests: testBuf,
			},
			wantModifiedManifests: bytes.NewBuffer([]byte(nginxPod + "---\n" + controllerDeploy)),
			wantErr:               false,
		},
		{
			name: "kustomize image tags only",
			fields: fields{
				repoOverride: nil,
				images:       []kustomize.Image{{Name: "nginx", NewTag: "v2"}},
			},
			args: args{
				renderedManifests: testBuf,
			},
			wantModifiedManifests: bytes.NewBuffer([]byte(strings.ReplaceAll(nginxPod, `image: nginx`, `image: nginx:v2`) + "---\n" + controllerDeploy)),
			wantErr:               false,
		},
		{
			name: "override registry only",
			fields: fields{
				repoOverride: []corev1alpha1.ImageOverride{{Registry: "docker.io", NewRegistry: "docker.cc"}},
				images:       nil,
			},
			args: args{
				renderedManifests: testBuf,
			},
			wantModifiedManifests: bytes.NewBuffer([]byte(strings.ReplaceAll(nginxPod, `image: nginx`, `image: docker.cc/library/nginx:latest`) + "---\n" + controllerDeploy)),
			wantErr:               false,
		},
		{
			name: "override registry and kustomize image",
			fields: fields{
				repoOverride: []corev1alpha1.ImageOverride{{Registry: "docker.io", NewRegistry: "docker.cc"}},
				images:       []kustomize.Image{{Name: "nginx", NewTag: "v2"}},
			},
			args: args{
				renderedManifests: testBuf,
			},
			wantModifiedManifests: bytes.NewBuffer([]byte(strings.ReplaceAll(nginxPod, `image: nginx`, `image: docker.cc/library/nginx:v2`) + "---\n" + controllerDeploy)),
			wantErr:               false,
		},
		{
			name: "override registry, kustomize image and define componentplan name",
			fields: fields{
				repoOverride: []corev1alpha1.ImageOverride{{Registry: "docker.io", NewRegistry: "docker.cc"}},
				images:       []kustomize.Image{{Name: "nginx", NewTag: "v2"}},
			},
			args: args{
				renderedManifests: testBuf,
			},
			wantModifiedManifests: bytes.NewBuffer([]byte(finalGotImage)),
			wantErr:               false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &postRenderer{
				kustomizeRenderMutex: sync.Mutex{},
				repoOverride:         tt.fields.repoOverride,
				images:               tt.fields.images,
			}
			gotModifiedManifests, err := c.Run(tt.args.renderedManifests)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotModifiedManifests == nil || strings.TrimSpace(gotModifiedManifests.String()) != strings.TrimSpace(tt.wantModifiedManifests.String()) {
				t.Errorf("Run() gotModifiedManifests = %v, want %v", gotModifiedManifests, tt.wantModifiedManifests)
				if gotModifiedManifests != nil {
					t.Errorf("diff: %s", compare.YAMLCmp(tt.wantModifiedManifests.String(), gotModifiedManifests.String()))
				}
			}
		})
	}
}
