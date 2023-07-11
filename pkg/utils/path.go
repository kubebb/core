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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/kubebb/core/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ParseValuesReference parses ValuesReference
func ParseValuesReference(ctx context.Context, cli client.Client, ns, dir string, reference *v1alpha1.ValuesReference) (fileName string, err error) {
	if dir == "" || reference == nil {
		return "", nil
	}
	n := types.NamespacedName{Namespace: ns, Name: reference.Name}
	var data string
	ok := false
	switch reference.Kind {
	case "ConfigMap":
		cm := corev1.ConfigMap{}
		if err := cli.Get(ctx, n, &cm); err != nil {
			return "", err
		}
		data, ok = cm.Data[reference.GetValuesKey()]
		if !ok || len(data) == 0 {
			binaryData, ok := cm.BinaryData[reference.GetValuesKey()]
			if !ok || len(binaryData) == 0 {
				return "", errors.New("no data found in this configmap")
			}
			data = string(binaryData)
		}
	case "Secret":
		secret := corev1.Secret{}
		if err := cli.Get(ctx, n, &secret); err != nil {
			return "", err
		}
		data, ok = secret.StringData[reference.GetValuesKey()]
		if !ok || len(data) == 0 {
			binaryData, ok := secret.Data[reference.GetValuesKey()]
			if !ok || len(binaryData) == 0 {
				return "", errors.New("no data found in this secret")
			}
			data = string(binaryData)
		}
	default:
		return "", errors.New("no Kind setting found")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, reference.GetValuesKey()))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(data); err != nil {
		return "", err
	}
	return filepath.Join(dir, reference.GetValuesKey()), nil
}

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
