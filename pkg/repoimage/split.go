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

	"github.com/distribution/distribution/v3/reference"
)

func Split(s string) (registry string, path string, remains string, err error) {
	ref, err := reference.ParseNormalizedNamed(s)
	if err != nil {
		return "", "", "", err
	}
	registry = reference.Domain(ref)
	paths := reference.Path(ref)
	data := strings.Split(paths, "/")
	var p []string
	for index, i := range data {
		if i == "" || index == len(data)-1 {
			continue
		}
		p = append(p, i)
	}
	path = strings.Join(p, "/")
	if reference.IsNameOnly(ref) {
		ref = reference.TagNameOnly(ref)
	}
	prefix := registry + "/"
	if len(path) != 0 {
		prefix += path + "/"
	}
	remains = strings.TrimPrefix(ref.String(), prefix)
	return
}

const (
	//	https://github.com/distribution/distribution/blob/6a57630cf40122000083e60bcb7e97c50a904c5e/reference/normalize.go#L31
	DefaultDomain    = "docker.io"
	OfficialRepoName = "library"
)
