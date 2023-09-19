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
	"os"
	"path/filepath"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/helmpath"
	hrepo "helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
)

const (
	minIntervalSeconds = 120
)

var _ IWatcher = (*HTTPWatcher)(nil)

func NewHTTPWatcher(
	instance *v1alpha1.Repository,
	c client.Client,
	ctx context.Context,
	logger logr.Logger,
	duration time.Duration,
	cancel context.CancelFunc,
	scheme *runtime.Scheme,
	fm map[string]v1alpha1.FilterCond,
) IWatcher {
	result := &HTTPWatcher{
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

type HTTPWatcher struct {
	CommonAction
	cancel   context.CancelFunc
	instance *v1alpha1.Repository
	duration time.Duration
	repoName string

	logger    logr.Logger
	scheme    *runtime.Scheme
	filterMap map[string]v1alpha1.FilterCond
}

func (c *HTTPWatcher) Start() error {
	entry, err := Start(c.ctx, c.instance, c.duration, c.repoName, c.c, c.logger)
	if err != nil {
		return err
	}
	entry.Name = c.repoName
	entry.URL = c.instance.Spec.URL

	if err := helm.RepoAdd(c.ctx, c.logger, entry, c.duration/2); err != nil {
		c.logger.Error(err, "Failed to add repository")
		now := metav1.Now()
		readyCond := getReadyCond(now)
		syncCond := getSyncCond(now)
		readyCond.Status = v1.ConditionFalse
		readyCond.Message = fmt.Sprintf("failed to add repo %s", err.Error())
		readyCond.Reason = v1alpha1.ReasonUnavailable

		syncCond.Status = v1.ConditionFalse
		syncCond.Message = fmt.Sprintf("failed to add repo %s", err.Error())
		syncCond.Reason = v1alpha1.ReasonUnavailable

		updateRepository(c.ctx, c.instance, c.c, c.logger, readyCond, syncCond)
		return err
	}

	go wait.Until(c.Poll, c.duration, c.ctx.Done())
	return nil
}

func (c *HTTPWatcher) Stop() {
	c.logger.Info("Delete Or Update Repository, stop watcher")
	if err := helm.RepoRemove(c.ctx, c.logger, c.repoName); err != nil {
		c.logger.Error(err, "Failed to remove repository")
	}
	c.cancel()
}

// Poll the components
func (c *HTTPWatcher) Poll() {
	c.logger.Info("HTTP poll")
	now := metav1.Now()
	readyCond := getReadyCond(now)
	syncCond := getSyncCond(now)

	if err := helm.RepoUpdate(c.ctx, c.logger, c.repoName, c.duration/2); err != nil {
		c.logger.Error(err, "Failed to update repository")
		readyCond.Status = v1.ConditionFalse
		readyCond.Message = fmt.Sprintf("failed to update repo %s", err.Error())
		readyCond.Reason = v1alpha1.ReasonUnavailable

		syncCond.Status = v1.ConditionFalse
		syncCond.Message = fmt.Sprintf("failed to update repo %s", err.Error())
		syncCond.Reason = v1alpha1.ReasonUnavailable
		updateRepository(c.ctx, c.instance, c.c, c.logger, readyCond, syncCond)
		return
	}

	indexFile, err := c.fetchIndexYaml()
	if err != nil {
		c.logger.Error(err, "Failed to fetch index file")
		readyCond.Status = v1.ConditionFalse
		readyCond.Message = fmt.Sprintf("failed to fetch index file. %s", err.Error())
		readyCond.Reason = v1alpha1.ReasonUnavailable

		syncCond.Status = v1.ConditionFalse
		syncCond.Message = "failed to get index.yaml and could not sync components"
		syncCond.Reason = v1alpha1.ReasonUnavailable
	} else {
		if diffAction, err := c.diff(indexFile); err != nil {
			c.logger.Error(err, "failed to get diff")
			syncCond.Status = v1.ConditionFalse
			syncCond.Message = fmt.Sprintf("failed to get component synchronization information. %s", err.Error())
			syncCond.Reason = v1alpha1.ReasonUnavailable
		} else {
			syncCond.LastSuccessfulTime = now
			for _, item := range diffAction[0] {
				c.logger.Info("create component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
				if err := c.Create(&item); err != nil && !errors.IsAlreadyExists(err) {
					c.logger.Error(err, "failed to create component")
				} else {
					c.logger.Info("Successfully created component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
				}
			}
			for _, item := range diffAction[1] {
				c.logger.Info("update component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
				if err := c.Update(&item); err != nil {
					c.logger.Error(err, "failed to update component status")
				}
			}
			for _, item := range diffAction[2] {
				c.logger.Info("component is marked as deprecated", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
				if err := c.Delete(&item); err != nil {
					c.logger.Error(err, "mark the component status as deprecated has failed.")
				}
			}
		}
	}

	updateRepository(c.ctx, c.instance, c.c, c.logger, readyCond, syncCond)
}

// fetchIndexYaml get the index.yaml file
func (c *HTTPWatcher) fetchIndexYaml() (*hrepo.IndexFile, error) {
	var settings = cli.New()
	repoCache := settings.RepositoryCache
	fname := filepath.Join(repoCache, helmpath.CacheIndexFile(c.repoName))
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	repositories := hrepo.IndexFile{}
	if err = yaml.Unmarshal(data, &repositories); err != nil {
		return nil, err
	}
	return &repositories, nil
}

// indexFileToComponent gets the component from the index file
func (c *HTTPWatcher) indexFileToComponent(indexFile *hrepo.IndexFile) []v1alpha1.Component {
	components := make([]v1alpha1.Component, len(indexFile.Entries))
	index := 0

	for entryName, versions := range indexFile.Entries {
		// filter component and its versions
		filterVersionIndices, keep := v1alpha1.Match(c.filterMap, v1alpha1.Filter{Name: entryName, Versions: versions})
		if !keep {
			continue
		}
		components[index] = v1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%s", c.instance.GetName(), entryName),
				Namespace: c.instance.GetNamespace(),
				Labels: map[string]string{
					v1alpha1.ComponentRepositoryLabel: c.instance.GetName(),
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
				Versions:    make([]v1alpha1.ComponentVersion, 0),
				Maintainers: make([]v1alpha1.Maintainer, 0),
			},
		}

		maintainers := make(map[string]v1alpha1.Maintainer)
		latest := true
		for _, idx := range filterVersionIndices {
			version := versions[idx]

			for _, m := range version.Maintainers {
				if _, ok := maintainers[m.Name]; !ok {
					maintainers[m.Name] = v1alpha1.Maintainer{
						Name:  m.Name,
						Email: m.Email,
						URL:   m.URL,
					}
				}
			}
			components[index].Status.Versions = append(components[index].Status.Versions, v1alpha1.ComponentVersion{
				Annotations: version.Annotations,
				Version:     version.Version,
				AppVersion:  version.AppVersion,
				CreatedAt:   metav1.NewTime(version.Created),
				Digest:      version.Digest,
				UpdatedAt:   metav1.Now(),
				Deprecated:  version.Deprecated,
				URLs:        version.URLs,
			})

			if latest {
				keywords := version.Keywords
				if r := c.instance.Spec.KeywordLenLimit; r > 0 && len(keywords) > r {
					keywords = keywords[:r]
				}
				components[index].Status.Description = version.Description
				components[index].Status.Home = version.Home
				components[index].Status.Icon = version.Icon
				components[index].Status.Keywords = keywords
				components[index].Status.Sources = version.Sources
				if version.Annotations != nil {
					// update displayName based on the annotation from latest version
					components[index].Status.DisplayName = version.Annotations[v1alpha1.DisplayNameAnnotationKey]
				}
				latest = false
			}
		}
		for _, m := range maintainers {
			components[index].Status.Maintainers = append(components[index].Status.Maintainers, m)
		}

		_ = controllerutil.SetOwnerReference(c.instance, &components[index], c.scheme)
		index++
	}
	return components[:index]
}

// diff This function gets the new Component, the updated Component,
// and the Component that needs to be marked for deletion based on the list of charts obtained from the given link
// compared to the already existing Components in the cluster
func (c *HTTPWatcher) diff(indexFile *hrepo.IndexFile) ([3][]v1alpha1.Component, error) {
	targetComponents := c.indexFileToComponent(indexFile)
	existComponents := v1alpha1.ComponentList{}
	if err := c.c.List(c.ctx, &existComponents, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			v1alpha1.ComponentRepositoryLabel: c.instance.GetName(),
		}),
		Namespace: c.instance.GetNamespace(),
	}); err != nil {
		return [3][]v1alpha1.Component{}, err
	}

	targetComponentMap := make(map[string]v1alpha1.Component)
	for _, component := range targetComponents {
		targetComponentMap[fmt.Sprintf("%s/%s", component.GetNamespace(), component.GetName())] = component
	}

	addComonent := make([]v1alpha1.Component, 0)
	updateComponent := make([]v1alpha1.Component, 0)
	delComponent := make([]v1alpha1.Component, 0)

	for _, component := range existComponents.Items {
		key := fmt.Sprintf("%s/%s", component.GetNamespace(), component.GetName())
		tmp, ok := targetComponentMap[key]
		if !ok {
			c.logger.Info("should mark component as deleted", "Component.Name", key)
			component.Status.Deprecated = true
			delComponent = append(delComponent, component)
			continue
		}
		delete(targetComponentMap, key)
		// If the version lengths of the two are different or a Component in the cluster is marked as deprecated, its status should be updated.
		if len(component.Status.Versions) != len(tmp.Status.Versions) || component.Status.Deprecated {
			component.Status = tmp.Status
			updateComponent = append(updateComponent, component)
			continue
		}

		for _, v := range component.Status.Versions {
			found := false
			for _, v1 := range tmp.Status.Versions {
				if v.Digest == v1.Digest {
					found = true
					break
				}
			}
			if !found {
				component.Status = tmp.Status
				updateComponent = append(updateComponent, component)
				break
			}
		}
	}
	for _, component := range targetComponentMap {
		addComonent = append(addComonent, component)
	}
	return [3][]v1alpha1.Component{addComonent, updateComponent, delComponent}, nil
}
