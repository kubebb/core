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

package utils

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// ParseValues parses Values
func ParseValues(dir string, reference *apiextensionsv1.JSON) (fileName string, err error) {
	if dir == "" || reference == nil || reference.Raw == nil {
		return "", nil
	}
	data, err := yaml.JSONToYAML(reference.Raw)
	if err != nil {
		return "", err
	}
	fileName = fmt.Sprintf("%d.yaml", rand.Int31())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, fileName))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

func GetOCIEntryName(url string) string {
	nameSegments := strings.Split(url, "/")
	entryName := nameSegments[len(nameSegments)-1]
	return entryName
}
