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
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewInstallCmd() *cobra.Command {
	var (
		coreConf             = NewConf(CORE)
		upgradeCoreConf      = NewConf(CORE)
		clusterComponentConf = NewConf(CLUSTERCOMPONENT)
		componentStoreConf   = NewConf(COMPONENTSTORE)
		u4aConf              = NewConf(U4A)
		namespace            string
	)
	var (
		coreExtraConfString           string
		clusterCompnentExtrConfString string
		u4aExtraConfString            string
		componentStroeExtraConfString string
		kubeconfig                    string
		nodeName                      string
	)
	cmd := &cobra.Command{
		Use:  "install",
		Long: "install core, cluster-component",
		Run: func(cmd *cobra.Command, args []string) {
			background := context.Background()
			if len(coreExtraConfString) > 0 {
				coreConf.Args.Values = append(coreConf.Args.Values, strings.Split(coreExtraConfString, ",")...)
			}
			if len(clusterCompnentExtrConfString) > 0 {
				clusterComponentConf.Args.Values = append(clusterComponentConf.Args.Values, strings.Split(clusterCompnentExtrConfString, ",")...)
			}
			if len(u4aExtraConfString) > 0 {
				u4aConf.Args.Values = append(u4aConf.Args.Values, strings.Split(u4aExtraConfString, ",")...)
			}
			if len(componentStroeExtraConfString) > 0 {
				componentStoreConf.Args.Values = append(componentStoreConf.Args.Values, strings.Split(componentStroeExtraConfString, ",")...)
			}

			upgradeCoreConf.Install = false
			upgradeCoreConf.Upgrade = false
			// install cluster-component
			if !clusterComponentConf.Install {
				u4aConf.Install = false
			} else {
				clusterComponentConf.Args.Values = append(clusterComponentConf.Args.Values, fmt.Sprintf("%s=%s", mustAddForClusterComponent, nodeName))
				upgradeCoreConf.Upgrade = true
				upgradeCoreConf.Args.Values = append(upgradeCoreConf.Args.Values, "webhook.enable=true")
			}

			var nodeIP string
			if !u4aConf.Install {
				componentStoreConf.Install = false
			} else {
				once.Do(initCli)
				node := corev1.Node{}
				if err := cc.Get(background, types.NamespacedName{Name: nodeName}, &node); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to get node %s", nodeName)
					return
				}

				for _, address := range node.Status.Addresses {
					if address.Type == corev1.NodeInternalIP {
						nodeIP = address.Address
						break
					}
				}
			}

			step := 0
			taskConfs := []Config{coreConf, clusterComponentConf, upgradeCoreConf, u4aConf, componentStoreConf}
			for idx := range taskConfs {
				taskConf := taskConfs[idx]
				taskConf.Namespace = namespace
				taskConf.NodeName = nodeName
				taskConf.NodeIP = nodeIP

				if !taskConf.Install && !taskConf.Upgrade {
					fmt.Printf("skip %s", taskConf.RegisterName)
					continue
				}
				instance := GetInstaller(taskConf.RegisterName)
				if taskConf.Install && instance != nil {
					installer := instance(&taskConf)
					fmt.Printf("[%d] install %s", step, taskConf.RegisterName)
					step++

					if err := installer.Install(background); err != nil {
						fmt.Fprintf(os.Stderr, "failed to install %s, with error %s. start to uninstall\n", taskConf.RegisterName, err)
						// TODO uninstall
						return
					}
					fmt.Printf("\t%s install done\n", taskConf.RegisterName)
					if idx == 0 {
						fmt.Println("\twait components")
						// TODO: check components
						time.Sleep(20 * time.Second)
					}
					continue
				}

				if taskConf.Upgrade && instance != nil {
					installer := instance(&taskConf)
					fmt.Printf("[%d] upgrade %s", step, taskConf.RegisterName)
					step++

					if err := installer.Upgrade(background); err != nil {
						fmt.Fprintf(os.Stderr, "failed to upgrade %s, with error %s. start to uninstall\n", taskConf.RegisterName, err)
						// TODO uninstall
						return
					}
					fmt.Printf("\t%s upgrade done\n", taskConf.RegisterName)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&clusterComponentConf.Install, "cluster-component", true, "install cluster-component?")
	cmd.Flags().BoolVar(&u4aConf.Install, "u4a", true, "install u4a")
	cmd.Flags().BoolVar(&componentStoreConf.Install, "component-store", true, "install component-store?")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "install namespace")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "")
	cmd.Flags().StringVar(&coreConf.Version, "core-version", "", "")
	cmd.Flags().StringVar(&clusterComponentConf.Version, "cc-version", "", "")
	cmd.Flags().StringVar(&u4aConf.Version, "u4a-version", "", "")
	cmd.Flags().StringVar(&componentStoreConf.Version, "cs-version", "", "")
	cmd.Flags().StringVar(&nodeName, "node-name", "", "")

	cmd.Flags().StringVar(&coreExtraConfString, "core-extra-conf", "", "a=b,c=d")
	cmd.Flags().StringVar(&clusterCompnentExtrConfString, "cluster-component-extra-conf", "", "a=b,c=d")
	cmd.Flags().StringVar(&u4aExtraConfString, "u4a-extra-conf", "", "a=b,c=d")
	cmd.Flags().StringVar(&componentStroeExtraConfString, "component-store-extra-conf", "", "a=b,c=d")
	return cmd
}
