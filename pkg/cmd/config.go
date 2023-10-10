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
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
)

type config struct {
	// {"core": []string{"webhook.enalbe=true"}}
	installers map[string][]string
	// maybe for helm client
	cfg *rest.Config // nolint
	// If necessary, define other parameters here
}

type InstallerFunc func(*config) Installer

var (
	store map[string]InstallerFunc
)

const (
	CORE             = "core"
	CLUSTERCOMPONENT = "cluster-component"
	U4A              = "u4a"
	COMPONENTSTORE   = "component-store"
)

// Enroll Only name conflicts will return an error
func Enroll(name string, instance InstallerFunc) {
	store[name] = instance
}

func GetInstaller(name string) InstallerFunc {
	return store[name]
}

type Installer interface {
	Description() string
	Install() error
	Uninstall()
}

func NewInstallCmd() *cobra.Command {
	var (
		installClusterComponent, installU4A, installComponentStore bool
	)
	cmd := &cobra.Command{
		Use:  "install",
		Long: "install core, cluster-component",
		Run: func(cmd *cobra.Command, args []string) {
			initConf := config{
				// Must install core
				installers: map[string][]string{
					CORE:             {},
					CLUSTERCOMPONENT: {},
					U4A:              {},
					COMPONENTSTORE:   {},
				},
			}
			if installClusterComponent { // nolint
				// if install cluster-component, we will upgrade core
				// TODO
			}
			if installU4A { // nolint
				// make sure cluster-component is set
				// TODO
			}
			if installComponentStore { // nolint
				// TODO
			}

			// get complete config
			installTasks := []string{CORE, CLUSTERCOMPONENT, U4A, COMPONENTSTORE}
			tasks := make([]Installer, 0)
			for _, task := range installTasks {
				if _, ok := initConf.installers[task]; ok {
					if fn := GetInstaller(task); fn != nil {
						tasks = append(tasks, fn(&initConf))
					}
				}
			}

			for idx, installer := range tasks {
				fmt.Printf("Step %d: %s\n", idx+1, installer.Description())
				if err := installer.Install(); err != nil {
					// uninstall
					for i := idx; i >= 0; i-- {
						tasks[i].Uninstall()
					}
				}
			}
		},
	}

	cmd.Flags().BoolVar(&installClusterComponent, "cluster-component", false, "install cluster-component?")
	cmd.Flags().BoolVar(&installU4A, "u4a", false, "install u4a?")
	cmd.Flags().BoolVar(&installComponentStore, "component-store", false, "install component-store?")
	return cmd
}
