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
	"bytes"
	"sync"

	"sigs.k8s.io/kustomize/api/krusty"
	kustomize "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/repoimage"
)

const inputFile = "input.yaml"

type postRenderer struct {
	// For https://github.com/kubernetes-sigs/kustomize/issues/3659
	kustomizeRenderMutex sync.Mutex
	repoOverride         []corev1alpha1.ImageOverride
	images               []kustomize.Image
}

func newPostRenderer(repoOverride []corev1alpha1.ImageOverride, images []kustomize.Image) *postRenderer {
	return &postRenderer{repoOverride: repoOverride, images: images}
}

func (c *postRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	fs := filesys.MakeFsInMemory()
	cfg := kustomize.Kustomization{}
	cfg.APIVersion = kustomize.KustomizationVersion
	cfg.Kind = kustomize.KustomizationKind
	cfg.Images = c.images

	cfg.Resources = append(cfg.Resources, inputFile)
	f, err := fs.Create(inputFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err = f.Write(renderedManifests.Bytes()); err != nil {
		return nil, err
	}

	kustomization, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	f, err = fs.Create("kustomization.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err = f.Write(kustomization); err != nil {
		return nil, err
	}

	c.kustomizeRenderMutex.Lock()
	defer c.kustomizeRenderMutex.Unlock()

	buildOptions := &krusty.Options{
		LoadRestrictions: kustomize.LoadRestrictionsNone,
		PluginConfig:     kustomize.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	resMap, err := k.Run(fs, ".")
	if err != nil {
		return nil, err
	}
	path := corev1alpha1.GetImageOverridePath()
	if len(path) != 0 && len(c.repoOverride) != 0 {
		fsslice := make([]kustomize.FieldSpec, len(path))
		for i, p := range path {
			fsslice[i] = kustomize.FieldSpec{Path: p}
		}
		if err = resMap.ApplyFilter(repoimage.Filter{ImageOverride: c.repoOverride, FsSlice: fsslice}); err != nil {
			return nil, err
		}
	}
	yaml, err := resMap.AsYaml()
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(yaml), nil
}
