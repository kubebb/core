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
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/utils/env"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
	"github.com/kubebb/core/pkg/utils"
)

var _ IWatcher = (*OCIWatcher)(nil)

func NewOCIWatcher(
	instance *v1alpha1.Repository,
	c client.Client,
	ctx context.Context,
	logger logr.Logger,
	duration time.Duration,
	cancel context.CancelFunc,
	scheme *runtime.Scheme,
	fm map[string]v1alpha1.FilterCond,
) IWatcher {
	result := &OCIWatcher{
		instance:  instance,
		logger:    logger,
		duration:  duration,
		cancel:    cancel,
		scheme:    scheme,
		repoName:  instance.NamespacedName(),
		filterMap: fm,
	}

	// Common Action in the watcher needs client and context to function
	result.c = c
	result.ctx = ctx
	return result
}

type OCIWatcher struct {
	CommonAction
	cancel   context.CancelFunc
	instance *v1alpha1.Repository
	duration time.Duration
	repoName string

	logger    logr.Logger
	scheme    *runtime.Scheme
	filterMap map[string]v1alpha1.FilterCond
}

func (c *OCIWatcher) Start() error {
	go wait.Until(c.Poll, c.duration, c.ctx.Done())
	return nil
}

func (c *OCIWatcher) Stop() {
	c.logger.Info("Delete Or Update Repository, stop watcher")
	if err := helm.RepoRemove(c.ctx, c.logger, c.repoName); err != nil {
		c.logger.Error(err, "Failed to remove repository")
	}
	c.cancel()
}

// Poll the components
func (c *OCIWatcher) Poll() {
	c.logger.Info("OCI poll")
	now := metav1.Now()
	readyCond := getReadyCond(now)
	syncCond := getSyncCond(now)

	cfg, err := ctrl.GetConfig()
	if err != nil {
		c.logger.Error(err, "Cannot get config")
		return
	}
	ns := c.instance.GetNamespace()
	getter := genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &ns,
	}

	if err = c.fetchOCIComponent(c.ctx, &getter, c.c, c.logger, ns, c.instance); err != nil {
		c.logger.Error(err, "failed to get diff")
		syncCond.Status = v1.ConditionFalse
		syncCond.Message = fmt.Sprintf("failed to get component synchronization information. %s", err.Error())
		syncCond.Reason = v1alpha1.ReasonUnavailable
	} else {
		syncCond.LastSuccessfulTime = now
	}
	updateRepository(c.ctx, c.instance, c.c, c.logger, readyCond, syncCond)
}

func (c *OCIWatcher) fetchOCIComponent(ctx context.Context, getter genericclioptions.RESTClientGetter, cli client.Client, logger logr.Logger, ns string, repo *v1alpha1.Repository) (err error) {
	repositoryURLs, err := helm.GetOCIRepoList(ctx, repo)
	if err != nil {
		return err
	}
	componentList := &v1alpha1.ComponentList{}
	if err := cli.List(ctx, componentList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{v1alpha1.ComponentRepositoryLabel: repo.GetName()}), Namespace: repo.GetNamespace()}); err != nil {
		return err
	}
	hasTag := make(map[string]map[string]bool) // Map[name]map[tag]
	existComponents := make(map[string]v1alpha1.Component)
	for _, component := range componentList.Items {
		name := component.Status.Name
		if _, ok := hasTag[name]; !ok {
			hasTag[name] = make(map[string]bool)
		}
		for _, version := range component.Status.Versions {
			hasTag[name][version.Version] = true
		}
		existComponents[name] = component
	}
	workers, err := env.GetInt("OCI_PULL_WORKER", 5) // Increase this number will download faster, but also more likely to trigger '429 Too Many Requests' error.
	if err != nil {
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)
	for i, pullURL := range repositoryURLs {
		i, pullURL := i, pullURL // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			start := time.Now()
			defer func() {
				logger.V(1).Info(fmt.Sprintf("finish handling %d/%d cost:%s", i, len(repositoryURLs), time.Since(start)), "url", pullURL)
			}()
			logger.V(1).Info(fmt.Sprintf("start handling %d/%d", i, len(repositoryURLs)), "url", pullURL)
			entryName := utils.GetOCIEntryName(pullURL)
			skip, exist := hasTag[entryName]
			warpper, err := helm.NewCoreHelmWrapper(getter, ns, logger, cli, nil, repo, nil)
			if err != nil {
				return err
			}
			latest, all, err := warpper.GetOCIRepoCharts(ctx, pullURL, skip)
			if err != nil {
				return err
			}
			if latest == nil && len(all) == 0 {
				return nil
			}
			var component v1alpha1.Component
			if !exist {
				component = v1alpha1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s.%s", c.instance.GetName(), entryName),
						Namespace: c.instance.GetNamespace(),
						Labels: map[string]string{
							v1alpha1.ComponentRepositoryLabel: c.instance.GetName(),
						},
						Annotations: map[string]string{
							v1alpha1.OCIPullURLAnnotation: pullURL,
						},
					},
					Status: v1alpha1.ComponentStatus{
						RepositoryRef: &v1.ObjectReference{
							Kind:       c.instance.Kind,
							Name:       c.instance.GetName(),
							Namespace:  c.instance.GetNamespace(),
							UID:        c.instance.GetUID(),
							APIVersion: c.instance.APIVersion,
						},
						Name:        entryName,
						DisplayName: latest.Annotations[v1alpha1.DisplayNameAnnotationKey],
						Versions:    make([]v1alpha1.ComponentVersion, 0),
						Maintainers: make([]v1alpha1.Maintainer, 0),
					},
				}
				_ = controllerutil.SetOwnerReference(c.instance, &component, c.scheme)
			} else {
				component = existComponents[entryName]
			}
			maintainers := make(map[string]v1alpha1.Maintainer)
			for _, m := range latest.Maintainers {
				if _, ok := maintainers[m.Name]; !ok {
					maintainers[m.Name] = v1alpha1.Maintainer{
						Name:  m.Name,
						Email: m.Email,
						URL:   m.URL,
					}
				}
			}
			filterVersionIndices, keep := v1alpha1.Match(c.filterMap, v1alpha1.Filter{Name: entryName, Versions: all})
			if keep {
				for _, idx := range filterVersionIndices {
					version := all[idx]
					component.Status.Versions = append(component.Status.Versions, v1alpha1.ComponentVersion{
						Annotations: version.Annotations,
						Version:     version.Version,
						AppVersion:  version.AppVersion,
						CreatedAt:   metav1.NewTime(version.Created),
						Digest:      version.Digest,
						UpdatedAt:   metav1.Now(),
						Deprecated:  version.Deprecated,
					})
				}
			}
			keywords := latest.Keywords
			if r := c.instance.Spec.KeywordLenLimit; r > 0 && len(keywords) > r {
				keywords = keywords[:r]
			}
			component.Status.Description = latest.Description
			component.Status.Home = latest.Home
			component.Status.Icon = latest.Icon
			component.Status.Keywords = keywords
			component.Status.Sources = latest.Sources
			for _, m := range maintainers {
				component.Status.Maintainers = append(component.Status.Maintainers, m)
			}
			if exist {
				c.updateComponent(component)
				delete(existComponents, component.Name)
			} else {
				c.createComponent(component)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for _, component := range existComponents {
		c.removeComponent(component)
	}
	return nil
}

func (c *OCIWatcher) createComponent(item v1alpha1.Component) {
	c.logger.Info("create component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
	if err := c.Create(&item); err != nil && !errors.IsAlreadyExists(err) {
		c.logger.Error(err, "failed to create component")
	} else {
		c.logger.Info("Successfully created component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
	}
}

func (c *OCIWatcher) updateComponent(item v1alpha1.Component) {
	c.logger.Info("update component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
	if err := c.Update(&item); err != nil {
		c.logger.Error(err, "failed to update component status")
	}
}

func (c *OCIWatcher) removeComponent(item v1alpha1.Component) {
	c.logger.Info("component is marked as deprecated", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
	if err := c.Delete(&item); err != nil {
		c.logger.Error(err, "mark the component status as deprecated has failed.")
	}
}
