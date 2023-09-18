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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	hrepo "helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
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
	// version match, the chart is exactly installed by this componentplan
	if rel.Version == cpl.Status.InstalledRevision {
		return h.uninstall(logger, cpl)
	}
	// descrpiton match, the chart is exactly installed by this componentplan
	if ns, name, uid, _, _ := ParseDescription(rel.Info.Description); ns == cpl.Namespace && name == cpl.Name && uid == string(cpl.GetUID()) {
		return h.uninstall(logger, cpl)
	}
	// when installing/upgrading/rollingback, description will be setted by helm, so we compare helm release version and component status
	if cpl.Status.InstalledRevision+1 == rel.Version {
		if cpl.IsActionedReason(corev1alpha1.ComponentPlanReasonInstalling) || cpl.IsActionedReason(corev1alpha1.ComponentPlanReasonUpgrading) || cpl.IsActionedReason(corev1alpha1.ComponentPlanReasonRollingBack) {
			return h.uninstall(logger, cpl)
		}
	}
	return nil
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

func RollBack(ctx context.Context, getter genericclioptions.RESTClientGetter, logger logr.Logger, cpl *corev1alpha1.ComponentPlan) (err error) {
	h, err := NewHelmWrapper(getter, cpl.Namespace, logger)
	if err != nil {
		return err
	}
	return h.rollback(logger, cpl)
}

// GetOCIRepoCharts retrieves the latest chart metadata and all component versions for a given OCI repository.
func GetOCIRepoCharts(ctx context.Context, getter genericclioptions.RESTClientGetter, cli client.Client, logger logr.Logger, ns, pullURL string, repo *corev1alpha1.Repository, skipTags map[string]bool) (latest *chart.Metadata, all []*hrepo.ChartVersion, err error) {
	h, err := NewHelmWrapper(getter, ns, logger)
	if err != nil {
		return nil, nil, err
	}
	tags, err := h.config.RegistryClient.Tags(strings.TrimPrefix(pullURL, "oci://"))
	if err != nil {
		return nil, nil, err
	}
	if len(tags) == 0 {
		return nil, nil, nil
	}
	latestOne := tags[0]
	if skipTags[latestOne] {
		return nil, nil, nil
	}
	for i, tag := range tags {
		if skipTags[tag] {
			continue // It is ok for deprecated chart. Because once a chart is deprecated the expectation is the chart will see no further development. The Version will increase. All charts are immutable.
		}
		out, c, err := h.PullAndParse(ctx, logger, cli, repo, pullURL, tag)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			latest = c.Metadata
		}
		all = append(all, &hrepo.ChartVersion{
			Metadata: c.Metadata,
			URLs:     nil,
			Created:  time.Now(),
			Removed:  false,
			Digest:   ParseDigestFromPullOut(out),
		})
	}
	return latest, all, nil
}
