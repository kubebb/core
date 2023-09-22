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
	"os"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kubebb/core/api/v1alpha1"
)

const (
	defaultChanSize = 5
	defaultCustomer = 5
)

type ChartWorker interface {
	Push(ChartDef)
}

type ChartDef struct {
	URL           string
	Version       string
	ConfigMapName string
	Component     *v1alpha1.Component
	H             *CoreHelmWrapper
	Scheme        *runtime.Scheme
}

type option struct {
	customers int
	client    client.Client
	chCount   int
	ctx       context.Context
}

type chartWorker struct {
	options *option
	ch      chan ChartDef
	logger  logr.Logger
}

type Option func(*option)

func WithChCount(c int) Option {
	// don't allow unbuffered chan
	if c == 0 {
		c = defaultChanSize
	}
	return func(o *option) {
		o.chCount = c
	}
}

func WithK8sClient(c client.Client) Option {
	return func(o *option) {
		o.client = c
	}
}

func WithCustomers(customers int) Option {
	if customers <= 0 {
		customers = defaultCustomer
	}
	return func(o *option) {
		o.customers = customers
	}
}

func WithContext(ctx context.Context) Option {
	return func(o *option) {
		o.ctx = ctx
	}
}

func (c ChartDef) String() string {
	return fmt.Sprintf("url: %s, version: %s, target configmap: %s", c.URL, c.Version, c.ConfigMapName)
}

func NewChartWorker(options ...Option) ChartWorker {
	chartWorkerOptions := &option{}
	for _, op := range options {
		op(chartWorkerOptions)
	}

	if chartWorkerOptions.chCount == 0 {
		chartWorkerOptions.chCount = defaultChanSize
	}
	if chartWorkerOptions.customers == 0 {
		chartWorkerOptions.customers = defaultCustomer
	}
	if chartWorkerOptions.ctx == nil {
		chartWorkerOptions.ctx = context.Background()
	}

	ch := make(chan ChartDef, chartWorkerOptions.chCount)
	logger, _ := logr.FromContext(chartWorkerOptions.ctx)
	logger = logger.WithName("ChartWorker")
	cw := &chartWorker{
		options: chartWorkerOptions,
		ch:      ch,
		logger:  logger,
	}
	cw.start(chartWorkerOptions.ctx)
	return cw
}

func (c *chartWorker) Push(cd ChartDef) {
	c.ch <- cd
}

func (c *chartWorker) start(ctx context.Context) {
	c.logger.V(0).Info(fmt.Sprintf("chartworker start to work, with %d instances.", c.options.customers))
	for i := 0; i < c.options.customers; i++ {
		go func(i int) {
			for {
				select {
				case def := <-c.ch:
					c.logger.Info(fmt.Sprintf("[chartWorker:%d] start finishing the task: %s", i, def))
					_ = c.do(def)
				case <-ctx.Done():
					c.logger.Info("[chartWorker:%d] done!", i)
					return
				}
			}
		}(i)
	}
}

func (c *chartWorker) do(cd ChartDef) error {
	needCreate := false
	cm := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: cd.Component.Namespace,
			Name:      cd.ConfigMapName,
		},
	}
	if err := c.options.client.Get(c.options.ctx, types.NamespacedName{Namespace: cd.Component.Namespace, Name: cd.ConfigMapName}, &cm); err != nil {
		if !errors.IsNotFound(err) {
			c.logger.Error(err, "")
			return err
		}
		needCreate = true
	}

	_, ok1 := cm.Data[v1alpha1.ValuesConfigMapKey]
	_, ok2 := cm.Data[v1alpha1.ImagesConfigMapKey]
	_, ok3 := cm.Data[v1alpha1.READMEConfigMapKey]
	if ok1 && ok2 && ok3 {
		c.logger.Info("all required fields are present and are no longer processed.")
		return nil
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	_, dir, entryName, err := cd.H.Pull(c.options.ctx, cd.URL, cd.Version)
	if err != nil {
		c.logger.Error(err, "")
		return err
	}
	defer os.Remove(dir)

	if b, err := os.ReadFile(dir + "/" + entryName + "/values.yaml"); err == nil {
		cm.Data[v1alpha1.ValuesConfigMapKey] = string(b)
	} else {
		c.logger.Error(err, "")
	}

	for _, baseReadme := range []string{"/README.md", "/README", "/readme.md", "/readme"} {
		if b, err := os.ReadFile(dir + "/" + entryName + baseReadme); err == nil {
			cm.Data[v1alpha1.READMEConfigMapKey] = string(b)
			break
		}
	}

	if rel, err := cd.H.Template(c.options.ctx, cd.Version, dir+"/"+entryName); err == nil {
		if _, images, err := v1alpha1.GetResourcesAndImages(c.options.ctx, c.logger, c.options.client, rel.Manifest, cd.Component.Namespace); err == nil {
			cm.Data[v1alpha1.ImagesConfigMapKey] = strings.Join(images, ",")
		} else {
			c.logger.Error(err, "")
		}
	} else {
		c.logger.Error(err, "failed to template")
	}

	if needCreate {
		err = c.options.client.Create(c.options.ctx, &cm)
		if err != nil {
			c.logger.Error(err, "")
		}
		return err
	}

	if err := controllerutil.SetOwnerReference(cd.Component, &cm, cd.Scheme); err != nil {
		c.logger.Error(err, "")
		return err
	}

	err = c.options.client.Update(c.options.ctx, &cm)
	if err != nil {
		c.logger.Error(err, "")
	}
	return err
}
