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

package v1alpha1

import (
	"regexp"

	"github.com/Masterminds/semver/v3"
	hrepo "helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kustomize "sigs.k8s.io/kustomize/api/types"
)

const (
	// DisplayNameAnnotationKey is the key of the annotation used to set the display name of the resource
	DisplayNameAnnotationKey = Group + "/displayname"
	// Finalizer is the key of the finalizer
	Finalizer = Group + "/finalizer"
)

// ComponentVersion Indicates the fields required for a specific version of Component.
type ComponentVersion struct {
	Version    string      `json:"version"`
	AppVersion string      `json:"appVersion"`
	UpdatedAt  metav1.Time `json:"updatedAt"`
	CreatedAt  metav1.Time `json:"createdAt"`
	Digest     string      `json:"digest"`
	Deprecated bool        `json:"deprecated"`
}

// Equal compares two ComponetVersions, ignoring UpdatedAt and CreatedAt fields
func (c ComponentVersion) Equal(v *ComponentVersion) bool {
	return c.Digest == v.Digest && c.AppVersion == v.AppVersion && c.Version == v.Version && c.Deprecated == v.Deprecated
}

// Maintainer describes a Chart maintainer.
// inspire by https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/pkg/chart/metadata.go#L26
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// Override defines the override settings for the component
// FIXME fix comment
type Override struct {
	// Values is passed to helm install --values or -f
	// specify values in a YAML file or a URL (can specify multiple)
	Values []string `json:"values,omitempty"`
	// Set is passed to helm install --set
	// set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
	Set []string `json:"set,omitempty"`
	// SetString is passed to helm install --set-string
	// set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
	// https://github.com/helm/helm/pull/3599
	SetString []string `json:"set-string,omitempty"`
	// SetFile is passed to helm install --set-file
	// set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
	// https://github.com/helm/helm/pull/3758
	SetFile []string `json:"set-file,omitempty"`
	// SetJSON is passed to helm install --set-json
	// set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)
	// https://github.com/helm/helm/pull/10693
	SetJSON []string `json:"set-json,omitempty"`
	// SetLiteral is passed to helm install --set-literal
	// set a literal STRING value on the command line
	// https://github.com/helm/helm/pull/9182
	SetLiteral []string `json:"set-literal,omitempty"`

	// Images for replace old image
	// see https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/images
	// +optional
	Images []kustomize.Image `json:"images,omitempty"`
}

// NameConfig defines the name of helm release
// If Name and NameTemplate are both set, will use Name first.
// If both not set, will use helm install --generate-name
type NameConfig struct {
	// Name is pass to helm install <chart> <name>, name arg
	Name string `json:"name,omitempty"`
	// NameTemplate is pass to helm install --name-template
	// FIXME add logic
	NameTemplate string `json:"nameTemplate,omitempty"`
}

// Config defines the configuration of the ComponentPlan
// Greatly inspired by https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/cmd/helm/install.go#L161
// And https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/cmd/helm/upgrade.go#L70
// Note: we will helm INSTALL release if not exists or helm UPGRADE if exists.**
// Note: helm release will be installed/upgraded in same namespace with ComponentPlan, So no args like --create-namespace
// Note: helm release will be installed/upgraded, so no args like --dry-run
// Note: helm release will be installed/upgraded without show notes, so no args like --render-subchart-notes
// Note: helm release will be upgraded with Override Config, so no args like --reset-values or --reuse-values
// TODO: we should consider hooks, --no-hooks helm template --hooks
type Config struct {
	Override Override `json:"override,omitempty"`

	NameConfig `json:",inline"`

	// FIXME reconsider there config because we will use helm template not helm install
	// Force is pass to helm install/upgrade --force
	// force resource updates through a replacement strategy
	Force bool `json:"force,omitempty"`

	// Replace is pass to helm install --replace
	// re-use the given name, only if that name is a deleted release which remains in the history. This is unsafe in production
	Replace bool `json:"replace,omitempty"`

	// TimeoutSeconds is pass to helm install/upgrade --timeout, default is 300s
	// time to wait for any individual Kubernetes operation (like Jobs for hooks)
	TimeOutSeconds int `json:"timeoutSeconds,omitempty"`

	// Wait is pass to helm install/upgrade --wait
	// if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment, StatefulSet, or ReplicaSet are in a ready state before marking the release as successful. It will wait for as long as --timeout
	Wait bool `json:"wait,omitempty"`

	// WaitForJobs is pass to helm install/upgrade --wait-for-jobs
	// if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout
	WaitForJobs bool `json:"waitForJobs,omitempty"`

	// Description is pass to helm install/upgrade --description
	// add a custom description
	Description string `json:"description,omitempty"`

	// Devel is pass to helm install/upgrade --devel
	// use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored
	Devel bool `json:"devel,omitempty"`

	// DependencyUpdate is pass to helm install/upgrade --dependency-update
	// update dependencies if they are missing before installing the chart
	DependencyUpdate bool `json:"dependencyUpdate,omitempty"`

	// DisableOpenAPIValidation is pass to helm install/upgrade --disable-openapi-validation
	// if set, the installation process will not validate rendered templates against the Kubernetes OpenAPI Schema
	DisableOpenAPIValidation bool `json:"disableOpenAPIValidation,omitempty"`

	// Atomic is pass to helm install/upgrade --atomic
	// if set, the installation process deletes the installation on failure. The --wait flag will be set automatically if --atomic is used
	Atomic bool `json:"atomic,omitempty"`

	// SkipCRDs is pass to helm install/upgrade --skip-crds
	// if set, no CRDs will be installed. By default, CRDs are installed if not already present
	SkipCRDs bool `json:"skipCRDs,omitempty"`

	// EnableDNS is pass to helm install/upgrade --enable-dns
	// enable DNS lookups when rendering templates
	EnableDNS bool `json:"enableDNS,omitempty"`

	// Recreate is pass to helm upgrade --recreate-pods
	// performs pods restart for the resource if applicable
	Recreate bool `json:"recreate-pods,omitempty"`

	// MaxRetry
	MaxRetry *int64 `json:"maxRetry,omitempty"`
}

// UpdateCondWithFixedLen updates the Conditions of the resource and limits the length of the Conditions field to l.
// If l is less than or equal to 0, it means that the length is not limited.
//
// Example:
//
//	conds.Conditions=[a, b, c], l=2, cond=d -> conds.Conditions=[c, d]
func UpdateCondWithFixedLen(l int, conds *ConditionedStatus, cond Condition) {
	if ll := len(conds.Conditions); ll >= l && l > 0 {
		conds.Conditions = conds.Conditions[ll-l+1:]
	}
	conds.Conditions = append(conds.Conditions, cond)
}

// +kubebuilder:object:generate=false
type FilterFunc func(FilterCond, string) bool

var defaultFilterFuncs = []FilterFunc{FilterMatchVersion, FilterMatchVersionRegexp, FilterMatchVersionConstraint}

// +kubebuilder:object:generate=false
type Filter struct {
	Name     string
	Versions []*hrepo.ChartVersion
}

func FilterMatchVersion(cond FilterCond, version string) bool {
	for _, v := range cond.VersionedFilterCond.Versions {
		if v == version {
			return true
		}
	}
	return false
}

func FilterMatchVersionRegexp(cond FilterCond, version string) bool {
	if len(cond.VersionedFilterCond.VersionRegexp) > 0 {
		reg, err := regexp.Compile(cond.VersionedFilterCond.VersionRegexp)
		if err == nil && reg.MatchString(version) {
			return true
		}
	}
	return false
}

func FilterMatchVersionConstraint(cond FilterCond, version string) bool {
	if len(cond.VersionedFilterCond.VersionConstraint) > 0 {
		constraint, err := semver.NewConstraint(cond.VersionedFilterCond.VersionConstraint)
		if err != nil {
			return false
		}
		v, err := semver.NewVersion(version)
		if err != nil {
			return false
		}
		return constraint.Check(v)
	}
	return false
}

// Match determines if this component is retained, and if so, filters for conforming versions.
func Match(fc map[string]FilterCond, filter Filter, funcs ...FilterFunc) ([]int, bool) {
	var versions []int
	if len(funcs) == 0 {
		funcs = defaultFilterFuncs
	}
	if len(fc) == 0 {
		for i := range filter.Versions {
			versions = append(versions, i)
		}
		return versions, true
	}
	filterCond, ok := fc[filter.Name]
	if ok && filterCond.Operation == FilterOpIgnore {
		return versions, false
	}

	for i, v := range filter.Versions {
		if v.Deprecated && !filterCond.Deprecated {
			continue
		}

		for _, f := range funcs {
			if f(filterCond, v.Version) {
				versions = append(versions, i)
				break
			}
		}
	}

	return versions, true
}

func IsCondSame(c1, c2 FilterCond) bool {
	return c1.Name == c2.Name && c1.Deprecated == c2.Deprecated && c1.Operation == c2.Operation &&
		((c1.VersionedFilterCond == nil && c2.VersionedFilterCond == nil) ||
			(c1.VersionedFilterCond != nil && c2.VersionedFilterCond != nil &&
				sets.NewString(c1.VersionedFilterCond.Versions...).Equal(sets.NewString(c2.VersionedFilterCond.Versions...)) &&
				c1.VersionedFilterCond.VersionRegexp == c2.VersionedFilterCond.VersionRegexp && c1.VersionedFilterCond.VersionConstraint == c2.VersionedFilterCond.VersionConstraint))
}

func IsFilterSame(cond1, cond2 map[string]FilterCond) bool {
	l1, l2 := len(cond1), len(cond2)
	if l1 == 0 && l2 == 0 {
		return true
	}
	if l1 != l2 {
		return false
	}

	for name, cond := range cond1 {
		v1, ok := cond2[name]
		if !ok || !IsCondSame(cond, v1) {
			return false
		}
	}
	return true
}
