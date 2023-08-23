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

package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentPlan controller", func() {
	Context("ComponentPlan controller test", func() {
		const ComponentPlanName = "test-componentplan"
		ctx := context.Background()
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ComponentPlanName,
				Namespace: ComponentPlanName,
			},
		}
		typeNamespaceName := types.NamespacedName{Name: ComponentPlanName, Namespace: ComponentPlanName}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			// Note: Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for ComponentPlan", func() {
			By("Creating the custom resource for the Kind ComponentPlan")
			plan := &corev1alpha1.ComponentPlan{}
			err := k8sClient.Get(ctx, typeNamespaceName, plan)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				plan := &corev1alpha1.ComponentPlan{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ComponentPlanName,
						Namespace: namespace.Name,
					},
					Spec: corev1alpha1.ComponentPlanSpec{
						ComponentRef: &corev1.ObjectReference{
							Namespace: ComponentPlanName,
							Name:      ComponentPlanName,
						},
						InstallVersion: "",
						Approved:       false,
						Config:         corev1alpha1.Config{},
					},
				}
				err = k8sClient.Create(ctx, plan)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &corev1alpha1.ComponentPlan{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			reconciler := &ComponentPlanReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))
		})
	})
})
