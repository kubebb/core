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
	"testing"
)

func TestHelm_RegistryLogin(t *testing.T) {
	type fields struct {
		binaryName string
		WorkDir    string
		IsHelmOCI  bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Helm{
				binaryName: tt.fields.binaryName,
				WorkDir:    tt.fields.WorkDir,
				IsHelmOCI:  tt.fields.IsHelmOCI,
			}
			if err := h.RegistryLogin(); (err != nil) != tt.wantErr {
				t.Errorf("RegistryLogin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHelm_dependencyBuild(t *testing.T) {
	type fields struct {
		binaryName string
		WorkDir    string
		IsHelmOCI  bool
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Helm{
				binaryName: tt.fields.binaryName,
				WorkDir:    tt.fields.WorkDir,
				IsHelmOCI:  tt.fields.IsHelmOCI,
			}
			got, err := h.dependencyBuild(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("dependencyBuild() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("dependencyBuild() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelm_repoAdd(t *testing.T) {
	type fields struct {
		binaryName string
		WorkDir    string
		IsHelmOCI  bool
	}
	type args struct {
		ctx  context.Context
		name string
		url  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Helm{
				binaryName: tt.fields.binaryName,
				WorkDir:    tt.fields.WorkDir,
				IsHelmOCI:  tt.fields.IsHelmOCI,
			}
			got, err := h.repoAdd(tt.args.ctx, tt.args.name, tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("repoAdd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("repoAdd() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelm_run(t *testing.T) {
	type fields struct {
		binaryName string
		WorkDir    string
		IsHelmOCI  bool
	}
	type args struct {
		ctx  context.Context
		args []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Helm{
				binaryName: tt.fields.binaryName,
				WorkDir:    tt.fields.WorkDir,
				IsHelmOCI:  tt.fields.IsHelmOCI,
			}
			got, err := h.run(tt.args.ctx, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("run() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cleanupChartLockFile(t *testing.T) {
	type args struct {
		chartPath string
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
			if err := cleanupChartLockFile(tt.args.chartPath); (err != nil) != tt.wantErr {
				t.Errorf("cleanupChartLockFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_isMissingDependencyErr(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMissingDependencyErr(tt.args.err); got != tt.want {
				t.Errorf("isMissingDependencyErr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelm_template(t *testing.T) {
	type fields struct {
		binaryName string
		WorkDir    string
		IsHelmOCI  bool
	}
	type args struct {
		ctx        context.Context
		name       string
		namespace  string
		chart      string
		version    string
		set        []string
		setString  []string
		setFile    []string
		SetJSON    []string
		SetLiteral []string
		skipCrd    bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Helm{
				binaryName: tt.fields.binaryName,
				WorkDir:    tt.fields.WorkDir,
				IsHelmOCI:  tt.fields.IsHelmOCI,
			}
			got, err := h.template(tt.args.ctx, tt.args.name, tt.args.namespace, tt.args.chart, tt.args.version, tt.args.set, tt.args.setString, tt.args.setFile, tt.args.SetJSON, tt.args.SetLiteral, tt.args.skipCrd)
			if (err != nil) != tt.wantErr {
				t.Errorf("template() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("template() got = %v, want %v", got, tt.want)
			}
		})
	}
}
