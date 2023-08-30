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
	"context"
	"reflect"
	"testing"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
)

func TestGetHarborRepository(t *testing.T) {
	type args struct {
		ctx  context.Context
		repo *corev1alpha1.Repository
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantHas map[string]bool
		wantErr bool
	}{
		// {
		// Note: demo.goharbor.io is cleaned and reset every two days.
		// So if this test goes wrong, we need to first look to see if the image has been deleted,
		// and then troubleshoot if it's a problem with the code
		//	name: "normal test 1",
		//	args: args{
		//		ctx: context.TODO(),
		//		repo: &corev1alpha1.Repository{
		//			Spec: corev1alpha1.RepositorySpec{
		//				URL: "oci://demo.goharbor.io/helm-test",
		//			},
		//		},
		//	},
		//	wantHas: map[string]bool{"oci://demo.goharbor.io/helm-test/nginx": true, "oci://demo.goharbor.io/helm-test/wordpress": true},
		//	wantErr: false,
		// },
		{
			name: "normal test 2",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://demo.goharbor.io/helm-test/nginx",
					},
				},
			},
			want:    []string{"oci://demo.goharbor.io/helm-test/nginx"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHarborRepository(tt.args.ctx, tt.args.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHarborRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetHarborRepository() got = %v, want %v", got, tt.want)
			}
			if len(tt.wantHas) != 0 {
				for _, g := range got {
					ok, exist := tt.wantHas[g]
					if exist {
						if !ok {
							t.Fatalf("GetHarborRepository() got = %v, wantHas %v", got, tt.wantHas)
						}
						delete(tt.wantHas, g)
					}
				}
				for _, v := range tt.wantHas {
					if v {
						t.Fatalf("GetHarborRepository() got = %v, wantHas %v", got, tt.wantHas)
					}
				}
			}
		})
	}
}

func TestGetDockerhubHelmRepository(t *testing.T) {
	type args struct {
		ctx  context.Context
		repo *corev1alpha1.Repository
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantHas map[string]bool
		wantErr bool
	}{
		{
			name: "normal test 1",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://registry-1.docker.io/bitnamicharts",
					},
				},
			},
			wantHas: map[string]bool{"oci://registry-1.docker.io/bitnamicharts/wordpress": true, "oci://registry-1.docker.io/bitnamicharts/nginx": true},
			wantErr: false,
		},
		{
			name: "normal test 2",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://registry-1.docker.io/bitnamicharts/wordpress",
					},
				},
			},
			want:    []string{"oci://registry-1.docker.io/bitnamicharts/wordpress"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDockerhubHelmRepository(tt.args.ctx, tt.args.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDockerhubHelmRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDockerhubHelmRepository() got = %v, want %v", got, tt.want)
			}
			if len(tt.wantHas) != 0 {
				for _, g := range got {
					ok, exist := tt.wantHas[g]
					if exist {
						if !ok {
							t.Fatalf("GetDockerhubHelmRepository() got = %v, wantHas %v", got, tt.wantHas)
						}
						delete(tt.wantHas, g)
					}
				}
				for _, v := range tt.wantHas {
					if v {
						t.Fatalf("GetDockerhubHelmRepository() got = %v, wantHas %v", got, tt.wantHas)
					}
				}
			}
		})
	}
}

func TestGetGithubHelmRepository(t *testing.T) {
	type args struct {
		ctx  context.Context
		repo *corev1alpha1.Repository
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantHas map[string]bool
		wantErr bool
	}{
		{
			name: "GitHub Organization only has name",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/oci-helm-example",
					},
				},
			},
			wantHas: map[string]bool{"oci://ghcr.io/oci-helm-example/helm-oci-example/nginx": true, "oci://ghcr.io/oci-helm-example/helm-oci-example/wordpress": true, "oci://ghcr.io/oci-helm-example/redis": true},
			wantErr: false,
		},
		{
			name: "GitHub Organization separate image",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/oci-helm-example/redis",
					},
				},
			},
			wantHas: map[string]bool{"oci://ghcr.io/oci-helm-example/helm-oci-example/nginx": false, "oci://ghcr.io/oci-helm-example/helm-oci-example/wordpress": false, "oci://ghcr.io/oci-helm-example/redis": true},
			wantErr: false,
		},
		{
			name: "GitHub Organization with repository name",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/oci-helm-example/helm-oci-example",
					},
				},
			},
			wantHas: map[string]bool{"oci://ghcr.io/oci-helm-example/helm-oci-example/nginx": true, "oci://ghcr.io/oci-helm-example/helm-oci-example/wordpress": true, "oci://ghcr.io/oci-helm-example/redis": false},
			wantErr: false,
		},
		{
			name: "GitHub Organization with repository name and image name",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/oci-helm-example/helm-oci-example/nginx",
					},
				},
			},
			want:    []string{"oci://ghcr.io/oci-helm-example/helm-oci-example/nginx"},
			wantErr: false,
		},
		{
			name: "GitHub User only has username",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/abirdcfly",
					},
				},
			},
			wantHas: map[string]bool{"oci://ghcr.io/abirdcfly/redis": true, "oci://ghcr.io/abirdcfly/helm-oci-example/wordpress": true, "oci://ghcr.io/abirdcfly/helm-oci-example/nginx": true},
			wantErr: false,
		},
		{
			name: "GitHub User separate image",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/abirdcfly/redis",
					},
				},
			},
			wantHas: map[string]bool{"oci://ghcr.io/abirdcfly/helm-oci-example/nginx": false, "oci://ghcr.io/abirdcfly/helm-oci-example/wordpress": false, "oci://ghcr.io/abirdcfly/redis": true},
			wantErr: false,
		},
		{
			name: "GitHub User with repository name",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/abirdcfly/helm-oci-example",
					},
				},
			},
			wantHas: map[string]bool{"oci://ghcr.io/abirdcfly/helm-oci-example/nginx": true, "oci://ghcr.io/abirdcfly/helm-oci-example/wordpress": true, "oci://ghcr.io/abirdcfly/helm-oci-example/redis": false},
			wantErr: false,
		},
		{
			name: "GitHub User with repository name and image name",
			args: args{
				ctx: context.TODO(),
				repo: &corev1alpha1.Repository{
					Spec: corev1alpha1.RepositorySpec{
						URL: "oci://ghcr.io/abirdcfly/helm-oci-example/nginx",
					},
				},
			},
			want:    []string{"oci://ghcr.io/abirdcfly/helm-oci-example/nginx"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetGithubHelmRepository(tt.args.ctx, tt.args.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGithubHelmRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetGithubHelmRepository() got = %v, want %v", got, tt.want)
			}
			if len(tt.wantHas) != 0 {
				for _, g := range got {
					ok, exist := tt.wantHas[g]
					if exist {
						if !ok {
							t.Fatalf("GetGithubHelmRepository() got = %v, wantHas %v", got, tt.wantHas)
						}
						delete(tt.wantHas, g)
					}
				}
				for _, v := range tt.wantHas {
					if v {
						t.Fatalf("GetGithubHelmRepository() got = %v, wantHas %v", got, tt.wantHas)
					}
				}
			}
		})
	}
}
