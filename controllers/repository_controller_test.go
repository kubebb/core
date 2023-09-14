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

package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/repository/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repository controller", Ordered, func() {
	const (
		repoAname      = "repoa"
		repoAnamespace = "kube-system"
		repoBname      = "repob"
		repoBnamespace = "default"
	)
	var (
		err     error
		rootCtx = context.Background()
		ctx     context.Context
		cancel  context.CancelFunc

		srv = mock.NewMockChartServer(8000)
	)

	BeforeAll(func() {
		ctx, cancel = context.WithCancel(rootCtx)
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				Expect(err).ToNot(HaveOccurred())
			}
		}()
	})

	AfterAll(func() {
		if err = srv.Shutdown(ctx); err != nil {
			Expect(err).ToNot(HaveOccurred())
		}
		cancel()
	})

	Context("Repository controller", func() {
		desc := "Create repository, to test controller polling. It can be created successfully. Only one component is included at the same time. In the first time, only one version is included. In the next cycle, two versions will appear."
		It(desc, func() {
			repo := v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repoAname,
					Namespace: repoAnamespace,
				},
				Spec: v1alpha1.RepositorySpec{
					URL: "http://127.0.0.1:8000/a",
					PullStategy: &v1alpha1.PullStategy{
						IntervalSeconds: 120,
						Retry:           5,
					},
				},
			}
			By("create repository repoa")
			if err = k8sClient.Create(ctx, &repo); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}
			By("there should be only one component and only one version")
			repoName := repoAname + ".chartmuseum"
			if err = retry.OnError(wait.Backoff{Factor: 1.0, Steps: 3, Duration: time.Second * 10}, func(err error) bool {
				return err != nil
			}, func() error {
				component := v1alpha1.Component{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: repoAnamespace, Name: repoName}, &component); err != nil {
					return err
				}
				if r := len(component.Status.Versions); r != 1 {
					return fmt.Errorf("expect only one version, but actuall is %d", r)
				}
				return nil
			}); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}

			By("there should be a component with two versions")
			time.Sleep(time.Second * 120)
			if err = retry.OnError(wait.Backoff{Factor: 1.0, Steps: 3, Duration: time.Second * 10}, func(err error) bool {
				return err != nil
			}, func() error {
				component := v1alpha1.Component{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: repoAnamespace, Name: repoName}, &component); err != nil {
					return err
				}
				if r := len(component.Status.Versions); r != 2 {
					return fmt.Errorf("expect two versions, but actuall is %d", r)
				}
				return nil
			}); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}
			fmt.Println("step1 done")
		})

		It(`Create a repository and accurately filter out chartmuseum and filter out obsolete versions of minio. It is guaranteed that there will be only minio in the end, and only one version.`, func() {
			repo := v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repoBname,
					Namespace: repoBnamespace,
				},
				Spec: v1alpha1.RepositorySpec{
					URL: "http://127.0.0.1:8000/b",
					PullStategy: &v1alpha1.PullStategy{
						IntervalSeconds: 120,
						Retry:           5,
					},
					Filter: []v1alpha1.FilterCond{
						{
							Name:      "chartmuseum",
							Operation: v1alpha1.FilterOpIgnore,
						},
						{
							Name:           "minio",
							KeepDeprecated: false,
							Operation:      v1alpha1.FilterOpKeep,
						},
					},
				},
			}
			By("create repository repob")
			if err = k8sClient.Create(ctx, &repo); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}

			By("Finally check, there should be only one component and only one version")
			repoName := repoBname + ".minio"
			if err = retry.OnError(wait.Backoff{Factor: 1.0, Steps: 3, Duration: time.Second * 10}, func(err error) bool {
				return err != nil
			}, func() error {
				component := v1alpha1.Component{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: repoBnamespace, Name: repoName}, &component); err != nil {
					return err
				}
				if r := len(component.Status.Versions); r != 1 {
					return fmt.Errorf("expect only one version, but actuall is %d", r)
				}
				return nil
			}); err != nil {
				Expect(err).ToNot(HaveOccurred())
			}
			fmt.Println("step2 done")
		})
	})
})
