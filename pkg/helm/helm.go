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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/utils"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This make user reuse all helm environment variables, like HELM_PLUGINS etc.
var settings = cli.New()

// HelmWrapper is a wrapper for helm command
type HelmWrapper struct {
	config *action.Configuration
}

// NewHelmWrapper returns a new helmWrapper instance
func NewHelmWrapper(getter genericclioptions.RESTClientGetter, namespace string, logger logr.Logger) (*HelmWrapper, error) {
	cfg := new(action.Configuration)
	if err := cfg.Init(getter, namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		logger.V(1).Info(fmt.Sprintf(format, v...))
	}); err != nil {
		return nil, err
	}
	return &HelmWrapper{
		config: cfg,
	}, nil
}

// install
// inspire by https://github.com/helm/helm/blob/main/cmd/helm/install.go
func (h *HelmWrapper) install(ctx context.Context, logger logr.Logger, cli client.Client, cpl *corev1alpha1.ComponentPlan, repo *corev1alpha1.Repository, dryRun bool, chartName string) (rel *release.Release, err error) {
	log := logger.WithValues("ComponentPlan", klog.KObj(cpl))
	if dryRun {
		log.WithValues("dryRun", true)
	}
	i := action.NewInstall(h.config)
	i.CreateNamespace = false // installed in the same namespace with ComponentPlan
	i.DryRun = dryRun
	i.DisableHooks = cpl.Spec.DisableHooks
	i.Replace = false // reuse the given name is not safe in production
	i.Timeout = cpl.Spec.Timeout()
	i.Wait = cpl.Spec.Wait
	i.WaitForJobs = cpl.Spec.WaitForJobs
	i.GenerateName = false // we use ComponentPlan.Spec.Name
	i.NameTemplate = ""    // we use ComponentPlan.Spec.Name
	i.Description = cpl.Spec.Description
	i.Devel = false // just set `>0.0.0-0` to ComponentPlan.Spec.Version is enough
	i.DependencyUpdate = true
	i.DisableOpenAPIValidation = cpl.Spec.DisableOpenAPIValidation
	i.Atomic = cpl.Spec.Atomic
	i.SkipCRDs = cpl.Spec.SkipCRDs
	i.SubNotes = false // we cant see notes or subnotes

	i.Version = cpl.Spec.InstallVersion
	i.Verify = false // TODO enable these args after we can config keyring
	i.RepoURL = repo.Spec.URL
	i.Keyring = "" // TODO enable these args after we can config keyring
	// i.Username // FIXME
	// i.Password // FIXME
	// i.CertFile // FIXME
	// i.KeyFile // FIXME
	// i.CaFile // FIXME
	i.InsecureSkipTLSverify = repo.Spec.Insecure
	i.PassCredentialsAll = false // TODO do we need add this args to override?
	i.PostRenderer = newPostRenderer(repo.Spec.ImageOverride, cpl.Spec.Override.Images)
	log.V(1).Info(fmt.Sprintf("Original chart version: %q", i.Version))

	i.ReleaseName = cpl.GetReleaseName()
	chartRequested, vals, err := h.prepare(ctx, cli, log, i.ChartPathOptions, cpl, chartName)
	if err != nil {
		return nil, err
	}
	i.Namespace = cpl.Namespace
	if dryRun {
		i.ClientOnly = true
	}
	return i.RunWithContext(ctx, chartRequested, vals)
}

// upgrade
// inspire by https://github.com/helm/helm/blob/main/cmd/helm/upgrade.go
func (h *HelmWrapper) upgrade(ctx context.Context, logger logr.Logger, cli client.Client, cpl *corev1alpha1.ComponentPlan, repo *corev1alpha1.Repository, dryRun bool, chartName string) (rel *release.Release, err error) {
	log := logger.WithValues("ComponentPlan", klog.KObj(cpl))
	if dryRun {
		log.WithValues("dryRun", true)
	}
	i := action.NewUpgrade(h.config)
	i.Install = false // we just want to upgrade, for install, we use Install function
	i.Devel = false   // just set `>0.0.0-0` to ComponentPlan.Spec.Version is enough
	i.DryRun = dryRun
	i.Force = cpl.Spec.Force
	i.DisableHooks = cpl.Spec.DisableHooks
	i.DisableOpenAPIValidation = cpl.Spec.DisableOpenAPIValidation
	i.SkipCRDs = cpl.Spec.SkipCRDs
	i.Timeout = cpl.Spec.Timeout()
	i.ResetValues = false // dont reset value to default
	i.ReuseValues = false // dont reuse value
	i.Wait = cpl.Spec.Wait
	i.WaitForJobs = cpl.Spec.WaitForJobs
	i.Atomic = cpl.Spec.Atomic
	i.MaxHistory = cpl.Spec.GetMaxHistory()
	i.CleanupOnFail = cpl.Spec.CleanupOnFail
	i.SubNotes = false // we cant see notes or subnotes
	i.Description = cpl.Spec.Description
	i.DependencyUpdate = true

	i.Version = cpl.Spec.InstallVersion
	i.Verify = false // TODO enable these args after we can config keyring
	i.RepoURL = repo.Spec.URL
	i.Keyring = "" // TODO enable these args after we can config keyring
	// i.Username // FIXME
	// i.Password // FIXME
	// i.CertFile // FIXME
	// i.KeyFile // FIXME
	// i.CaFile // FIXME
	i.InsecureSkipTLSverify = repo.Spec.Insecure
	i.PassCredentialsAll = false // TODO do we need add this args to override?
	i.PostRenderer = newPostRenderer(repo.Spec.ImageOverride, cpl.Spec.Override.Images)

	chartRequested, vals, err := h.prepare(ctx, cli, log, i.ChartPathOptions, cpl, chartName)
	if err != nil {
		return nil, err
	}
	i.Namespace = cpl.Namespace
	return i.RunWithContext(ctx, cpl.GetReleaseName(), chartRequested, vals)
}

// uninstall
// inspire by https://github.com/helm/helm/blob/main/cmd/helm/uninstall.go
func (h *HelmWrapper) uninstall(logger logr.Logger, cpl *corev1alpha1.ComponentPlan) (err error) {
	log := logger.WithValues("ComponentPlan", klog.KObj(cpl))
	i := action.NewUninstall(h.config)
	i.DryRun = false // do not need to simulate the installation
	i.DisableHooks = cpl.Spec.DisableHooks
	i.KeepHistory = cpl.Spec.KeepHistory
	i.Timeout = cpl.Spec.Timeout()
	i.Wait = cpl.Spec.Wait
	res, err := i.Run(cpl.GetReleaseName())
	if err != nil {
		return err
	}
	if res != nil && res.Info != "" {
		log.Info(res.Info)
	}
	log.Info(fmt.Sprintf("release \"%s\" uninstalled", cpl.GetReleaseName()))
	return nil
}

// getLastRelease observes the last revision
func (h *HelmWrapper) getLastRelease(releaseName string) (*release.Release, error) {
	rel, err := h.config.Releases.Last(releaseName)
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		err = nil
	}
	return rel, err
}

// chartDownload uses the chartName to download the chart
// The Repo url is stored in the ChartPathOptions
func (h *HelmWrapper) chartDownload(chartName string, logger logr.Logger, i action.ChartPathOptions) (string, error) {
	buf := new(strings.Builder)
	defer func() {
		for _, l := range strings.Split(buf.String(), "\n") {
			logger.V(1).Info(l, "chartName", chartName)
		}
	}()
	dl := downloader.ChartDownloader{
		Out:     buf,
		Keyring: i.Keyring,
		Getters: getter.All(settings),
		Options: []getter.Option{
			getter.WithPassCredentialsAll(i.PassCredentialsAll),
			getter.WithTLSClientConfig(i.CertFile, i.KeyFile, i.CaFile),
			getter.WithInsecureSkipVerifyTLS(i.InsecureSkipTLSverify),
		},
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		RegistryClient:   h.config.RegistryClient,
	}

	if i.Verify {
		dl.Verify = downloader.VerifyAlways
	}

	chartURL, err := repo.FindChartInAuthAndTLSAndPassRepoURL(i.RepoURL, i.Username, i.Password, chartName, i.Version,
		i.CertFile, i.KeyFile, i.CaFile, i.InsecureSkipTLSverify, i.PassCredentialsAll, getter.All(settings))
	if err != nil {
		return "", err
	}

	// Only pass the user/pass on when the user has said to or when the
	// location of the chart repo and the chart are the same domain.
	u1, err := url.Parse(i.RepoURL)
	if err != nil {
		return "", err
	}
	u2, err := url.Parse(chartURL)
	if err != nil {
		return "", err
	}

	// Host on URL (returned from url.Parse) contains the port if present.
	// This check ensures credentials are not passed between different
	// services on different ports.
	if i.PassCredentialsAll || (u1.Scheme == u2.Scheme && u1.Host == u2.Host) {
		dl.Options = append(dl.Options, getter.WithBasicAuth(i.Username, i.Password))
	} else {
		dl.Options = append(dl.Options, getter.WithBasicAuth("", ""))
	}

	if err := os.MkdirAll(settings.RepositoryCache, 0755); err != nil {
		return "", err
	}

	filename, _, err := dl.DownloadTo(chartURL, i.Version, settings.RepositoryCache)
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		return lname, nil
	}
	return filename, err
}

// prepare is common steps for install or upgrade
func (h *HelmWrapper) prepare(ctx context.Context, cli client.Client, logger logr.Logger, chartPathOption action.ChartPathOptions, cpl *corev1alpha1.ComponentPlan, chartName string) (chartRequested *chart.Chart, vals map[string]interface{}, err error) {
	// Note: we should not use helm i.ChartPathOptions.LocateChart(chartName, settings) because this function writes log to stdout...
	cp, err := h.chartDownload(chartName, logger, chartPathOption)
	if err != nil {
		return nil, nil, err
	}
	logger.V(1).Info(fmt.Sprintf("CHART PATH: %s", cp))

	p := getter.All(settings)
	valueOpts := &values.Options{}
	for _, valuesFrom := range cpl.Spec.Override.ValuesFrom {
		fileName, err := utils.ParseValuesReference(ctx, cli, cpl.Namespace, valuesFrom.GetValuesFileDir(helmpath.CachePath(""), cpl.Namespace), valuesFrom)
		if err != nil {
			return nil, nil, err
		}
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, fileName)
		logger.V(1).Info(fmt.Sprintf("Add Override.ValuesFrom From: %s", fileName))
	}
	if o := cpl.Spec.Override; o.Values != nil {
		fileName, err := utils.ParseValues(o.GetValueFileDir(helmpath.CachePath(""), cpl.Namespace, cpl.Name), o.Values)
		if err != nil {
			return nil, nil, err
		}
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, fileName)
		logger.V(1).Info(fmt.Sprintf("Add Override.Values From: %s", fileName))
	}
	if set := cpl.Spec.Override.Set; len(set) != 0 {
		valueOpts.Values = cpl.Spec.Override.Set
	}
	if set := cpl.Spec.Override.SetString; len(set) != 0 {
		valueOpts.StringValues = cpl.Spec.Override.SetString
	}
	vals, err = valueOpts.MergeValues(p)
	if err != nil {
		return nil, nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err = loader.Load(cp)
	if err != nil {
		return nil, nil, err
	}

	if chartRequested.Metadata.Type != "" && chartRequested.Metadata.Type != "application" {
		return nil, nil, errors.Errorf("%s charts are not installable", chartRequested.Metadata.Type)
	}

	if chartRequested.Metadata.Deprecated {
		logger.V(1).Info("This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			buf := new(strings.Builder)
			man := &downloader.Manager{
				Out:              buf,
				ChartPath:        cp,
				Keyring:          chartPathOption.Keyring,
				SkipUpdate:       false,
				Getters:          p,
				RepositoryConfig: settings.RepositoryConfig,
				RepositoryCache:  settings.RepositoryCache,
				Debug:            settings.Debug,
			}
			printLog := func() {
				for _, l := range strings.Split(buf.String(), "\n") {
					logger.V(1).Info(l)
				}
			}
			if err := man.Update(); err != nil {
				printLog()
				return nil, nil, err
			}
			printLog()
			// Reload the chart with the updated Chart.lock file.
			if chartRequested, err = loader.Load(cp); err != nil {
				return nil, nil, errors.Wrap(err, "failed reloading chart after repo update")
			}
		}
	}
	return chartRequested, vals, nil
}
