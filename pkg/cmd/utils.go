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

package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WaitDeployment(ctx context.Context, c client.Client, namespace string, deployNames []string, timeourSeconds int) error {
	if len(deployNames) == 0 {
		return nil
	}

	steps := timeourSeconds / 5
	if timeourSeconds%5 != 0 {
		steps++
	}

	backOff := wait.Backoff{
		Steps:    steps,
		Duration: time.Second * 5,
	}

	result := make(chan struct{}, len(deployNames))
	var wg sync.WaitGroup
	for _, name := range deployNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			dep := appsv1.Deployment{}
			if err := retry.OnError(backOff, func(error) bool {
				return true
			}, func() error {
				if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &dep); err != nil {
					fmt.Printf("\t in namespace: %s not found deployment %s, wait\n", namespace, name)
					return err
				}
				fmt.Printf("\tname: %s availableReplicas: %d --- replicas: %d\n", name, dep.Status.AvailableReplicas, *dep.Spec.Replicas)
				if dep.Status.AvailableReplicas == *dep.Spec.Replicas {
					return nil
				}
				return fmt.Errorf("deployment %s doest not match condition. availableReplicas: %d, replicas: %d",
					name, dep.Status.AvailableReplicas, *dep.Spec.Replicas)
			}); err != nil {
				result <- struct{}{}
			}
		}(name)
	}
	wg.Wait()
	if len(result) > 0 {
		return fmt.Errorf("not all deployment match condition")
	}

	return nil
}
