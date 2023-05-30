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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	hrepo "helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kubebb/core/api/v1alpha1"
)

const minIntervalSeconds = 120

var _ IWatcher = (*chartmuseum)(nil)

func NewChartmuseum(
	ctx context.Context,
	logger logr.Logger,
	c client.Client,
	scheme *runtime.Scheme,
	instance *v1alpha1.Repository,
	statusLen int,
	cancel context.CancelFunc) IWatcher {

	duration := time.Duration(minIntervalSeconds) * time.Second
	if instance.Spec.PullStategy != nil && instance.Spec.PullStategy.IntervalSeconds > minIntervalSeconds {
		duration = time.Duration(instance.Spec.PullStategy.IntervalSeconds) * time.Second
	}

	if r := time.Duration(time.Second * minIntervalSeconds); r.Milliseconds() > duration.Milliseconds() {
		logger.Info("the minimum cycle period is 120 seconds, but it is actually less, so the default 120 seconds is used as the period.")
		duration = r
	}
	return &chartmuseum{
		instance:  instance,
		c:         c,
		ctx:       ctx,
		logger:    logger,
		duration:  duration,
		cancel:    cancel,
		statusLen: statusLen,
		scheme:    scheme,
	}

}

type chartmuseum struct {
	ctx       context.Context
	cancel    context.CancelFunc
	instance  *v1alpha1.Repository
	duration  time.Duration
	statusLen int

	c      client.Client
	logger logr.Logger
	scheme *runtime.Scheme
}

func (c *chartmuseum) Start() {
	c.logger.Info("Start to fetch")
	go wait.Until(c.Poll, c.duration, c.ctx.Done())
}

func (c *chartmuseum) Stop() {
	c.logger.Info("Delete Or Update Repository, stop watcher")
	c.cancel()
}

func (c *chartmuseum) Poll() {
	cond := v1alpha1.Condition{
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}
	indexFile, err := c.fetchIndexYaml()
	if err != nil {
		c.logger.Error(err, "Failed to fetch index file")
		cond.Status = v1.ConditionFalse
		cond.Message = "failed to fetch index file"
		cond.Reason = v1alpha1.ReasonUnavailable
		cond.Type = v1alpha1.TypeFailedSync
	} else {
		cond.Message = "index yaml synced successfully, createing components"
		cond.Reason = v1alpha1.ReasonAvailable
		cond.Type = v1alpha1.TypeSynced

		diffAction, err := c.diff(indexFile)
		if err != nil {
			c.logger.Error(err, "failed to get diff")
			cond.Status = v1.ConditionFalse
			cond.Message = err.Error()
			cond.Reason = v1alpha1.ReasonUnavailable
			cond.Type = v1alpha1.TypeFailedSync
		} else {
			for _, item := range diffAction[0] {
				c.logger.Info("create component", "Component.Name", item.GetName(), "Component.Namespace", item.GetNamespace())
				if c.Create(&item); err != nil && !errors.IsAlreadyExists(err) {
					c.logger.Error(err, "failed to create")
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
	i := v1alpha1.Repository{}
	if err := c.c.Get(c.ctx, types.NamespacedName{Name: c.instance.GetName(), Namespace: c.instance.GetNamespace()}, &i); err != nil {
		c.logger.Error(err, "try to update repository, but failed to get the latest version.", "Cond", cond)
	} else {
		v1alpha1.UpdateCondWithFixedLen(c.statusLen, &i.Status.ConditionedStatus, cond)
		if err := c.c.Status().Update(c.ctx, &i); err != nil {
			c.logger.Error(err, "failed to update repository status")
		}
	}
}
func (c *chartmuseum) Create(component *v1alpha1.Component) error {
	return c.c.Create(c.ctx, component)
}

func (c *chartmuseum) Update(component *v1alpha1.Component) error {
	return c.c.Status().Update(c.ctx, component)
}

func (c *chartmuseum) Delete(component *v1alpha1.Component) error {
	return c.c.Status().Update(c.ctx, component)
}

func (c *chartmuseum) fetchIndexYaml() (*hrepo.IndexFile, error) {
	u := strings.TrimSuffix(c.instance.Spec.URL, "/") + "/index.yaml"
	c.logger.Info("Requesting", "URL", u)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		c.logger.Error(err, "")
		return nil, err
	}

	httpClient := &http.Client{}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	var (
		username, password        string
		caData, keyData, certData []byte
	)
	if c.instance.Spec.AuthSecret != "" {
		secret := v1.Secret{}
		if err := c.c.Get(c.ctx, types.NamespacedName{Namespace: c.instance.GetNamespace(), Name: c.instance.GetName()}, &secret); err != nil {
			c.logger.Error(err, "")
			return nil, err
		}

		username = string(secret.Data[v1alpha1.Username])
		password = string(secret.Data[v1alpha1.Password])
		if username != "" && password != "" {
			c.logger.Info("SetBasicAuth", "Secret", c.instance.GetNamespace()+"/"+c.instance.GetName())
			req.SetBasicAuth(username, password)
		}

		caData = secret.Data[v1alpha1.CAData]
		keyData = secret.Data[v1alpha1.KeyData]
		certData = secret.Data[v1alpha1.CertData]
	}

	if strings.HasPrefix(u, "https") {
		c.logger.Info("Skip", "TLS", c.instance.Spec.Insecure)
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: c.instance.Spec.Insecure}
		if !c.instance.Spec.Insecure {
			if len(caData) > 0 {
				x509Pool := x509.NewCertPool()
				x509Pool.AppendCertsFromPEM(caData)
				transport.TLSClientConfig.RootCAs = x509Pool
			}
			if len(keyData) > 0 && len(certData) > 0 {
				cert, err := tls.X509KeyPair(certData, keyData)
				if err != nil {
					c.logger.Error(err, "")
					return nil, err
				}
				transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
			}
		}
	}

	steps := 1
	if c.instance.Spec.PullStategy != nil {
		if c.instance.Spec.PullStategy.TimeoutSeconds > 0 {
			httpClient.Timeout = time.Duration(c.instance.Spec.PullStategy.TimeoutSeconds) * time.Second
			c.logger.Info("Set HTTP Client", "Timeout", httpClient.Timeout)
		}
		if c.instance.Spec.PullStategy.Retry > 0 {
			steps = c.instance.Spec.PullStategy.Retry
		}
	}

	httpClient.Transport = transport
	repositories := hrepo.IndexFile{}
	if err := retry.OnError(wait.Backoff{Factor: 1.0, Steps: steps, Duration: time.Second * 5}, func(error) bool {
		return true
	}, func() error {
		resp, err := httpClient.Do(req)
		if err != nil {
			c.logger.Error(err, "do request error")
			return err
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Error(err, "read response error")
			return err
		}

		if err = yaml.Unmarshal(data, &repositories); err != nil {
			c.logger.Error(err, "unmarshal response body error")
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("timeout, exceeds maximum number of attempts %d", steps)
	}
	return &repositories, nil
}

func (c *chartmuseum) indexFileToComponent(indexFile *hrepo.IndexFile) []v1alpha1.Component {
	components := make([]v1alpha1.Component, len(indexFile.Entries))
	index := 0
	for entryName, versions := range indexFile.Entries {
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
		for _, version := range versions {
			if !v1alpha1.Match(c.instance.Spec.Filter, v1alpha1.Filter{Name: entryName, Version: version.Version, Deprecated: version.Deprecated}) {
				continue
			}
			components[index].Status.Versions = append(components[index].Status.Versions, v1alpha1.ComponentVersion{
				Version:    version.Version,
				AppVersion: version.AppVersion,
				CreatedAt:  metav1.NewTime(version.Created),
				Digest:     version.Digest,
				UpdatedAt:  metav1.Now(),
				Deprecated: version.Deprecated,
			})

			for _, m := range version.Maintainers {
				if _, ok := maintainers[m.Name]; !ok {
					maintainers[m.Name] = v1alpha1.Maintainer{
						Name:  m.Name,
						Email: m.Email,
						URL:   m.URL,
					}
				}
			}
			if latest {
				components[index].Status.Description = version.Description
				components[index].Status.Home = version.Home
				components[index].Status.Icon = version.Icon
				components[index].Status.Keywords = version.Keywords
				components[index].Status.Sources = version.Sources
				latest = false
			}
		}
		for _, m := range maintainers {
			components[index].Status.Maintainers = append(components[index].Status.Maintainers, m)
		}

		controllerutil.SetOwnerReference(c.instance, &components[index], c.scheme)
		index++
	}
	return components
}

// diff This function gets the new Component, the updated Component,
// and the Component that needs to be marked for deletion based on the list of charts obtained from the given link
// compared to the already existing Components in the cluster
func (c *chartmuseum) diff(indexFile *hrepo.IndexFile) ([3][]v1alpha1.Component, error) {
	targetComponents := c.indexFileToComponent(indexFile)
	exitComponents := v1alpha1.ComponentList{}
	if err := c.c.List(c.ctx, &exitComponents, &client.ListOptions{
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

	for _, component := range exitComponents.Items {
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
