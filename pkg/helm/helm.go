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
	"os/exec"
	"path"
	"strings"
)

type Helm struct {
	binaryName string
	WorkDir    string
	IsHelmOCI  bool
}

func NewHelm(workDir string, isHelmOCI bool) *Helm {
	binaryName := os.Getenv("HELM_BINARYNAME")
	if binaryName == "" {
		binaryName = "helm"
	}
	return &Helm{
		binaryName: binaryName,
		WorkDir:    workDir,
		IsHelmOCI:  isHelmOCI,
	}
}

func (h *Helm) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, h.binaryName, args...)
	cmd.Dir = h.WorkDir
	cmd.Env = os.Environ()
	if h.IsHelmOCI {
		cmd.Env = append(cmd.Env, "HELM_EXPERIMENTAL_OCI=1")
	}
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("err:%s stderr:%s stdout:%s", err.Error(), errOut.String(), out.String())
	}
	if errOut.Len() > 0 {
		return "", fmt.Errorf("stderr:%s stdout:%s", errOut.String(), out.String())
	}
	return out.String(), nil
}

// RegistryLogin login to a helm registry
// FIXME  this function
func (h *Helm) RegistryLogin() error {
	//panic("not implemented")
	return nil
}

func (h *Helm) repoAdd(ctx context.Context, name string, url string) (string, error) {
	args := []string{"repo", "add"}
	// TODO add username and password
	/* TODO
	   --allow-deprecated-repos     by default, this command will not allow adding official repos that have been permanently deleted. This disables that behavior
	    --ca-file string             verify certificates of HTTPS-enabled servers using this CA bundle
	    --cert-file string           identify HTTPS client using this SSL certificate file
	    --force-update               replace (overwrite) the repo if it already exists
	-h, --help                       help for add
	    --insecure-skip-tls-verify   skip tls certificate checks for the repository
	    --key-file string            identify HTTPS client using this SSL key file
	    --no-update                  Ignored. Formerly, it would disabled forced updates. It is deprecated by force-update.
	    --pass-credentials           pass credentials to all domains
	    --password string            chart repository password
	    --password-stdin             read chart repository password from stdin
	    --username string            chart repository username
	*/
	args = append(args, name, url)
	return h.run(ctx, args...)
}

func (h *Helm) repoUpdate(ctx context.Context, name string) (string, error) {
	args := []string{"repo", "update"}
	args = append(args, name)
	return h.run(ctx, args...)
}

func (h *Helm) repoRemomve(ctx context.Context, name string) (string, error) {
	return h.run(ctx, "repo", "remove", name)
}

func (h *Helm) template(ctx context.Context, name, namespace, chart, version string, set, setString, setFile, SetJSON, SetLiteral, ValueFileName []string, skipCrd bool) (string, error) {
	// TODO need --validate?
	// TODO need --kube-version?
	// TODO need --api-versions?
	// TODO some issue https://github.com/argoproj/argo-cd/issues/7291
	args := []string{"template", name, chart}

	args = append(args, "--no-hooks", "--skip-tests") // TODO handle these args

	args = addArg(args, "namespace", namespace)
	args = addArg(args, "version", version)
	args = addArg(args, "set", set...)
	args = addArg(args, "set-string", setString...)
	args = addArg(args, "set-file", setFile...)
	args = addArg(args, "set-json", SetJSON...)
	args = addArg(args, "set-literal", SetLiteral...)
	args = addArg(args, "values", ValueFileName...)

	if !skipCrd {
		args = append(args, "--include-crds")
	}

	return h.run(ctx, args...)
}

func (h *Helm) dependencyBuild(ctx context.Context) (string, error) {
	return h.run(ctx, "dependency", "build")
}

func addArg(args []string, argKey string, argValue ...string) []string {
	if len(argValue) == 0 || argValue[0] == "" {
		return args
	}
	for _, v := range argValue {
		args = append(args, "--"+argKey, v)
	}
	return args
}

// isMissingDependencyErr tests if the error is related to a missing chart dependency
func isMissingDependencyErr(err error) bool {
	return strings.Contains(err.Error(), "found in requirements.yaml, but missing in charts") ||
		strings.Contains(err.Error(), "found in Chart.yaml, but missing in charts/ directory")
}

// workaround for Helm3 bug. Remove after https://github.com/helm/helm/issues/6870 is fixed.
// The `helm template` command generates Chart.lock after which `helm dependency build` does not work
// As workaround removing lock file unless it exists before running helm template
func cleanupChartLockFile(chartPath string) (err error) {
	lockPath := path.Join(chartPath, "Chart.lock")
	_, err = os.Stat(lockPath)
	if err != nil && os.IsNotExist(err) {
		return os.Remove(lockPath)
	}
	return
}
