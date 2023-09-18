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
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/homedir"
)

type HelmRelease interface {
	GetDefaultInstallCfg() *action.Install
	SetValuesOptions(f, set, setString, setFile []string) *values.Options
	Install(ctx context.Context, logger logr.Logger, client *action.Install, valueOpts *values.Options, name, chart string) (rel *release.Release, out string, err error)
	InstallWithDefaulConfig(ctx context.Context, logger logr.Logger, name, chart string) (rel *release.Release, out string, err error)
	GetDefaultUpgradeCfg() (upgradeCfg *action.Upgrade, createNamespace bool)
	Upgrade(ctx context.Context, logger logr.Logger, client *action.Upgrade, valueOpts *values.Options, releaseName, chart string, createNamespace bool) (rel *release.Release, out string, err error)
	UpgradeWithDefaulConfig(ctx context.Context, logger logr.Logger, releaseName, chart string) (rel *release.Release, out string, err error)
	GetDefaultTemplateCfg() (templateCfg *action.Install, validate, skipTests, includeCrds bool, kubeVersion string, extraAPIs, showFiles []string)
	Template(ctx context.Context, logger logr.Logger, client *action.Install, valueOpts *values.Options, name, chart, kubeVersion string, validate, includeCrds, skipTests bool, extraAPIs, showFiles []string) (rel *release.Release, templateOut string, err error)
	TemplateWithDefaulConfig(ctx context.Context, logger logr.Logger, name, chart string) (rel *release.Release, out string, err error)
	GetDefaultRollbackCfg() *action.Rollback
	Rollback(ctx context.Context, logger logr.Logger, client *action.Rollback, releaseName string, revision int) (out string, err error)
	RollbackWithDefaultConfig(ctx context.Context, logger logr.Logger, releaseName string, revision int) (out string, err error)
	GetDefaultUninstallCfg() *action.Uninstall
	Uninstall(ctx context.Context, logger logr.Logger, client *action.Uninstall, releaseNames ...string) (out string, err error)
	UninstallWithDefaultConfig(ctx context.Context, logger logr.Logger, releaseNames ...string) (out string, err error)
	GetDefaultPullCfg() *action.Pull
	Pull(ctx context.Context, logger logr.Logger, client *action.Pull, args ...string) (out string, err error)
	PullWithDefaultConfig(ctx context.Context, logger logr.Logger, args ...string) (out string, err error)
	GetLastRelease(releaseName string) (*release.Release, error)
}

var _ HelmRelease = &HelmWrapper{}

// This make user reuse all helm environment variables, like HELM_PLUGINS etc.
var settings = cli.New()

// HelmWrapper is a wrapper for helm command
type HelmWrapper struct {
	config *action.Configuration
	buf    *bytes.Buffer
}

// NewHelmWrapper returns a new helmWrapper instance
func NewHelmWrapper(getter genericclioptions.RESTClientGetter, namespace string, logger logr.Logger) (*HelmWrapper, error) {
	cfg := new(action.Configuration)
	if err := cfg.Init(getter, namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		logger.V(1).Info(fmt.Sprintf(format, v...))
	}); err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
		registry.ClientOptWriter(buf),
	)
	if err != nil {
		return nil, err
	}
	cfg.RegistryClient = registryClient

	return &HelmWrapper{
		config: cfg,
		buf:    buf,
	}, nil
}

func (h *HelmWrapper) GetDefaultInstallCfg() *action.Install {
	client := action.NewInstall(h.config)
	client.CreateNamespace = false          // helm install --create-namespace
	client.DryRun = false                   // helm install --dry-run
	client.DisableHooks = false             // helm install --no-hooks
	client.Replace = false                  // helm install --replace
	client.Timeout = 300 * time.Second      // helm install --timeout
	client.Wait = false                     // helm install --wait
	client.WaitForJobs = false              // helm install --wait-for-jobs
	client.GenerateName = false             // helm install --generate-name
	client.NameTemplate = ""                // helm install --name-template
	client.Description = ""                 // helm install --description
	client.Devel = false                    // helm install --devel
	client.DependencyUpdate = false         // helm install --dependency-update
	client.DisableOpenAPIValidation = false // helm install --disable-openapi-validation
	client.Atomic = false                   // helm install --atomic
	client.SkipCRDs = false                 // helm install --skip-crds
	client.SubNotes = false                 // helm install --render-subchart-notes
	h.setDefaultChartPathCfg(&client.ChartPathOptions)
	return client
}

func (h *HelmWrapper) setDefaultChartPathCfg(client *action.ChartPathOptions) {
	client.Version = ""                  // helm install --version
	client.Verify = false                // helm install --verify
	client.Keyring = defaultKeyring()    // helm install --keyring
	client.RepoURL = ""                  // helm install --repo
	client.Username = ""                 // helm install --username
	client.Password = ""                 // helm install --password
	client.CertFile = ""                 // helm install --cert-file
	client.KeyFile = ""                  // helm install --key-file
	client.InsecureSkipTLSverify = false // helm install --insecure-skip-tls-verify
	client.CaFile = ""                   // helm install --ca-file
	client.PassCredentialsAll = false    // helm install --pass-credentials
}

// defaultKeyring returns the expanded path to the default keyring.
// copy from github.com/helm/helm/cmd/helm/dependency_build.go
func defaultKeyring() string {
	if v, ok := os.LookupEnv("GNUPGHOME"); ok {
		return filepath.Join(v, "pubring.gpg")
	}
	return filepath.Join(homedir.HomeDir(), ".gnupg", "pubring.gpg")
}

// SetValuesOptions
// -f --values specify values in a YAML file or a URL (can specify multiple)
// --set set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
// --set-string set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
// --set-file set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
func (h *HelmWrapper) SetValuesOptions(f, set, setString, setFile []string) *values.Options {
	valueOpts := &values.Options{}
	valueOpts.ValueFiles = f
	valueOpts.Values = set
	valueOpts.StringValues = setString
	valueOpts.FileValues = setFile
	return valueOpts
}

// Install [NAME] [CHART]
func (h *HelmWrapper) Install(ctx context.Context, logger logr.Logger, client *action.Install, valueOpts *values.Options, name, chart string) (rel *release.Release, out string, err error) {
	logger.V(0).Info(fmt.Sprintf("Original chart version: %q", client.Version))
	if client.Version == "" && client.Devel {
		logger.V(0).Info("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}
	name, chart, err = client.NameAndChart([]string{name, chart})
	if err != nil {
		return nil, "", err
	}
	client.ReleaseName = name

	cp, err := client.ChartPathOptions.LocateChart(chart, settings)
	if err != nil {
		return nil, "", err
	}

	logger.V(0).Info(fmt.Sprintf("CHART PATH: %s\n", cp))

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, "", err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, "", err
	}
	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, "", err
	}

	if chartRequested.Metadata.Deprecated {
		logger.Info("This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			err = errors.Wrap(err, "An error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies")
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              h.buf,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				if err := man.Update(); err != nil {
					return nil, "", err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, "", errors.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return nil, "", err
			}
		}
	}

	// client.Namespace = settings.Namespace()

	// Create context and prepare the handle of SIGTERM
	// ctx := context.Background()
	// ctx, cancel := context.WithCancel(ctx)
	//
	//// Set up channel on which to send signal notifications.
	//// We must use a buffered channel or risk missing the signal
	//// if we're not ready to receive when the signal is sent.
	// cSignal := make(chan os.Signal, 2)
	// signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	// go func() {
	//	<-cSignal
	//	fmt.Fprintf(out, "Release %s has been cancelled.\n", args[0])
	//	cancel()
	// }()

	rel, err = client.RunWithContext(ctx, chartRequested, vals)
	out = h.buf.String()
	h.buf.Reset()
	return rel, out, err
}

// InstallWithDefaulConfig [NAME] [CHART]
func (h *HelmWrapper) InstallWithDefaulConfig(ctx context.Context, logger logr.Logger, name, chart string) (rel *release.Release, out string, err error) {
	client := h.GetDefaultInstallCfg()
	valueOpts := h.SetValuesOptions(nil, nil, nil, nil)
	return h.Install(ctx, logger, client, valueOpts, name, chart)
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
// copy from https://github.com/helm/helm/blob/9f869c6b214e75b48024fc9e1b2fb1c51e76b63d/cmd/helm/install.go#L270
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

func (h *HelmWrapper) GetDefaultUpgradeCfg() (upgradeCfg *action.Upgrade, createNamespace bool) {
	client := action.NewUpgrade(h.config)
	client.Install = false                  // helm upgrade --install
	client.Devel = false                    // helm upgrade --devel
	client.DryRun = false                   // helm upgrade --dry-run
	client.Recreate = false                 // helm upgrade --recreate-pods but deprecated
	client.Force = false                    // helm upgrade --force
	client.DisableHooks = false             // helm upgrade --no-hooks
	client.DisableOpenAPIValidation = false // helm upgrade --disable-openapi-validation
	client.SkipCRDs = false                 // helm upgrade --skip-crds
	client.Timeout = 300 * time.Second      // helm upgrade --timeout
	client.ResetValues = false              // helm upgrade --reset-values
	client.ReuseValues = false              // helm upgrade --reuse-values
	client.Wait = false                     // helm upgrade --wait
	client.WaitForJobs = false              // helm upgrade --wait-for-jobs
	client.Atomic = false                   // helm upgrade --atomic
	client.MaxHistory = settings.MaxHistory // helm upgrade --history-max
	client.CleanupOnFail = false            // helm upgrade --cleanup-on-fail
	client.SubNotes = false                 // helm upgrade --render-subchart-notes
	client.Description = ""                 // helm upgrade --description
	client.DependencyUpdate = false         // helm upgrade --dependency-update
	h.setDefaultChartPathCfg(&client.ChartPathOptions)
	createNamespace = false // helm upgrade --create-namespace
	return client, createNamespace
}

// Upgrade [RELEASE] [CHART]
// createNamespace: helm upgrade --create-namespace
func (h *HelmWrapper) Upgrade(ctx context.Context, logger logr.Logger, client *action.Upgrade, valueOpts *values.Options, releaseName, chart string, createNamespace bool) (rel *release.Release, out string, err error) {
	client.Namespace = settings.Namespace()
	// Fixes #7002 - Support reading values from STDIN for `upgrade` command
	// Must load values AFTER determining if we have to call install so that values loaded from stdin are not read twice
	if client.Install {
		// If a release does not exist, install it.
		histClient := action.NewHistory(h.config)
		histClient.Max = 1
		if _, err := histClient.Run(releaseName); errors.Is(err, driver.ErrReleaseNotFound) {
			// Only print this to stdout for table output
			// if outfmt == output.Table {
			//	fmt.Fprintf(out, "Release %q does not exist. Installing it now.\n", args[0])
			//}
			logger.Info(fmt.Sprintf("Release %s does not exist. Installing it now.", releaseName))

			instClient := action.NewInstall(h.config)
			instClient.CreateNamespace = createNamespace
			instClient.ChartPathOptions = client.ChartPathOptions
			instClient.DryRun = client.DryRun
			instClient.DisableHooks = client.DisableHooks
			instClient.SkipCRDs = client.SkipCRDs
			instClient.Timeout = client.Timeout
			instClient.Wait = client.Wait
			instClient.WaitForJobs = client.WaitForJobs
			instClient.Devel = client.Devel
			instClient.Namespace = client.Namespace
			instClient.Atomic = client.Atomic
			instClient.PostRenderer = client.PostRenderer
			instClient.DisableOpenAPIValidation = client.DisableOpenAPIValidation
			instClient.SubNotes = client.SubNotes
			instClient.Description = client.Description
			instClient.DependencyUpdate = client.DependencyUpdate

			return h.Install(ctx, logger, instClient, valueOpts, releaseName, chart)
		} else if err != nil {
			return nil, "", err
		}
	}

	if client.Version == "" && client.Devel {
		logger.V(0).Info("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	chartPath, err := client.ChartPathOptions.LocateChart(chart, settings)
	if err != nil {
		return nil, "", err
	}

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, "", err
	}

	// Check chart dependencies to make sure all are present in /charts
	ch, err := loader.Load(chartPath)
	if err != nil {
		return nil, "", err
	}
	if req := ch.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(ch, req); err != nil {
			err = errors.Wrap(err, "An error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies")
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              h.buf,
					ChartPath:        chartPath,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				if err := man.Update(); err != nil {
					return nil, "", err
				}
				// Reload the chart with the updated Chart.lock file.
				if ch, err = loader.Load(chartPath); err != nil {
					return nil, "", errors.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return nil, "", err
			}
		}
	}

	if ch.Metadata.Deprecated {
		logger.Info("This chart is deprecated")
	}

	// Create context and prepare the handle of SIGTERM
	// ctx := context.Background()
	// ctx, cancel := context.WithCancel(ctx)

	//// Set up channel on which to send signal notifications.
	//// We must use a buffered channel or risk missing the signal
	//// if we're not ready to receive when the signal is sent.
	// cSignal := make(chan os.Signal, 2)
	// signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	// go func() {
	//	<-cSignal
	//	fmt.Fprintf(out, "Release %s has been cancelled.\n", args[0])
	//	cancel()
	// }()

	rel, err = client.RunWithContext(ctx, releaseName, ch, vals)
	out = h.buf.String()
	h.buf.Reset()
	if err != nil {
		return rel, out, errors.Wrap(err, "UPGRADE FAILED")
	}
	return rel, out, nil
}

// UpgradeWithDefaulConfig [RELEASE] [CHART]
func (h *HelmWrapper) UpgradeWithDefaulConfig(ctx context.Context, logger logr.Logger, releaseName, chart string) (rel *release.Release, out string, err error) {
	client, createNamespace := h.GetDefaultUpgradeCfg()
	valueOpts := h.SetValuesOptions(nil, nil, nil, nil)
	return h.Upgrade(ctx, logger, client, valueOpts, releaseName, chart, createNamespace)
}

func (h *HelmWrapper) GetDefaultTemplateCfg() (templateCfg *action.Install, validate, skipTests, includeCrds bool, kubeVersion string, extraAPIs, showFiles []string) {
	client := h.GetDefaultInstallCfg()
	validate = false              // helm template --validate
	includeCrds = false           // helm template --include-crds
	skipTests = false             // helm template --skip-tests
	client.IsUpgrade = false      // helm template --is-upgrade
	kubeVersion = ""              // helm template --kube-version
	extraAPIs = []string{}        // helm template --api-versions
	showFiles = []string{}        // helm template --show-only
	client.UseReleaseName = false // helm template --release-name
	return client, validate, skipTests, includeCrds, kubeVersion, extraAPIs, showFiles
}

// Template [NAME] [CHART]
// kubeVersion: helm template --kube-version
// validate: helm template --validate
// extraAPIs: helm template --api-versions
func (h *HelmWrapper) Template(ctx context.Context, logger logr.Logger, client *action.Install, valueOpts *values.Options, name, chart, kubeVersion string, validate, includeCrds, skipTests bool, extraAPIs, showFiles []string) (rel *release.Release, templateOut string, err error) {
	if kubeVersion != "" {
		parsedKubeVersion, err := chartutil.ParseKubeVersion(kubeVersion)
		if err != nil {
			return nil, "", fmt.Errorf("invalid kube version '%s': %s", kubeVersion, err)
		}
		client.KubeVersion = parsedKubeVersion
	}

	client.DryRun = true
	client.ReleaseName = "release-name"
	client.Replace = true // Skip the name check
	client.ClientOnly = !validate
	client.APIVersions = chartutil.VersionSet(extraAPIs)
	client.IncludeCRDs = includeCrds

	rel, installOut, err := h.Install(ctx, logger, client, valueOpts, name, chart)
	if err != nil && !settings.Debug {
		if rel != nil {
			return rel, installOut, fmt.Errorf("%w\n\nUse --debug flag to render out invalid YAML", err)
		}
		return nil, installOut, err
	}

	out := new(bytes.Buffer)
	// We ignore a potential error here because, when the --debug flag was specified,
	// we always want to print the YAML, even if it is not valid. The error is still returned afterwards.
	if rel != nil {
		var manifests bytes.Buffer
		fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
		if !client.DisableHooks {
			fileWritten := make(map[string]bool)
			for _, m := range rel.Hooks {
				if skipTests && isTestHook(m) {
					continue
				}
				if client.OutputDir == "" {
					fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
				} else {
					newDir := client.OutputDir
					if client.UseReleaseName {
						newDir = filepath.Join(client.OutputDir, client.ReleaseName)
					}
					err = writeToFile(newDir, m.Path, m.Manifest, fileWritten[m.Path])
					if err != nil {
						return nil, "", err
					}
					fileWritten[m.Path] = true
				}
			}
		}

		// if we have a list of files to render, then check that each of the
		// provided files exists in the chart.
		if len(showFiles) > 0 {
			// This is necessary to ensure consistent manifest ordering when using --show-only
			// with globs or directory names.
			splitManifests := releaseutil.SplitManifests(manifests.String())
			manifestsKeys := make([]string, 0, len(splitManifests))
			for k := range splitManifests {
				manifestsKeys = append(manifestsKeys, k)
			}
			sort.Sort(releaseutil.BySplitManifestsOrder(manifestsKeys))

			manifestNameRegex := regexp.MustCompile("# Source: [^/]+/(.+)")
			var manifestsToRender []string
			for _, f := range showFiles {
				missing := true
				// Use linux-style filepath separators to unify user's input path
				f = filepath.ToSlash(f)
				for _, manifestKey := range manifestsKeys {
					manifest := splitManifests[manifestKey]
					submatch := manifestNameRegex.FindStringSubmatch(manifest)
					if len(submatch) == 0 {
						continue
					}
					manifestName := submatch[1]
					// manifest.Name is rendered using linux-style filepath separators on Windows as
					// well as macOS/linux.
					manifestPathSplit := strings.Split(manifestName, "/")
					// manifest.Path is connected using linux-style filepath separators on Windows as
					// well as macOS/linux
					manifestPath := strings.Join(manifestPathSplit, "/")

					// if the filepath provided matches a manifest path in the
					// chart, render that manifest
					if matched, _ := filepath.Match(f, manifestPath); !matched {
						continue
					}
					manifestsToRender = append(manifestsToRender, manifest)
					missing = false
				}
				if missing {
					return nil, "", fmt.Errorf("could not find template %s in chart", f)
				}
			}
			for _, m := range manifestsToRender {
				fmt.Fprintf(out, "---\n%s\n", m)
			}
		} else {
			fmt.Fprintf(out, "%s", manifests.String())
		}
	}
	writeOut := out.String()
	return rel, installOut + writeOut, nil
}

// TemplateWithDefaulConfig [NAME] [CHART]
func (h *HelmWrapper) TemplateWithDefaulConfig(ctx context.Context, logger logr.Logger, name, chart string) (rel *release.Release, out string, err error) {
	client, validate, skipTests, includeCrds, kubeVersion, extraAPIs, showFiles := h.GetDefaultTemplateCfg()
	valueOpts := h.SetValuesOptions(nil, nil, nil, nil)
	return h.Template(ctx, logger, client, valueOpts, name, chart, kubeVersion, validate, includeCrds, skipTests, extraAPIs, showFiles)
}

// copy from https://github.com/helm/helm/blob/160da867d05ed3a0585e8bc4bf55f9fdeefe0d9f/cmd/helm/template.go#L191
func isTestHook(h *release.Hook) bool {
	for _, e := range h.Events {
		if e == release.HookTest {
			return true
		}
	}
	return false
}

// The following functions (writeToFile, createOrOpenFile, and ensureDirectoryForFile)
// are copied from the actions package. This is part of a change to correct a
// bug introduced by #8156. As part of the todo to refactor renderResources
// this duplicate code should be removed. It is added here so that the API
// surface area is as minimally impacted as possible in fixing the issue.

func writeToFile(outputDir string, name string, data string, append bool) error {
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

	err := ensureDirectoryForFile(outfileName)
	if err != nil {
		return err
	}

	f, err := createOrOpenFile(outfileName, append)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("---\n# Source: %s\n%s\n", name, data))

	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outfileName)
	return nil
}

// copy from https://github.com/helm/helm/blob/160da867d05ed3a0585e8bc4bf55f9fdeefe0d9f/cmd/helm/template.go#L230
func createOrOpenFile(filename string, append bool) (*os.File, error) {
	if append {
		return os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	}
	return os.Create(filename)
}

// copy from https://github.com/helm/helm/blob/160da867d05ed3a0585e8bc4bf55f9fdeefe0d9f/cmd/helm/template.go#L237
func ensureDirectoryForFile(file string) error {
	baseDir := path.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.MkdirAll(baseDir, 0755)
}

func (h *HelmWrapper) GetDefaultRollbackCfg() *action.Rollback {
	client := action.NewRollback(h.config)
	client.DryRun = false                   // helm rollback --dry-run
	client.Recreate = false                 // helm rollback --recreate-pods
	client.Force = false                    // helm rollback --force
	client.DisableHooks = false             // helm rollback --no-hooks
	client.Timeout = 300 * time.Second      // helm rollback --timeout
	client.Wait = false                     // helm rollback --wait
	client.WaitForJobs = false              // helm rollback --wait-for-jobs
	client.CleanupOnFail = false            // helm rollback --cleanup-on-fail
	client.MaxHistory = settings.MaxHistory // helm rollback --history-max
	return client
}

// Rollback <RELEASE> [REVISION]
func (h *HelmWrapper) Rollback(ctx context.Context, logger logr.Logger, client *action.Rollback, releaseName string, revision int) (out string, err error) {
	if revision != 0 {
		client.Version = revision
	}
	err = client.Run(releaseName)
	out = h.buf.String()
	h.buf.Reset()
	return out, err
}

// RollbackWithDefaultConfig <RELEASE> [REVISION]
func (h *HelmWrapper) RollbackWithDefaultConfig(ctx context.Context, logger logr.Logger, releaseName string, revision int) (out string, err error) {
	client := h.GetDefaultRollbackCfg()
	return h.Rollback(ctx, logger, client, releaseName, revision)
}
func (h *HelmWrapper) GetDefaultUninstallCfg() *action.Uninstall {
	client := action.NewUninstall(h.config)
	client.DryRun = false              // helm uninstall --dry-run
	client.DisableHooks = false        // helm uninstall --no-hooks
	client.KeepHistory = false         // helm uninstall --keep-history
	client.Wait = false                // helm uninstall --wait
	client.Timeout = 300 * time.Second // helm uninstall --timeout
	client.Description = ""            // helm uninstall --description
	return client
}

// Uninstall RELEASE_NAME [...]
func (h *HelmWrapper) Uninstall(ctx context.Context, logger logr.Logger, client *action.Uninstall, releaseNames ...string) (out string, err error) {
	for i := 0; i < len(releaseNames); i++ {
		res, err := client.Run(releaseNames[i])
		if err != nil {
			return "", err
		}
		if res != nil && res.Info != "" {
			fmt.Fprintf(h.buf, res.Info)
		}
		fmt.Fprintf(h.buf, "release \"%s\" uninstalled\n", releaseNames[i])
	}
	out = h.buf.String()
	h.buf.Reset()
	return out, err
}

// UninstallWithDefaultConfig RELEASE_NAME [...]
func (h *HelmWrapper) UninstallWithDefaultConfig(ctx context.Context, logger logr.Logger, releaseNames ...string) (out string, err error) {
	client := h.GetDefaultUninstallCfg()
	return h.Uninstall(ctx, logger, client, releaseNames...)
}

func (h *HelmWrapper) GetDefaultPullCfg() *action.Pull {
	client := action.NewPullWithOpts(action.WithConfig(h.config))
	client.Devel = false       // helm pull --devel
	client.Untar = false       // helm pull --untar
	client.VerifyLater = false // helm pull --prov
	client.UntarDir = "."      // helm pull --untardir
	client.DestDir = "."       // helm pull --destination
	h.setDefaultChartPathCfg(&client.ChartPathOptions)
	return client
}

// Pull [chart URL | repo/chartname] [...]
func (h *HelmWrapper) Pull(ctx context.Context, logger logr.Logger, client *action.Pull, args ...string) (out string, err error) {
	client.Settings = settings
	if client.Version == "" && client.Devel {
		logger.V(0).Info("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	for i := 0; i < len(args); i++ {
		output, err := client.Run(args[i])
		if err != nil {
			return "", err
		}
		fmt.Fprint(h.buf, output)
	}
	out = h.buf.String()
	h.buf.Reset()
	return out, err
}

// PullWithDefaultConfig [chart URL | repo/chartname] [...]
func (h *HelmWrapper) PullWithDefaultConfig(ctx context.Context, logger logr.Logger, args ...string) (out string, err error) {
	client := h.GetDefaultPullCfg()
	return h.Pull(ctx, logger, client, args...)
}

// GetLastRelease observes the last revision
func (h *HelmWrapper) GetLastRelease(releaseName string) (*release.Release, error) {
	rel, err := h.config.Releases.Last(releaseName)
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		err = nil
	}
	return rel, err
}
