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

package repository

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/repository/mock"
)

func TestHTTPWatcherStartFailed(t *testing.T) {
	builder := fake.NewClientBuilder()

	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubebb",
			Namespace: "default",
		},
		Spec: v1alpha1.RepositorySpec{
			URL: "http://localhost:8001/a",
			PullStategy: &v1alpha1.PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},
		},
	}
	builder.WithRuntimeObjects(repo)
	builder.WithScheme(scheme)

	c := builder.Build()
	backgroundCtx := context.Background()
	logger, _ := logr.FromContext(backgroundCtx)
	ctx, cancel := context.WithCancel(backgroundCtx)
	w := NewWatcher(ctx, logger, c, scheme, repo, cancel)
	if err := w.Start(); err == nil {
		t.Fatalf("Test failed. expect watcher start failed.")
	}
}
func TestHTTPWatcher(t *testing.T) {
	builder := fake.NewClientBuilder()

	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubebb",
			Namespace: "default",
		},
		Spec: v1alpha1.RepositorySpec{
			URL: "http://localhost:8001/a",
			PullStategy: &v1alpha1.PullStategy{
				IntervalSeconds: 120,
				Retry:           5,
			},
			AuthSecret: "auth",
		},
	}
	u := base64.StdEncoding.EncodeToString([]byte("username"))
	p := base64.StdEncoding.EncodeToString([]byte("password"))
	srv := mock.NewMockChartServerWithBasicAuth(8001, "username", "password")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth",
			Namespace: "default",
		},
		Data: map[string][]byte{
			v1alpha1.Username: []byte(u),
			v1alpha1.Password: []byte(p),
		},
	}
	builder.WithRuntimeObjects(repo, secret)
	builder.WithScheme(scheme)

	c := builder.Build()
	backgroundCtx := context.Background()
	logger, _ := logr.FromContext(backgroundCtx)
	ctx, cancel := context.WithCancel(backgroundCtx)
	w := NewWatcher(ctx, logger, c, scheme, repo, cancel)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("server failed to start")
			return
		}
	}()

	defer func() {
		_ = srv.Shutdown(ctx)
		w.Stop()
	}()
	_ = w.Start()

	time.Sleep(5 * time.Second)
	t.Log("1. sleep for 5 seconds, wait until the first synchronization is successful, and check the data")
	time.Sleep(5 * time.Second)
	componentList := v1alpha1.ComponentList{}
	if err := c.List(ctx, &componentList, client.InNamespace("default")); err != nil {
		t.Fatalf("get component list failed. error: %v", err)
	}
	t.Log("1.1 check the number of components, expected one")
	if len(componentList.Items) != 1 {
		t.Fatalf("epxected 1 component, but actually %d", len(componentList.Items))
	}
	t.Log("1.2 check the component version number, expected one")
	if len(componentList.Items[0].Status.Versions) != 1 {
		t.Fatalf("expected only one version of component, but actually there are %d", len(componentList.Items[0].Status.Versions))
	}

	t.Log("2. sleep for 120 seconds, wait for the next synchronization cycle, and check the data")
	time.Sleep(120 * time.Second)
	componentList = v1alpha1.ComponentList{}
	if err := c.List(ctx, &componentList, client.InNamespace("default")); err != nil {
		t.Fatalf("get component list failed. error: %v", err)
	}
	t.Log("2.1 check the number of components, expected one")
	if len(componentList.Items) != 1 {
		t.Fatalf("epxected 1 component, but actually %d", len(componentList.Items))
	}
	t.Log("2.2 check the component version number, expected two")
	if len(componentList.Items[0].Status.Versions) != 2 {
		t.Fatalf("expected only one version of component, but actually there are %d", len(componentList.Items[0].Status.Versions))
	}
}
