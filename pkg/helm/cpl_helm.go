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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/release"
	hrepo "helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/utils"
)

type CoreHelm interface {
	InstallOrUpgrade(ctx context.Context, chartName string) (rel *release.Release, err error)
	Uninstall(ctx context.Context) error
	GetLastRelease() (rel *release.Release, err error)
	GetManifestsByDryRun(ctx context.Context, chartName string) (data string, err error)
	Rollback(ctx context.Context) error
	GetOCIRepoCharts(ctx context.Context, pullURL string, skipTags map[string]bool) (latest *chart.Metadata, all []*hrepo.ChartVersion, err error)
	PullAndParse(ctx context.Context, pullURL, version string) (out string, chartRequested *chart.Chart, err error)
	Pull(ctx context.Context, pullURL, version string) (out, dir, entryName string, err error)
	Template(ctx context.Context, version string, localPath string) (rel *release.Release, err error)
}

var _ CoreHelm = &CoreHelmWrapper{}

// CoreHelmWrapper is a wrapper for helm command
type CoreHelmWrapper struct {
	*HelmWrapper
	cpl       *corev1alpha1.ComponentPlan
	repo      *corev1alpha1.Repository
	component *corev1alpha1.Component
	cli       client.Client
	logger    logr.Logger
}

// NewCoreHelmWrapper returns a new helmWrapper instance
func NewCoreHelmWrapper(getter genericclioptions.RESTClientGetter, namespace string, logger logr.Logger, cli client.Client, cpl *corev1alpha1.ComponentPlan, repo *corev1alpha1.Repository, component *corev1alpha1.Component) (*CoreHelmWrapper, error) {
	h, err := NewHelmWrapper(getter, namespace, logger)
	if err != nil {
		return nil, err
	}
	return &CoreHelmWrapper{
		HelmWrapper: h,
		cpl:         cpl,
		repo:        repo,
		component:   component,
		cli:         cli,
		logger:      logger,
	}, nil
}

// InstallOrUpgrade installs / ungrade a helm chart to the cluster
func (c *CoreHelmWrapper) InstallOrUpgrade(ctx context.Context, chartName string) (rel *release.Release, err error) {
	return c.installOrUpgrade(ctx, chartName, false)
}

// Uninstall installs a helm chart to the cluster
func (c *CoreHelmWrapper) Uninstall(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			if strings.HasPrefix(err.Error(), "uninstallation completed with 1 error(s): uninstall: Failed to purge the release: release: not found") { //nolint:dupword
				err = nil
			}
		}
	}()
	rel, err := c.GetLastRelease()
	if err != nil {
		return err
	}
	if rel == nil {
		return nil
	}
	var parseDescription bool
	var relNs, relName, relUID string
	if rel.Info != nil && rel.Info.Status == release.StatusDeployed && !strings.HasPrefix(rel.Info.Description, "Rollback ") {
		// descrpiton match, the chart is exactly installed by this componentplan
		relNs, relName, relUID, _, _ = ParseDescription(rel.Info.Description)
		parseDescription = true
	}
	if parseDescription {
		if c.cpl.Namespace == relNs || c.cpl.Name == relName || string(c.cpl.GetUID()) == relUID {
			return c.uninstall(ctx)
		}
	} else {
		// version match, the chart is exactly installed by this componentplan
		if rel.Version == c.cpl.Status.InstalledRevision {
			return c.uninstall(ctx)
		}
		// when installing/upgrading/rollingback, description will be setted by helm, so we compare helm release version and component status
		if c.cpl.Status.InstalledRevision+1 == rel.Version {
			if c.cpl.IsActionedReason(corev1alpha1.ComponentPlanReasonInstalling) || c.cpl.IsActionedReason(corev1alpha1.ComponentPlanReasonUpgrading) || c.cpl.IsActionedReason(corev1alpha1.ComponentPlanReasonRollingBack) {
				return c.uninstall(ctx)
			}
		}
	}
	return nil
}

// GetLastRelease get last release revision
func (c *CoreHelmWrapper) GetLastRelease() (rel *release.Release, err error) {
	return c.HelmWrapper.GetLastRelease(c.cpl.GetReleaseName())
}

// GetManifestsByDryRun get helm templates by dryRun
func (c *CoreHelmWrapper) GetManifestsByDryRun(ctx context.Context, chartName string) (data string, err error) {
	rel, err := c.installOrUpgrade(ctx, chartName, true)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

// installOrUpgrade installs / ungrade a helm chart to the cluster
func (c *CoreHelmWrapper) installOrUpgrade(ctx context.Context, chartName string, dryRun bool) (rel *release.Release, err error) {
	rel, err = c.HelmWrapper.GetLastRelease(c.cpl.GetReleaseName())
	if err != nil {
		return nil, err
	}
	if rel == nil {
		rel, err = c.install(ctx, dryRun, chartName)
		if err != nil {
			return nil, err
		}
		c.logger.Info(fmt.Sprintf("helm install completed dryRun:%t", dryRun), ReleaseLog(rel)...)
	} else {
		c.logger.Info("helm find last release", ReleaseLog(rel)...)
		rel, err = c.upgrade(ctx, dryRun, chartName)
		if err != nil {
			return nil, err
		}
		c.logger.Info(fmt.Sprintf("upgrade completed dryRun:%t", dryRun), ReleaseLog(rel)...)
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

func (c *CoreHelmWrapper) Rollback(ctx context.Context) (err error) {
	return c.rollback(ctx)
}

// GetOCIRepoCharts retrieves the latest chart metadata and all component versions for a given OCI repository.
func (c *CoreHelmWrapper) GetOCIRepoCharts(ctx context.Context, pullURL string, skipTags map[string]bool) (latest *chart.Metadata, all []*hrepo.ChartVersion, err error) {
	tags, err := c.config.RegistryClient.Tags(strings.TrimPrefix(pullURL, "oci://"))
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
		out, c, err := c.PullAndParse(ctx, pullURL, tag)
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

func (c *CoreHelmWrapper) PullAndParse(ctx context.Context, pullURL, version string) (out string, chartRequested *chart.Chart, err error) {
	out, dir, entryName, err := c.Pull(ctx, pullURL, version)
	if err != nil {
		c.logger.Error(err, "cannot download chart")
		return out, nil, err
	}
	defer os.RemoveAll(dir)
	chartRequested, err = loader.Load(dir + "/" + entryName)
	if err != nil {
		c.logger.Error(err, "Cannot load chart")
		return out, nil, err
	}
	return out, chartRequested, nil
}

func (c *CoreHelmWrapper) Pull(ctx context.Context, pullURL, version string) (out, dir, entryName string, err error) {
	entryName = utils.GetOCIEntryName(pullURL)
	if !c.repo.IsOCI() {
		entryName = utils.GetHTTPEntryName(pullURL)
	}
	i := c.HelmWrapper.GetDefaultPullCfg()
	if c.repo.Spec.AuthSecret != "" {
		i.Username, i.Password, i.CaFile, i.CertFile, i.KeyFile, err = corev1alpha1.ParseRepoSecret(c.cli, c.repo)
		if err != nil {
			return "", "", "", err
		}
	}
	i.Version = version
	i.Settings = settings
	i.Devel = false
	i.VerifyLater = false
	i.Untar = true
	i.UntarDir, err = os.MkdirTemp("", "kubebb-*")
	if err != nil {
		c.logger.Error(err, "Failed to create a new dir")
		_ = os.RemoveAll(i.UntarDir)
		return "", "", "", err
	}
	out, err = c.HelmWrapper.Pull(ctx, c.logger, i, pullURL)
	if err != nil {
		c.logger.Error(err, "cannot download chart")
		_ = os.RemoveAll(i.UntarDir)
		return "", "", "", err
	}
	c.logger.Info(out)
	return out, i.UntarDir, entryName, nil
}

func (c *CoreHelmWrapper) Template(ctx context.Context, version, localPath string) (rel *release.Release, err error) {
	i, _, _, _, _, _, _ := c.GetDefaultTemplateCfg()
	i.DryRun = true
	i.ReleaseName = "release-name"
	i.Replace = true          // skip the name check
	i.ClientOnly = true       // do not validate
	i.IncludeCRDs = false     // we just want images info, no crd
	i.CreateNamespace = false // installed in the same namespace with ComponentPlan
	i.DisableHooks = true     // we just want images info, no crd
	i.Replace = false         // reuse the given name is not safe in production
	i.Devel = false           // just set `>0.0.0-0` to ComponentPlan.Spec.Version is enough
	i.DependencyUpdate = true
	i.DisableOpenAPIValidation = true // we just want images info, no crd
	i.SkipCRDs = true                 // we just want images info, no crd
	i.SubNotes = false                // we cant see notes or subnotes
	i.Version = version
	i.Verify = false // TODO enable these args after we can config keyring
	i.RepoURL = ""   // We only support adding a repo and then installing it, not installing it directly via an url.
	i.Keyring = ""   // TODO enable these args after we can config keyring
	if c.repo.Spec.AuthSecret != "" {
		i.Username, i.Password, i.CaFile, i.CertFile, i.KeyFile, err = corev1alpha1.ParseRepoSecret(c.cli, c.repo)
		if err != nil {
			return nil, err
		}
	}
	i.InsecureSkipTLSverify = c.repo.Spec.Insecure
	i.PassCredentialsAll = false // TODO do we need add this args to override?

	validate := false
	skipTests := false
	includeCrds := false
	kubeVersion := ""
	extraAPIs := []string{}
	showFiles := []string{}
	ValueOpts := c.SetValuesOptions(nil, nil, nil, nil)
	rel, _, err = c.HelmWrapper.Template(ctx, c.logger, i, ValueOpts, i.ReleaseName, localPath, kubeVersion, validate, includeCrds, skipTests, extraAPIs, showFiles)
	if err != nil {
		return nil, err
	}
	return rel, err
}

func (c *CoreHelmWrapper) install(ctx context.Context, dryRun bool, chartName string) (rel *release.Release, err error) {
	log := c.logger.WithValues("ComponentPlan", klog.KObj(c.cpl))
	if c.repo.IsOCI() {
		log.Info("Installing OCI Component")
	} else {
		log.Info("Installing non-OCI Component")
	}
	if dryRun {
		log.WithValues("dryRun", true)
	}

	i := c.HelmWrapper.GetDefaultInstallCfg()
	i.CreateNamespace = false // installed in the same namespace with ComponentPlan
	i.DryRun = dryRun
	i.DisableHooks = c.cpl.Spec.DisableHooks
	i.Replace = false // reuse the given name is not safe in production
	i.Timeout = c.cpl.Spec.Timeout()
	i.Wait = c.cpl.Spec.Wait
	i.WaitForJobs = c.cpl.Spec.WaitForJobs
	i.GenerateName = false // we use ComponentPlan.Spec.Name
	i.NameTemplate = ""    // we use ComponentPlan.Spec.Name
	i.Description = generateDescription(c.cpl)
	i.Devel = false // just set `>0.0.0-0` to ComponentPlan.Spec.Version is enough
	i.DependencyUpdate = true
	i.DisableOpenAPIValidation = c.cpl.Spec.DisableOpenAPIValidation
	i.Atomic = c.cpl.Spec.Atomic
	i.SkipCRDs = c.cpl.Spec.SkipCRDs
	i.SubNotes = false // we cant see notes or subnotes
	i.Version = c.cpl.Spec.InstallVersion
	i.Verify = false // TODO enable these args after we can config keyring
	i.RepoURL = ""   // We only support adding a repo and then installing it, not installing it directly via an url.
	i.Keyring = ""   // TODO enable these args after we can config keyring
	if c.repo.Spec.AuthSecret != "" {
		i.Username, i.Password, i.CaFile, i.CertFile, i.KeyFile, err = corev1alpha1.ParseRepoSecret(c.cli, c.repo)
		if err != nil {
			return nil, err
		}
	}
	i.InsecureSkipTLSverify = c.repo.Spec.Insecure
	i.PassCredentialsAll = false // TODO do we need add this args to override?
	i.PostRenderer = newPostRenderer(c.repo.Spec.ImageOverride, c.cpl.Spec.Override.Images)
	log.V(1).Info(fmt.Sprintf("Original chart version: %q", i.Version))
	i.ReleaseName = c.cpl.GetReleaseName()
	valueOpts, err := c.setVals(ctx)
	if err != nil {
		return nil, err
	}
	i.Namespace = c.cpl.Namespace
	if dryRun {
		i.ClientOnly = true
	}
	var out string
	rel, out, err = c.HelmWrapper.Install(ctx, c.logger, i, valueOpts, c.cpl.GetReleaseName(), chartName)
	c.logger.V(0).Info(out)
	return
}

func (c *CoreHelmWrapper) upgrade(ctx context.Context, dryRun bool, chartName string) (rel *release.Release, err error) {
	log := c.logger.WithValues("ComponentPlan", klog.KObj(c.cpl))
	if c.repo.IsOCI() {
		log.Info("Upgrading OCI Component")
	} else {
		log.Info("Upgrading non-OCI Component")
	}
	if dryRun {
		log.WithValues("dryRun", true)
	}
	i, _ := c.GetDefaultUpgradeCfg()
	i.Install = false // we just want to upgrade, for install, we use Install function
	i.Devel = false   // just set `>0.0.0-0` to ComponentPlan.Spec.Version is enough
	i.DryRun = dryRun
	i.Force = c.cpl.Spec.Force
	i.DisableHooks = c.cpl.Spec.DisableHooks
	i.DisableOpenAPIValidation = c.cpl.Spec.DisableOpenAPIValidation
	i.SkipCRDs = c.cpl.Spec.SkipCRDs
	i.Timeout = c.cpl.Spec.Timeout()
	i.ResetValues = false // dont reset value to default
	i.ReuseValues = false // dont reuse value
	i.Wait = c.cpl.Spec.Wait
	i.WaitForJobs = c.cpl.Spec.WaitForJobs
	i.Atomic = c.cpl.Spec.Atomic
	i.MaxHistory = c.cpl.Spec.GetMaxHistory()
	i.CleanupOnFail = c.cpl.Spec.CleanupOnFail
	i.SubNotes = false // we cant see notes or subnotes
	i.Description = generateDescription(c.cpl)
	i.DependencyUpdate = true
	i.Version = c.cpl.Spec.InstallVersion
	i.Verify = false // TODO enable these args after we can config keyring
	i.RepoURL = ""   // We only support adding a repo and then installing it, not installing it directly via a url.
	i.Keyring = ""   // TODO enable these args after we can config keyring
	if c.repo.Spec.AuthSecret != "" {
		i.Username, i.Password, i.CaFile, i.CertFile, i.KeyFile, err = corev1alpha1.ParseRepoSecret(c.cli, c.repo)
		if err != nil {
			return nil, err
		}
	}
	i.InsecureSkipTLSverify = c.repo.Spec.Insecure
	i.PassCredentialsAll = false // TODO do we need add this args to override?
	i.PostRenderer = newPostRenderer(c.repo.Spec.ImageOverride, c.cpl.Spec.Override.Images)
	valueOpts, err := c.setVals(ctx)
	if err != nil {
		return nil, err
	}
	i.Namespace = c.cpl.Namespace
	var out string
	rel, out, err = c.Upgrade(ctx, c.logger, i, valueOpts, c.cpl.GetReleaseName(), chartName, false)
	c.logger.V(0).Info(out)
	return rel, err
}

func (c *CoreHelmWrapper) uninstall(ctx context.Context) (err error) {
	log := c.logger.WithValues("ComponentPlan", klog.KObj(c.cpl))
	i := c.GetDefaultUninstallCfg()
	i.DryRun = false // do not need to simulate the installation
	i.DisableHooks = c.cpl.Spec.DisableHooks
	i.KeepHistory = c.cpl.Spec.KeepHistory
	i.Timeout = c.cpl.Spec.Timeout()
	i.Wait = c.cpl.Spec.Wait
	i.Description = generateDescription(c.cpl)
	out, err := c.HelmWrapper.Uninstall(ctx, c.logger, i, c.cpl.GetReleaseName())
	if err != nil {
		return err
	}
	log.Info(out)
	return nil
}

func (c *CoreHelmWrapper) rollback(ctx context.Context) (err error) {
	log := c.logger.WithValues("ComponentPlan", klog.KObj(c.cpl))
	i := c.GetDefaultRollbackCfg()
	i.DryRun = false // do not need to simulate the rollback
	i.DisableHooks = c.cpl.Spec.DisableHooks
	i.Timeout = c.cpl.Spec.Timeout()
	i.Wait = c.cpl.Spec.Wait
	i.WaitForJobs = c.cpl.Spec.WaitForJobs
	i.CleanupOnFail = c.cpl.Spec.CleanupOnFail
	i.MaxHistory = c.cpl.Spec.GetMaxHistory()
	i.Recreate = c.cpl.Spec.RecreatePods
	i.Force = c.cpl.Spec.Force
	i.Version = c.cpl.Status.InstalledRevision
	out, err := c.HelmWrapper.Rollback(ctx, c.logger, i, c.cpl.GetReleaseName(), c.cpl.Status.InstalledRevision)
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("release \"%s\" rollback to revision:%d", c.cpl.GetReleaseName(), c.cpl.Status.InstalledRevision))
	log.Info(out)
	return nil
}

func (c *CoreHelmWrapper) setVals(ctx context.Context) (valueOpts *values.Options, err error) {
	valueOpts = &values.Options{}
	for _, valuesFrom := range c.cpl.Spec.Override.ValuesFrom {
		fileName, err := valuesFrom.Parse(ctx, c.cli, c.cpl.Namespace, valuesFrom.GetValuesFileDir(helmpath.CachePath(""), c.cpl.Namespace))
		if err != nil {
			return nil, err
		}
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, fileName)
		c.logger.V(1).Info(fmt.Sprintf("Add Override.ValuesFrom From: %s", fileName))
	}
	if o := c.cpl.Spec.Override; o.Values != nil {
		fileName, err := utils.ParseValues(o.GetValueFileDir(helmpath.CachePath(""), c.cpl.Namespace, c.cpl.Name), o.Values)
		if err != nil {
			return nil, err
		}
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, fileName)
		c.logger.V(1).Info(fmt.Sprintf("Add Override.Values From: %s", fileName))
	}
	if set := c.cpl.Spec.Override.Set; len(set) != 0 {
		valueOpts.Values = c.cpl.Spec.Override.Set
	}
	if set := c.cpl.Spec.Override.SetString; len(set) != 0 {
		valueOpts.StringValues = c.cpl.Spec.Override.SetString
	}
	return valueOpts, nil
}

func generateDescription(plan *corev1alpha1.ComponentPlan) string {
	return fmt.Sprintf("core:%s/%s/%s/%d %s", plan.GetNamespace(), plan.GetName(), plan.GetUID(), plan.GetGeneration(), plan.Spec.Config.Description)
}

func ParseDescription(desc string) (ns, name, uid string, generation int64, raw string) {
	raw = desc
	if !strings.HasPrefix(desc, "core:") {
		return
	}
	other := strings.TrimPrefix(desc, "core:")
	s := strings.Split(other, " ")
	if len(s) < 2 {
		return
	}
	s0 := s[0]
	t := strings.Split(s0, "/")
	if len(t) != 4 {
		return
	}
	ns = t[0]
	name = t[1]
	uid = t[2]
	generation, _ = strconv.ParseInt(t[3], 10, 64)
	return ns, name, uid, generation, strings.Join(s[1:], " ")
}

// ParseDigestFromPullOut extracts the sha256 digest from the `helm pull` output string.
// It takes a string as input parameter which represents the output string.
// It returns a string which is the extracted sha256 digest.
func ParseDigestFromPullOut(out string) string {
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Digest: sha256:") {
			return strings.TrimPrefix(l, "Digest: sha256:")
		}
	}
	return ""
}
