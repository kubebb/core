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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kubebb/core/pkg/helm"
)

func init() {
	Enroll(CORE, CoreInstaller)
}

func CoreInstaller(cfg *Config) Installer {
	return &Core{cfg: cfg}
}

type Core struct {
	cfg *Config
}

func (c *Core) Description() string {
	return fmt.Sprintf("Install %s", c.cfg.RegisterName)
}

func (c *Core) Install(ctx context.Context) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't get cluster config %s", err)
		return fmt.Errorf("[%s] Failed to get cluster config %w", CORE, err)
	}
	getter := genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &c.cfg.Namespace,
	}
	fmt.Printf("\t[%s] construct new helm wrapper\n", CORE)
	logger := logr.Logger{}
	hl, err := helm.NewHelmWrapper(&getter, c.cfg.Namespace, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new helm wrapper error %s", err)
		return err
	}

	actionClient := hl.GetDefaultInstallCfg()
	actionClient.Version = c.cfg.Version
	fmt.Printf("\t[%s] helm install %s %s/kubebb-core\n", CORE, CORE, KUBEBBOFFICIALREPO)
	_, _, err = hl.Install(ctx, logger, actionClient, &c.cfg.Args, "kubebb-core", fmt.Sprintf("%s/kubebb-core", KUBEBBOFFICIALREPO))
	if err != nil {
		fmt.Fprintf(os.Stderr, "faile to install %s with error: %s", CORE, err)
		return err
	}
	once.Do(initCli)
	fmt.Printf("\t[%s] wait %s's deployments\n", CORE, CORE)

	return WaitDeployment(ctx, cc, c.cfg.Namespace, []string{CORE}, 60)
}

func (c *Core) Upgrade(ctx context.Context) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't get cluster config %s", err)
		return fmt.Errorf("[%s] Failed to get cluster config %w", CORE, err)
	}
	getter := genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &c.cfg.Namespace,
	}
	fmt.Printf("\t[%s] upgrade construct new helm wrapper\n", CORE)
	logger := logr.Logger{}
	hl, err := helm.NewHelmWrapper(&getter, c.cfg.Namespace, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new helm wrapper error %s", err)
		return err
	}

	actionClient, createNs := hl.GetDefaultUpgradeCfg()
	fmt.Printf("\t[%s] helm upgrade %s\n", CORE, CORE)

	_, _, err = hl.Upgrade(ctx, logger, actionClient, &c.cfg.Args, CORE, fmt.Sprintf("%s/kubebb-core", KUBEBBOFFICIALREPO), createNs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to upgrade %s with error: %s", CORE, err)
		return err
	}
	fmt.Printf("\t sleep 30s for %s pod running\n", CORE)
	time.Sleep(30 * time.Second)

	return WaitDeployment(ctx, cc, c.cfg.Namespace, []string{CORE}, 30)
}

func (c *Core) Uninstall(ctx context.Context) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[core operator] can't get cluster config %s", err)
	}
	getter := genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &c.cfg.Namespace,
	}
	fmt.Println("\tuninstall construct new helm wrapper")
	logger := logr.Logger{}
	hl, err := helm.NewHelmWrapper(&getter, c.cfg.Namespace, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new helm wrapper error %s", err)
		return
	}

	actionClient := hl.GetDefaultUninstallCfg()
	_, _ = hl.Uninstall(ctx, logger, actionClient, CORE)
}
