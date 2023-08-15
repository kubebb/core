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
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
)

type IWatcher interface {
	Switch
	Action
}

func NewWatcher(
	ctx context.Context,
	logger logr.Logger,
	c client.Client,
	scheme *runtime.Scheme,
	instance *v1alpha1.Repository,
	cancel context.CancelFunc,
) IWatcher {
	duration, fm := getWatcherValues(logger, instance)
	logger.Info("Create watcher with " + instance.Spec.URL)
	logger.Info(fmt.Sprint(strings.HasPrefix(instance.Spec.URL, "oci")))
	if strings.HasPrefix(instance.Spec.URL, "oci") {
		return NewOCIWatcher(instance, c, ctx, logger, duration, cancel, scheme)
	} else {
		return NewHTTPWatcher(instance, c, ctx, logger, duration, cancel, scheme, fm)
	}
}

func getWatcherValues(logger logr.Logger, instance *v1alpha1.Repository) (time.Duration, map[string]v1alpha1.FilterCond) {
	duration := time.Duration(minIntervalSeconds) * time.Second
	if instance.Spec.PullStategy != nil && instance.Spec.PullStategy.IntervalSeconds > minIntervalSeconds {
		duration = time.Duration(instance.Spec.PullStategy.IntervalSeconds) * time.Second
	}

	if r := time.Second * minIntervalSeconds; r.Milliseconds() > duration.Milliseconds() {
		logger.Info("the minimum cycle period is 120 seconds, but it is actually less, so the default 120 seconds is used as the period.")
		duration = r
	}
	fm := make(map[string]v1alpha1.FilterCond)
	for _, f := range instance.Spec.Filter {
		fm[f.Name] = f
	}
	return duration, fm
}

type Switch interface {
	Start() error
	Stop()
	Poll()
}

// Start the Watcher
func Start(ctx context.Context, instance *v1alpha1.Repository, duration time.Duration, repoName string, c client.Client, logger logr.Logger) (string, string, error) {
	logger.Info("Start to fetch")
	var username, password string
	_ = helm.RepoRemove(ctx, logger, repoName)
	if instance.Spec.AuthSecret != "" {
		i := v1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Name: instance.Spec.AuthSecret, Namespace: instance.GetNamespace()}, &i); err != nil {
			logger.Error(err, "Failed to get secret")
			return "", "", err
		}
		username = string(i.Data[v1alpha1.Username])
		password = string(i.Data[v1alpha1.Password])
	}
	return username, password, nil
}

// Action defines the actiosn required to control the components
type Action interface {
	Create(component *v1alpha1.Component) error
	Update(component *v1alpha1.Component) error
	Delete(component *v1alpha1.Component) error
}

// CommonAction shared by the watchers
type CommonAction struct {
	ctx context.Context
	c   client.Client
}

// Create the component and update it in the k8s client
func (c *CommonAction) Create(component *v1alpha1.Component) error {
	status := component.Status
	if err := c.c.Create(c.ctx, component); err != nil {
		return err
	}
	component.Status = status
	return c.Update(component)
}

// Update the component in the k8s client
func (c *CommonAction) Update(component *v1alpha1.Component) error {
	return c.c.Status().Update(c.ctx, component)
}

// Delete the component in the k8s client
func (c *CommonAction) Delete(component *v1alpha1.Component) error {
	return c.Update(component)
}

// getReadyCond gets the initial ready condition
func getReadyCond(now metav1.Time) v1alpha1.Condition {
	return v1alpha1.Condition{
		Status:             v1.ConditionTrue,
		LastTransitionTime: now,
		Message:            "",
		Type:               v1alpha1.TypeReady,
	}
}

// getSyncCond gets the initial sync condition
func getSyncCond(now metav1.Time) v1alpha1.Condition {
	return v1alpha1.Condition{
		Status:             v1.ConditionTrue,
		LastTransitionTime: now,
		Message:            "index yaml synced successfully, creating components",
		Type:               v1alpha1.TypeSynced,
	}
}

// updateRepository updates the repository
func updateRepository(ctx context.Context, instance *v1alpha1.Repository, c client.Client, logger logr.Logger, readyCond, syncCond v1alpha1.Condition) {
	i := v1alpha1.Repository{}
	if err := c.Get(ctx, types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}, &i); err != nil {
		logger.Error(err, "try to update repository, but failed to get the latest version.", "readyCond", readyCond, "syncCond", syncCond)
	} else {
		iDeepCopy := i.DeepCopy()
		// If the LastSuccessfulTime is empty, it means that this synchronization failed,
		// find the time when the latest synchronization was successful.
		if syncCond.LastSuccessfulTime.IsZero() {
			for _, cond := range iDeepCopy.Status.Conditions {
				if cond.Type == v1alpha1.TypeSynced && !cond.LastSuccessfulTime.IsZero() {
					syncCond.LastSuccessfulTime = cond.LastSuccessfulTime
					break
				}
			}
		}
		iDeepCopy.Status.SetConditions(readyCond, syncCond)
		if err := c.Status().Patch(ctx, iDeepCopy, client.MergeFrom(&i)); err != nil {
			logger.Error(err, "failed to patch repository status")
		}
	}
}
