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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name         string
		args         args
		wantRegistry string
		wantPath     string
		wantRemains  string
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name: "no tag",
			args: args{
				s: "nginx",
			},
			wantRegistry: DefaultDomain,
			wantPath:     OfficialRepoName,
			wantErr:      assert.NoError,
			wantRemains:  "nginx:latest",
		},
		{
			name: "with tag",
			args: args{
				s: "nginx:1.2.3",
			},
			wantRegistry: DefaultDomain,
			wantPath:     OfficialRepoName,
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "with digest",
			args: args{
				s: "nginx@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: DefaultDomain,
			wantPath:     OfficialRepoName,
			wantErr:      assert.NoError,
			wantRemains:  "nginx@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "with tag and digest",
			args: args{
				s: "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: DefaultDomain,
			wantPath:     OfficialRepoName,
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "with domain",
			args: args{
				s: "cc.docker.io/nginx:1.2.3",
			},
			wantRegistry: "cc.docker.io",
			wantPath:     "",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "with domain and port",
			args: args{
				s: "foo.com:443/nginx:1.2.3",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "with domain, port, tag and digest",
			args: args{
				s: "foo.com:443/nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "one path, no tag",
			args: args{
				s: "test/nginx",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:latest",
		},
		{
			name: "one path, with tag",
			args: args{
				s: "test/nginx:1.2.3",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "one path, with digest",
			args: args{
				s: "test/nginx@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "one path, with tag and digest",
			args: args{
				s: "test/nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "one path, with domain",
			args: args{
				s: "docker.cc/test/nginx:1.2.3",
			},
			wantRegistry: "docker.cc",
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "one path, with domain and port",
			args: args{
				s: "foo.com:443/test/nginx:1.2.3",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "one path, with domain, port, tag and digest",
			args: args{
				s: "foo.com:443/test/nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "test",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "two paths, no tag",
			args: args{
				s: "testa/testb/nginx",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:latest",
		},
		{
			name: "two paths, with tag",
			args: args{
				s: "testa/testb/nginx:1.2.3",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "two paths, with digest",
			args: args{
				s: "testa/testb/nginx@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "two paths, with tag and digest",
			args: args{
				s: "testa/testb/nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: DefaultDomain,
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "two paths, with domain",
			args: args{
				s: "docker.cc/testa/testb/nginx:1.2.3",
			},
			wantRegistry: "docker.cc",
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "two paths, with domain and port",
			args: args{
				s: "foo.com:443/testa/testb/nginx:1.2.3",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3",
		},
		{
			name: "two paths, with domain, port, tag and digest",
			args: args{
				s: "foo.com:443/testa/testb/nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "testa/testb",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
		{
			name: "five paths, with domain, port, tag and digest",
			args: args{
				s: "foo.com:443/testa/testb/testc/testd/teste/nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
			},
			wantRegistry: "foo.com:443",
			wantPath:     "testa/testb/testc/testd/teste",
			wantErr:      assert.NoError,
			wantRemains:  "nginx:1.2.3@sha256:affa73a743c5d81bd90fae203ff0ce11a544efd89b63402f7ba19919fe11615d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRegistry, gotPath, gotRemains, err := Split(tt.args.s)
			if !tt.wantErr(t, err, fmt.Sprintf("Split(%v)", tt.args.s)) {
				return
			}
			assert.Equalf(t, tt.wantRegistry, gotRegistry, "Split(%v)", tt.args.s)
			assert.Equalf(t, tt.wantPath, gotPath, "Split(%v)", tt.args.s)
			assert.Equalf(t, tt.wantRemains, gotRemains, "Split(%v)", tt.args.s)
		})
	}
}
