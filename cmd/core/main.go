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

package main

import (
	"github.com/spf13/cobra"

	"github.com/kubebb/core/pkg/cmd"
)

var longUsage = `Quickly deploy the kubebb platform in the existing k8s environment.

The platform mainly includes four components: core, cluster-component, u4a, and component-store.

For more details, please refer to https://kubebb.github.io/website

Examples:
    # install kubebb to a cluster
    corectl install --context ~/.kube/devconfig [--skip-component-store|--skip-u4a-component|--skip-cluster-component]
`

func main() {
	rootCmd := cobra.Command{
		Use:   "corectl",
		Short: "Quickly deploy the kubebb platform in the existing k8s environment",
		Long:  longUsage,
	}

	rootCmd.AddCommand(cmd.NewInstallCmd())
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
