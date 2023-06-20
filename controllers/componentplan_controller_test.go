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

package controllers

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	kustomize "sigs.k8s.io/kustomize/api/types"
)

const (
	nginxPod         = `{"apiVersion":"v1","kind":"Pod","metadata":{"labels":{"app":"nginx"},"name":"nginx","namespace":"default"},"spec":{"containers":[{"image":"nginx","imagePullPolicy":"IfNotPresent","name":"nginx"}]}}`
	controllerDeploy = `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"labels":{"control-plane":"controller-manager"},"name":"controller-manager","namespace":"baas-system"},"spec":{"replicas":1,"selector":{"matchLabels":{"control-plane":"controller-manager","name":"controller-manager"}},"template":{"metadata":{"labels":{"control-plane":"controller-manager","name":"controller-manager"}},"spec":{"containers":[{"image":"172.22.50.223/bestchains-dev/fabric-operator:7776e71","name":"operator"}]}}}}`
)

func TestComponentPlanReconciler_UpdateImages(t *testing.T) {
	type args struct {
		jsonManifests []string
		repoOverride  []corev1alpha1.ImageOverride
		images        []kustomize.Image
	}
	tests := []struct {
		name         string
		args         args
		wantJsonData []string
		wantErr      bool
	}{
		{
			name: "empty input",
			args: args{
				jsonManifests: nil,
				repoOverride:  nil,
				images:        nil,
			},
			wantJsonData: nil,
			wantErr:      false,
		},
		{
			name: "empty override will not change input",
			args: args{
				jsonManifests: []string{nginxPod, controllerDeploy},
				repoOverride:  nil,
				images:        nil,
			},
			wantJsonData: []string{nginxPod, controllerDeploy},
			wantErr:      false,
		},
		{
			name: "kustomize image tags only",
			args: args{
				jsonManifests: []string{nginxPod, controllerDeploy},
				repoOverride:  nil,
				images:        []kustomize.Image{{Name: "nginx", NewTag: "v2"}},
			},
			wantJsonData: []string{strings.ReplaceAll(nginxPod, `"image":"nginx"`, `"image":"nginx:v2"`), controllerDeploy},
			wantErr:      false,
		},
		{
			name: "override registry only",
			args: args{
				jsonManifests: []string{nginxPod, controllerDeploy},
				repoOverride:  []corev1alpha1.ImageOverride{{Registry: "docker.io", NewRegistry: "docker.cc"}},
				images:        nil,
			},
			wantJsonData: []string{strings.ReplaceAll(nginxPod, `"image":"nginx"`, `"image":"docker.cc/library/nginx:latest"`), controllerDeploy},
			wantErr:      false,
		},
		{
			name: "override registry and kustomize image",
			args: args{
				jsonManifests: []string{nginxPod, controllerDeploy},
				repoOverride:  []corev1alpha1.ImageOverride{{Registry: "docker.io", NewRegistry: "docker.cc"}},
				images:        []kustomize.Image{{Name: "nginx", NewTag: "v2"}},
			},
			wantJsonData: []string{strings.ReplaceAll(nginxPod, `"image":"nginx"`, `"image":"docker.cc/library/nginx:v2"`), controllerDeploy},
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ComponentPlanReconciler{
				Client:               fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build(),
				kustomizeRenderMutex: sync.Mutex{},
			}
			gotJsonData, err := r.UpdateImages(context.TODO(), tt.args.jsonManifests, tt.args.repoOverride, tt.args.images)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotJsonData, tt.wantJsonData) {
				t.Errorf("UpdateImages() gotJsonData = %v, want %v", gotJsonData, tt.wantJsonData)
			}
		})
	}
}
