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

	"github.com/go-logr/logr"
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InstallOrUpgrade installs / ungrade a helm chart to the cluster
func InstallOrUpgrade(ctx context.Context, getter genericclioptions.RESTClientGetter, cli client.Client, logger logr.Logger, cpl *corev1alpha1.ComponentPlan, repo *corev1alpha1.Repository, chartName string) (rel *release.Release, err error) {
	return installOrUpgrade(ctx, getter, cli, logger, cpl, repo, false, chartName)
}

// Uninstall installs a helm chart to the cluster
func Uninstall(ctx context.Context, getter genericclioptions.RESTClientGetter, logger logr.Logger, cpl *corev1alpha1.ComponentPlan) (err error) {
	h, err := NewHelmWrapper(getter, cpl.Namespace, logger)
	if err != nil {
		return err
	}
	rel, err := h.getLastRelease(cpl.GetReleaseName())
	if err != nil {
		return err
	}
	if rel == nil {
		return nil
	}
	if rel.Version != cpl.Status.InstalledRevision {
		return nil
	}
	return h.uninstall(logger, cpl)
}

// GetLastRelease get last release revision
func GetLastRelease(getter genericclioptions.RESTClientGetter, logger logr.Logger, cpl *corev1alpha1.ComponentPlan) (rel *release.Release, err error) {
	h, err := NewHelmWrapper(getter, cpl.Namespace, logger)
	if err != nil {
		return nil, err
	}
	return h.getLastRelease(cpl.GetReleaseName())
}

// GetManifests get helm templates
func GetManifests(ctx context.Context, getter genericclioptions.RESTClientGetter, cli client.Client, logger logr.Logger, cpl *corev1alpha1.ComponentPlan, repo *corev1alpha1.Repository, chartName string) (data string, err error) {
	rel, err := installOrUpgrade(ctx, getter, cli, logger, cpl, repo, true, chartName)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

// installOrUpgrade installs / ungrade a helm chart to the cluster
func installOrUpgrade(ctx context.Context, getter genericclioptions.RESTClientGetter, cli client.Client, logger logr.Logger, cpl *corev1alpha1.ComponentPlan, repo *corev1alpha1.Repository, dryRun bool, chartName string) (rel *release.Release, err error) {
	h, err := NewHelmWrapper(getter, cpl.Namespace, logger)
	if err != nil {
		return nil, err
	}
	rel, err = h.getLastRelease(cpl.GetReleaseName())
	if err != nil {
		return nil, err
	}
	if rel == nil {
		rel, err = h.install(ctx, logger, cli, cpl, repo, dryRun, chartName)
		if err != nil {
			return nil, err
		}
		logger.Info(fmt.Sprintf("helm install completed dryRun:%t", dryRun), ReleaseLog(rel)...)
	} else {
		logger.Info("helm find last release", ReleaseLog(rel)...)
		rel, err = h.upgrade(ctx, logger, cli, cpl, repo, dryRun, chartName)
		if err != nil {
			return nil, err
		}
		logger.Info(fmt.Sprintf("upgrade completed dryRun:%t", dryRun), ReleaseLog(rel)...)
	}
	return rel, nil
}

// ReleaseLog generates a log slice for a Helm release object.
func ReleaseLog(rel *release.Release) []interface{} {
	l := []interface{}{"release.Version", rel.Version}
	if rel.Info != nil {
		// do not show release.Info.Note which is long and not useful for controllers
		l = append(l, "release.Info.status", rel.Info.Status, "release.Info.firstDeployed", rel.Info.FirstDeployed, "release.Info.lastDeployed", rel.Info.LastDeployed, "release.Info.deleted", rel.Info.Deleted, "release.Info.description", rel.Info.Description)
	}
	return l
}
