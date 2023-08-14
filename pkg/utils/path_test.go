package utils

import (
	"errors"
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
		fileName, err := ParseValues(testCase.dir, &testCase.reference)
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Fatalf("Test Failed: %s, expected: %v, actual: %v", testCase.description, testCase.expectedError, err)
		} else if !(strings.HasPrefix(fileName, testCase.expectedPrefix) && strings.HasSuffix(fileName, testCase.expectedSuffix)) {
			t.Fatalf("Test Failed: %s, actual: %v", testCase.description, fileName)
		}
	}
}
