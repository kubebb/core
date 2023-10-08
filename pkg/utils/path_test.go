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

package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"syscall"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// TestParseValues for ParseValues
func TestParseValues(t *testing.T) {
	testCases := []struct {
		description string
		dir         string
		reference   apiextensionsv1.JSON

		expectedError  error
		expectedPrefix string
		expectedSuffix string
	}{
		{
			description: "write a normal file",
			dir:         "/tmp",
			reference: apiextensionsv1.JSON{
				Raw: []byte(`images:
      - name: docker.io/bitnami/nginx
        newTag: latest # default is docker.io/bitnami/nginx:1.25.1-debian-11-r0`),
			},

			expectedPrefix: "/tmp",
			expectedSuffix: ".yaml",
		},
		{
			description: "empty dir",
			dir:         "",
			reference: apiextensionsv1.JSON{
				Raw: []byte(`images:
      - name: docker.io/bitnami/nginx
        newTag: latest # default is docker.io/bitnami/nginx:1.25.1-debian-11-r0`),
			},

			expectedPrefix: "",
			expectedSuffix: "",
		},
		{
			description: "empty json",
			dir:         "/tmp",
			reference:   apiextensionsv1.JSON{},

			expectedPrefix: "",
			expectedSuffix: "",
		},
		{
			description: "incorrect json value",
			dir:         "/tmp",
			reference: apiextensionsv1.JSON{
				Raw: []byte(`images:123321
      - name: docker.io/bitnami/nginx
        newTag: latest # default is docker.io/bitnami/nginx:1.25.1-debian-11-r0`),
			},

			expectedError: errors.New("yaml: line 2: mapping values are not allowed in this context"),
		},
		{
			description: "dir cannot be created",
			dir:         "/etc/profile",
			reference: apiextensionsv1.JSON{
				Raw: []byte(`images:
      - name: docker.io/bitnami/nginx
        newTag: latest # default is docker.io/bitnami/nginx:1.25.1-debian-11-r0`),
			},

			expectedError: &fs.PathError{Op: "mkdir", Path: "/etc/profile", Err: syscall.ENOTDIR},
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test: %s", testCase.description), func(t *testing.T) {
			fileName, err := ParseValues(testCase.dir, &testCase.reference)
			if !reflect.DeepEqual(err, testCase.expectedError) {
				t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expectedError, err)
			} else if !(strings.HasPrefix(fileName, testCase.expectedPrefix) && strings.HasSuffix(fileName, testCase.expectedSuffix)) {
				t.Fatalf("Test Failed: %s, actual: %v", testCase.description, fileName)
			}
		})
	}
}

func TestGetOCIEntryName(t *testing.T) {
	type input struct {
		url, expectName string
	}
	for _, tc := range []input{
		{
			url:        "oci://a.com/abc/def",
			expectName: "def",
		},
		{
			url:        "oci://a.com/aa",
			expectName: "aa",
		},
		{
			url:        "oci://a.com",
			expectName: "",
		},
	} {
		if r := GetOCIEntryName(tc.url); r != tc.expectName {
			t.Fatalf("Test Failed. expect %s, actual: %s", tc.expectName, r)
		}
	}
}

func TestGetHTTPEntryName(t *testing.T) {
	type input struct {
		url, expectName string
	}
	for _, tc := range []input{
		{
			url:        "https://github.com/kubebb/components/releases/download/bc-apis-0.0.3/bc-apis-0.0.3.tgz ",
			expectName: "bc-apis",
		},
		{
			url:        "https://github.com/kubebb/components/releases/download/bc-apis-0.0.3/bc-apis0.0.3.tgz",
			expectName: "bc",
		},
		{
			url:        "https://github.com/kubebb/components/releases/download/bc-apis-0.0.3/bcapis",
			expectName: "",
		},
	} {
		if r := GetHTTPEntryName(tc.url); r != tc.expectName {
			t.Fatalf("Test Failed. expect %s, actual: %s", tc.expectName, r)
		}
	}
}
