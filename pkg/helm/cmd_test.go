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
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestInstall(t *testing.T) {
	type args struct {
		ctx       context.Context
		client    client.Client
		logger    logr.Logger
		planName  string
		manifests []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Install(tt.args.ctx, tt.args.client, tt.args.logger, tt.args.planName, tt.args.manifests); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnInstallByManifest(t *testing.T) {
	type args struct {
		ctx       context.Context
		client    client.Client
		manifests []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UnInstallByManifest(tt.args.ctx, tt.args.client, tt.args.manifests); (err != nil) != tt.wantErr {
				t.Errorf("UnInstallByManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnInstallByResources(t *testing.T) {
	type args struct {
		ctx       context.Context
		client    client.Client
		ns        string
		planName  string
		resources []corev1alpha1.Resource
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UnInstallByResources(tt.args.ctx, tt.args.client, tt.args.ns, tt.args.planName, tt.args.resources); (err != nil) != tt.wantErr {
				t.Errorf("UnInstallByResources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getHelmTemplate(t *testing.T) {
	type args struct {
		ctx       context.Context
		cli       client.Client
		logger    logr.Logger
		name      string
		namespace string
		chart     string
		version   string
		repoName  string
		repoUrl   string
		override  corev1alpha1.Override
		skipCrd   bool
		isOCI     bool
	}
	tests := []struct {
		name    string
		args    args
		want    []*unstructured.Unstructured
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getHelmTemplate(tt.args.ctx, tt.args.cli, tt.args.logger, tt.args.name, tt.args.namespace, tt.args.chart, tt.args.version, tt.args.repoName, tt.args.repoUrl, tt.args.override, tt.args.skipCrd, tt.args.isOCI)
			if (err != nil) != tt.wantErr {
				t.Errorf("getHelmTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHelmTemplate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetManifests(t *testing.T) {
	type args struct {
		ctx       context.Context
		cli       client.Client
		logger    logr.Logger
		name      string
		namespace string
		chart     string
		version   string
		repoName  string
		repoUrl   string
		override  corev1alpha1.Override
		skipCrd   bool
		isOCI     bool
	}
	tests := []struct {
		name     string
		args     args
		wantData []string
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotData, err := GetManifests(tt.args.ctx, tt.args.cli, tt.args.logger, tt.args.name, tt.args.namespace, tt.args.chart, tt.args.version, tt.args.repoName, tt.args.repoUrl, tt.args.override, tt.args.skipCrd, tt.args.isOCI)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetManifests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotData, tt.wantData) {
				t.Errorf("GetManifests() gotData = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}
