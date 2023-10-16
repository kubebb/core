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

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/kubebb/core/pkg/helm"
)

func init() {
	EnsureRepo()
}

type Config struct {
	Install      bool
	Upgrade      bool
	RegisterName string
	Namespace    string
	Version      string
	NodeName     string // for ingress
	NodeIP       string // for ingress
	Args         values.Options
}

func NewConf(registerName string) Config {
	return Config{
		Install:      true,
		Upgrade:      false,
		RegisterName: registerName,
		Args:         values.Options{},
	}
}

var (
	store = map[string]InstallFunc{}
)

const (
	CORE             = "kubebb-core"
	CLUSTERCOMPONENT = "cluster-component"
	U4A              = "u4a-component"
	COMPONENTSTORE   = "component-store"

	KUBEBBOFFICIALREPO = "kubebb-official-repo"
	// TODO set as flag param
	KUBEBBOFFICIALREPOADDRESS = "https://kubebb.github.io/components"
	DEFAULTINSTALLREPO        = "kubebb"
)

type InstallFunc func(*Config) Installer

// Enroll Only name conflicts will return an error
func Enroll(name string, i InstallFunc) {
	store[name] = i
}

func GetInstaller(name string) InstallFunc {
	return store[name]
}

type Installer interface {
	Description() string
	Install(context.Context) error
	Upgrade(context.Context) error
	Uninstall(context.Context)
}

func EnsureRepo() {
	entry := repo.Entry{
		Name: KUBEBBOFFICIALREPO,
		URL:  KUBEBBOFFICIALREPOADDRESS,
	}
	logger := logr.Logger{}
	ctx := context.Background()
	if err := helm.RepoRemove(ctx, logger, entry.Name); err != nil && !(err.Error() == "no repositories configured" || err.Error() == "no repo named \"kubebb-official-repo\" found") {
		fmt.Fprintf(os.Stderr, "failed to add repo %s", err)
		return
	}
	fmt.Println("try to add repo")
	_ = helm.RepoAdd(ctx, logger, entry, 5*time.Second)
}
