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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// IgnoreNotFound returns the supplied error, or nil if the error indicates a
// Kubernetes resource was not found.
func IgnoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// IsCRD returns true if the supplied object is a CRD
// inspire by github.com/argoproj/gitops-engine/pkg/utils/kube
func IsCRD(u *unstructured.Unstructured) bool {
	gvk := u.GroupVersionKind()
	return gvk.Kind == "CustomResourceDefinition" && gvk.Group == "apiextensions.k8s.io"
}

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
// inspire by github.com/argoproj/gitops-engine/pkg/utils/kube
func SplitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	ymls, err := SplitYAMLToString(yamlData)
	if err != nil {
		return nil, err
	}
	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// SplitYAMLToString splits a YAML file into strings. Returns list of yamls
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
// inspire by github.com/argoproj/gitops-engine/pkg/utils/kube
func SplitYAMLToString(yamlData []byte) ([]string, error) {
	// Similar way to what kubectl does
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/resource/visitor.go#L573-L600
	// Ideally k8s.io/cli-runtime/pkg/resource.Builder should be used instead of this method.
	// E.g. Builder does list unpacking and flattening and this code does not.
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []string
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		objs = append(objs, string(ext.Raw))
	}
	return objs, nil
}

// IsNullList checks if the object is a "List" type where items is null instead of an empty list.
// Handles a corner case where obj.IsList() returns false when a manifest is like:
// ---
// apiVersion: v1
// items: null
// kind: ConfigMapList
// copy from https://github.com/argoproj/argo-cd/blob/50b2f03657026a0987e4910eca4778e8950e6d87/reposerver/repository/repository.go#L1525
func IsNullList(obj *unstructured.Unstructured) bool {
	if _, ok := obj.Object["spec"]; ok {
		return false
	}
	if _, ok := obj.Object["status"]; ok {
		return false
	}
	field, ok := obj.Object["items"]
	if !ok {
		return false
	}
	return field == nil
}

func ResourceDiffStr(ctx context.Context, source, exist *unstructured.Unstructured, c client.Client) (string, error) {
	newOne := source.DeepCopy()
	if err := c.Patch(ctx, newOne, client.MergeFrom(exist), client.DryRunAll); err != nil {
		return "", err
	}
	existYaml, err := yaml.Marshal(OmitManagedFields(exist))
	if err != nil {
		return "", err
	}
	newYaml, err := yaml.Marshal(OmitManagedFields(newOne))
	if err != nil {
		return "", err
	}
	return Diff(string(existYaml), string(newYaml), 1), nil
	//oldSpec, oldExist, oldErr := unstructured.NestedMap(obj.Object, "spec")
	//spec, exist, err := unstructured.NestedMap(has.Object, "spec")
	//logger.Info("!!!", "objSpec", oldSpec, "objExist", oldExist, "objErr", oldErr)
	//logger.Info("!!!", "hasSpec", spec, "hasExist", exist, "hasErr", err)
	//diffStr := diff.ObjectDiff(oldSpec, spec)
	//r.SpecDiffwithExist = &diffStr
}

func OmitManagedFields(o runtime.Object) runtime.Object {
	a, err := meta.Accessor(o)
	if err != nil {
		// The object is not a `metav1.Object`, ignore it.
		return o
	}
	a.SetManagedFields(nil)
	return o
}

func Diff(old, new string, unified int) string {
	singleLine := func(s string) (r []string) {
		for _, l := range strings.Split(s, "\n") {
			r = append(r, l+"\n")
		}
		if len(r) > 1 && r[len(r)-1] == "\n" {
			r = r[:len(r)-1]
		}
		return r
	}

	prefixLines := func(s, prefix string) string {
		var buf bytes.Buffer
		lines := strings.Split(s, "\n")
		ls := regexp.MustCompile("^")
		for _, line := range lines[:len(lines)-1] {
			buf.WriteString(ls.ReplaceAllString(line, prefix))
			buf.WriteString("\n")
		}
		return buf.String()
	}

	dmp := diffmatchpatch.New()
	wSrc, wDst, warray := dmp.DiffLinesToRunes(old, new)
	diffs := dmp.DiffMainRunes(wSrc, wDst, true)
	diffs = dmp.DiffCharsToLines(diffs, warray)
	res := make([]string, 0)
	currentLine := 0
	diffLine := make(map[int]bool, 0)
	var out bytes.Buffer
	for _, diff := range diffs {
		text := diff.Text
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			currentLine++
			res = append(res, prefixLines(text, "+"))
			diffLine[currentLine] = true
		case diffmatchpatch.DiffDelete:
			currentLine++
			res = append(res, prefixLines(text, "-"))
			diffLine[currentLine] = true
		case diffmatchpatch.DiffEqual:
			r := singleLine(prefixLines(text, " "))
			res = append(res, r...)
			currentLine += len(r)
		}
	}
	for k := range diffLine {
		for i := k - unified; i <= k+unified; i++ {
			if i >= 0 && i < len(res) {
				diffLine[i] = true
			}
		}
	}
	diffLineKey := make([]int, 0, len(diffLine))
	for k := range diffLine {
		diffLineKey = append(diffLineKey, k)
	}
	sort.Ints(diffLineKey)
	lastLineNum := 0
	for _, l := range diffLineKey {
		if l-lastLineNum != 1 {
			out.WriteString("......\n")
		}
		out.WriteString(res[l])
		lastLineNum = l
	}
	return out.String()
}
