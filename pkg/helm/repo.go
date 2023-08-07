/*
 * Copyright 2023 The Kubebb Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/plugin"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"

	"sigs.k8s.io/yaml"
)

// collectPlugins scans for getter plugins.
// This will load plugins according to the cli.
func collectPlugins(settings *cli.EnvSettings) (getter.Providers, error) {
	plugins, err := plugin.FindPlugins(settings.PluginsDirectory)
	if err != nil {
		return nil, err
	}
	var result getter.Providers
	for _, plugin := range plugins {
		for _, downloader := range plugin.Metadata.Downloaders {
			result = append(result, getter.Provider{
				Schemes: downloader.Protocols,
				New: getter.NewPluginGetter(
					downloader.Command,
					settings,
					plugin.Metadata.Name,
					plugin.Dir,
				),
			})
		}
	}
	return result, nil
}

func allProviders(settings *cli.EnvSettings, httpRequestTimeout time.Duration) getter.Providers {
	result := getter.Providers{
		{
			Schemes: []string{"http", "https"},
			New: func(options ...getter.Option) (getter.Getter, error) {
				return getter.NewHTTPGetter(append(options, getter.WithTimeout(httpRequestTimeout))...)
			},
		},
		{
			Schemes: []string{registry.OCIScheme},
			New:     getter.NewOCIGetter,
		},
	}
	pluginDownloaders, _ := collectPlugins(settings)
	result = append(result, pluginDownloaders...)
	return result
}

// RepoAdd adds a chart repository
// inspire by https://github.com/helm/helm/blob/dbc6d8e20fe1d58d50e6ed30f09a04a77e4c68db/cmd/helm/repo_add.go
// some difference with `helm repo add` command
// 1. when the same repo name is added, it will overwrite
// 2. many options we do not need now are not supported yet.
func RepoAdd(ctx context.Context, logger logr.Logger, name, url, username, password string, httpRequestTimeout time.Duration) (err error) {
	entry := repo.Entry{Name: name, URL: url, Username: username, Password: password}
	repoFile := settings.RepositoryConfig
	repoCache := settings.RepositoryCache

	// Ensure the file directory exists as it is required for file locking
	if err = os.MkdirAll(filepath.Dir(repoFile), os.ModePerm); err != nil && !os.IsExist(err) {
		return err
	}

	// Acquire a file lock for process synchronization
	repoFileExt := filepath.Ext(repoFile)
	var lockPath string
	if len(repoFileExt) > 0 && len(repoFileExt) < len(repoFile) {
		lockPath = strings.TrimSuffix(repoFile, repoFileExt) + ".lock"
	} else {
		lockPath = repoFile + ".lock"
	}
	fileLock := flock.New(lockPath)
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock() //nolint:all
	}
	if err != nil {
		return err
	}

	b, err := os.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	// Check if the repo name is legal
	if strings.Contains(entry.Name, "/") {
		return errors.Errorf("repository name (%s) contains '/', please specify a different name without '/'", entry.Name)
	}

	r, err := repo.NewChartRepository(&entry, allProviders(settings, httpRequestTimeout))
	if err != nil {
		return err
	}

	if repoCache != "" {
		r.CachePath = repoCache
	}
	if _, err := r.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", entry.URL)
	}

	f.Update(&entry)

	if err := f.WriteFile(repoFile, 0644); err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("%q has been added to your repositories", entry.Name))
	return nil
}

// RepoUpdate
// inspire by https://github.com/helm/helm/blob/dbc6d8e20fe1d58d50e6ed30f09a04a77e4c68db/cmd/helm/repo_update.go#L117
// some difference with `helm repo update` command
// 1. only support update one repo
// 2. many options we do not need now are not supported yet.
func RepoUpdate(ctx context.Context, logger logr.Logger, name string, httpRequestTimeout time.Duration) (err error) {
	log := logger.WithValues("name", name)
	repoFile := settings.RepositoryConfig
	repoCache := settings.RepositoryCache

	f, err := repo.LoadFile(repoFile)
	switch {
	case os.IsNotExist(errors.Cause(err)):
		return errors.New("no repositories found.")
	case err != nil:
		return errors.Wrapf(err, "failed loading file: %s", repoFile)
	case len(f.Repositories) == 0:
		return errors.New("no repositories found.")
	}

	var wantRepo *repo.ChartRepository
	var found bool
	for _, cfg := range f.Repositories {
		if cfg.Name != name {
			continue
		}
		found = true
		wantRepo, err = repo.NewChartRepository(cfg, allProviders(settings, httpRequestTimeout))

		if err != nil {
			return err
		}
		if repoCache != "" {
			wantRepo.CachePath = repoCache
		}
	}
	if !found {
		return errors.Errorf("no repositories found matching '%s'.  Nothing will be updated", name)
	}

	logger.Info("Hang tight while we grab the latest from your chart repositories...")

	if _, err := wantRepo.DownloadIndexFile(); err != nil {
		log.Error(err, fmt.Sprintf("Unable to get an update from the %q chart repository (%s)", wantRepo.Config.Name, wantRepo.Config.URL))
		return err
	}
	log.Info(fmt.Sprintf("Successfully got an update from the %q chart repository", wantRepo.Config.Name))
	return nil
}

// RepoRemove
// inspire by https://github.com/helm/helm/blob/dbc6d8e20fe1d58d50e6ed30f09a04a77e4c68db/cmd/helm/repo_remove.go#L117
// some difference with `helm repo remove` command
// 1. only support remove one repo
// 2. many options we do not need now are not supported yet.
func RepoRemove(ctx context.Context, logger logr.Logger, name string) (err error) {
	log := logger.WithValues("name", name)
	repoFile := settings.RepositoryConfig
	repoCache := settings.RepositoryCache
	r, err := repo.LoadFile(repoFile)
	if os.IsNotExist(errors.Cause(err)) || len(r.Repositories) == 0 {
		return errors.New("no repositories configured")
	}

	if !r.Remove(name) {
		return errors.Errorf("no repo named %q found", name)
	}
	if err := r.WriteFile(repoFile, 0644); err != nil {
		return err
	}

	idx := filepath.Join(repoCache, helmpath.CacheChartsFile(name))
	if _, err := os.Stat(idx); err == nil {
		_ = os.Remove(idx)
	}

	idx = filepath.Join(repoCache, helmpath.CacheIndexFile(name))
	if _, err := os.Stat(idx); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "can't remove index file %s", idx)
	}
	if err = os.Remove(idx); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("%q has been removed from your repositories", name))
	return nil
}
