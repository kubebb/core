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
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/repo/repotest"
	"k8s.io/utils/strings/slices"
)

const (
	username = "kubebb-username"
	password = "kubebb-password"
)

// Note: Tests should consider tests in https://github.com/helm/helm/blob/v3.9.4/cmd/helm/repo_add_test.go.
// v3.9.4 is the version in go.mod that should be rechecked for test completeness when go.mod is updated.
func TestRepoAdd(t *testing.T) {
	const repoName = "test_repo_add"
	srv := newTempServer(t, "testdata/testserver/*.*", "", "")
	defer srv.Stop()
	srv1 := newTempServer(t, "testdata/testserver/*.*", "", "")
	defer srv1.Stop()
	srvBasicAuth := newTempServer(t, "testdata/testserver/*.*", username, password)
	defer srvBasicAuth.Stop()

	type args struct {
		ctx                context.Context
		logger             logr.Logger
		name               string
		url                string
		username           string
		password           string
		httpRequestTimeout time.Duration
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		errMsg     string
		needRemove bool
	}{
		{
			name: "add a repository",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName,
				url:    srv.URL(),
			},
			wantErr:    false,
			needRemove: true,
		},
		{
			name: "add repository second time with the same config, will have no error, this is different with the raw `helm repo add` command",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName,
				url:    srv.URL(),
			},
			wantErr:    false,
			needRemove: true,
		},
		{
			name: "add repository second time with different url, will have no error, this is different with the raw `helm repo add` command",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName,
				url:    srv1.URL(),
			},
			wantErr:    false,
			needRemove: true,
		},
		{
			name: "incorrect url",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName,
				url:    "",
			},
			wantErr: true,
			errMsg:  "could not find protocol handler for: ",
		},
		{
			name: "incorrect name",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   "test/test",
				url:    srv.URL(),
			},
			wantErr: true,
			errMsg:  "repository name (test/test) contains '/', please specify a different name without '/'",
		},
		{
			name: "basic auth with right username and password",
			args: args{
				ctx:      context.Background(),
				logger:   logr.Discard(),
				name:     repoName,
				url:      srvBasicAuth.URL(),
				username: username,
				password: password,
			},
			wantErr:    false,
			needRemove: true,
		},
		{
			name: "basic auth with wrong password",
			args: args{
				ctx:      context.Background(),
				logger:   logr.Discard(),
				name:     repoName,
				url:      srvBasicAuth.URL(),
				username: username,
				password: password + "1",
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("looks like \"%s\" is not a valid chart repository or cannot be reached: failed to fetch %s/index.yaml : 401 Unauthorized", srvBasicAuth.URL(), srvBasicAuth.URL()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RepoAdd(tt.args.ctx, tt.args.logger, tt.args.name, tt.args.url, tt.args.username, tt.args.password, tt.args.httpRequestTimeout); (err != nil) != tt.wantErr || tt.errMsg != "" && (err == nil || err.Error() != tt.errMsg) {
				t.Errorf("RepoAdd() wantErr = %v error = %v, wantErrMsg = %s", tt.wantErr, err, tt.errMsg)
			}
			if tt.needRemove {
				_ = RepoRemove(tt.args.ctx, tt.args.logger, tt.args.name)
			}
		})
	}
}

func newTempServer(t *testing.T, glob, optionUsername, optionPassword string) *repotest.Server {
	srv, err := repotest.NewTempServerWithCleanup(t, glob)
	srv.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if optionUsername != "" || optionPassword != "" {
		srv.WithMiddleware(func(w http.ResponseWriter, r *http.Request) {
			getUsername, getPassword, ok := r.BasicAuth()
			if !ok || optionUsername != getUsername || optionPassword != getPassword {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		})
	}
	srv.Start()
	return srv
}

// Note: Tests should consider tests in https://github.com/helm/helm/blob/v3.9.4/cmd/helm/repo_update_test.go.
// v3.9.4 is the version in go.mod that should be rechecked for test completeness when go.mod is updated.
func TestRepoUpdate(t *testing.T) {
	const repoName = "test_repo_update"
	srv := newTempServer(t, "testdata/testserver/*.*", username, password)
	defer srv.Stop()
	type args struct {
		ctx                context.Context
		logger             logr.Logger
		name               string
		httpRequestTimeout time.Duration
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		errMsg     []string
		needAdd    bool
		needRemove bool
	}{
		{
			name: "update a repository",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName,
			},
			wantErr:    false,
			needAdd:    true,
			needRemove: true,
		},
		{
			name: "update a noexist repository",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName + "1",
			},
			wantErr: true,
			errMsg:  []string{"no repositories found matching 'test_repo_update1'.  Nothing will be updated", "no repositories found."}, // Whether there was a repo before determines the difference in the error.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needAdd {
				_ = RepoAdd(tt.args.ctx, tt.args.logger, tt.args.name, srv.URL(), username, password, tt.args.httpRequestTimeout)
			}
			if err := RepoUpdate(tt.args.ctx, tt.args.logger, tt.args.name, tt.args.httpRequestTimeout); (err != nil) != tt.wantErr || len(tt.errMsg) != 0 && (err == nil || !slices.Contains(tt.errMsg, err.Error())) {
				t.Errorf("RepoUpdate() wantErr = %v error = %v, wantErrMsg = %s", tt.wantErr, err, tt.errMsg)
			}
			if tt.needRemove {
				_ = RepoRemove(tt.args.ctx, tt.args.logger, tt.args.name)
			}
		})
	}
}

// Note: Tests should consider tests in https://github.com/helm/helm/blob/v3.9.4/cmd/helm/repo_remove_test.go.
// v3.9.4 is the version in go.mod that should be rechecked for test completeness when go.mod is updated.
func TestRepoRemove(t *testing.T) {
	const repoName = "test_repo_remove"
	srv := newTempServer(t, "testdata/testserver/*.*", username, password)
	defer srv.Stop()
	type args struct {
		ctx    context.Context
		logger logr.Logger
		name   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		errMsg  []string
		needAdd bool
	}{
		{
			name: "remove a repository",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName,
			},
			wantErr: false,
			needAdd: true,
		},
		{
			name: "remove a noexist repository",
			args: args{
				ctx:    context.Background(),
				logger: logr.Discard(),
				name:   repoName + "1",
			},
			wantErr: true,
			errMsg:  []string{"no repo named \"test_repo_remove1\" found", "no repositories configured"}, // Whether there was a repo before determines the difference in the error.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needAdd {
				_ = RepoAdd(tt.args.ctx, tt.args.logger, tt.args.name, srv.URL(), username, password, time.Second)
			}
			if err := RepoRemove(tt.args.ctx, tt.args.logger, tt.args.name); (err != nil) != tt.wantErr || len(tt.errMsg) != 0 && (err == nil || !slices.Contains(tt.errMsg, err.Error())) {
				t.Errorf("RepoRemove() wantErr = %v error = %v, wantErrMsg = %s", tt.wantErr, err, tt.errMsg)
			}
		})
	}
}
