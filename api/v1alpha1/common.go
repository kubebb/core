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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	hrepo "helm.sh/helm/v3/pkg/repo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

// ValuesReference contains a reference to a resource containing Helm values,
// and optionally the key they can be found at.
type ValuesReference struct {
	// Kind of the values referent, valid values are ('Secret', 'ConfigMap').
	// +kubebuilder:validation:Enum=Secret;ConfigMap
	// +required
	Kind string `json:"kind"`

	// Name of the values' referent. Should reside in the same namespace as the
	// referring resource.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`

	// ValuesKey is the data key where the values.yaml or a specific value can be
	// found at. Defaults to 'values.yaml'.
	// When set, must be a valid Data Key, consisting of alphanumeric characters,
	// '-', '_' or '.'.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[\-._a-zA-Z0-9]+$`
	// +optional
	ValuesKey string `json:"valuesKey,omitempty"`

	// TargetPath is the YAML dot notation path the value should be merged at. When
	// set, the ValuesKey is expected to be a single flat value. Defaults to 'None',
	// which results in the values getting merged at the root.
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9_\-.\\\/]|\[[0-9]{1,5}\])+$`
	// +optional
	TargetPath string `json:"targetPath,omitempty"`
}

func (v *ValuesReference) GetValuesKey() string {
	if len(v.ValuesKey) == 0 {
		return "values.yaml"
	}
	return v.ValuesKey
}

// GetValuesFileDir returns the dir path to this ValuesReference file,
// for example: $HOME/.cache/helm/secret.default.testone
func (v *ValuesReference) GetValuesFileDir(helmCacheHome, namespace string) string {
	return filepath.Join(helmCacheHome, strings.ToLower(v.Kind)+"."+namespace+"."+v.Name)
}

// Override defines the override settings for the component
type Override struct {
	// Values is passed to helm install --values or -f
	// specify values in a YAML file or a URL (can specify multiple)
	// ValuesFrom holds references to resources containing Helm values for this HelmRelease,
	// and information about how they should be merged.
	ValuesFrom []*ValuesReference `json:"valuesFrom,omitempty"`
	// Values holds the values for this Helm release.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
	// Set is passed to helm install --set
	// can specify multiple or separate values with commas: key1=val1,key2=val2
	// Helm also provides other set options, such as --set-json or --set-literal,
	// which can be replaced by values or valuesFrom fields.
	Set []string `json:"set,omitempty"`
	// SetString is passed to helm install --set-string
	// set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
	// https://github.com/helm/helm/pull/3599
	// Helm also provides other set options, such as --set-json or --set-literal,
	// which can be replaced by values or valuesFrom fields.
	SetString []string `json:"set-string,omitempty"`

	// Images for replace old image
	// see https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/images
	// +optional
	Images []kustomize.Image `json:"images,omitempty"`
}

// GetValueFileDir returns the dir path to Override.Value file,
// for example: $HOME/.cache/helm/embed.default.testone
func (v *Override) GetValueFileDir(helmCacheHome, namespace, name string) string {
	return filepath.Join(helmCacheHome, "embed."+namespace+"."+name)
}

// Config defines the configuration of the ComponentPlan
// Greatly inspired by https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/cmd/helm/install.go#L161
// And https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/cmd/helm/upgrade.go#L70
// Note: we will helm INSTALL release if not exists or helm UPGRADE if exists.**
// Note: no --create-namespace, bacause helm relase will install in componentplan's namespace.
// Note: no --dry-run, bacause no need to config simulate.
// Note: no --replace, because re-use the given name is not safe in production
// Note: no --render-subchart-notes, bacause we do not show notes.
// Note: no --devel config, because it equivalent to version '>0.0.0-0'.
// Note: no --nameTemplate config, because we need a determined name, nameTemplate may produce different results when it is run multiple times.
// Note: no --generateName config with the same reason above.
// Note: no --reset-values or --reuse-values config, because we use Override config
// Note: other args like --username or --cert-file should setted in repo CRD.
// TODO: should we support --post-renderer and --post-renderer-args ?
// TODO: add --verify --keyring config after we handle keyring config
type Config struct {
	Override Override `json:"override,omitempty"`

	// Name is pass to helm install <chart> <name>, name arg
	Name string `json:"name,omitempty"`

	// Force is pass to helm upgrade --force
	// force resource updates through a replacement strategy
	Force bool `json:"force,omitempty"`

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

	// DependencyUpdate is pass to helm install/upgrade --dependency-update
	// update dependencies if they are missing before installing the chart
	DependencyUpdate bool `json:"dependencyUpdate,omitempty"`

	// DisableHooks is pass to helm install/upgrade --no-hooks
	// if set, prevent hooks from running during install and disable pre/post upgrade hooks
	DisableHooks bool `json:"disableHooks,omitempty"`

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

	// CleanupOnFail is pass to helm upgrade --cleanup-on-fail
	// allow deletion of new resources created in this upgrade when upgrade fails
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`

	// KeepHistory is paas to helm uninstall --keep-history
	// remove all associated resources and mark the release as deleted, but retain the release history.
	KeepHistory bool `json:"keepHistory,omitempty"`

	// MaxHistory is pass to helm upgrade --history-max
	// limit the maximum number of revisions saved per release. Use 0 for no limit
	MaxHistory *int `json:"historyMax,omitempty"`

	// MaxRetry
	MaxRetry *int `json:"maxRetry,omitempty"`
}

func (c *Config) Timeout() time.Duration {
	if c.TimeOutSeconds == 0 {
		return 300 * time.Second // default value in helm install/upgrade --timeout
	}
	return time.Duration(c.TimeOutSeconds) * time.Second
}
func (c *Config) GetMaxHistory() int {
	if c.MaxHistory == nil {
		return 10 // default value in helm upgrade --history-max
	}
	return *c.MaxHistory
}
func (c *Config) GetMaxRetry() int {
	if c.MaxRetry == nil {
		return 5
	}
	return *c.MaxRetry
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
	var (
		versions []int
		keep     bool
	)
	if len(funcs) == 0 {
		funcs = defaultFilterFuncs
	}

	// When the chart package name is not in the fc, we will keep all versions of the chart package.
	filterCond, ok := fc[filter.Name]
	if !ok {
		for i := range filter.Versions {
			versions = append(versions, i)
		}
		return versions, true
	}
	// If filterCond.VersionedFilterCond=nil, it means that
	// we determine whether to keep the version of the package based on filterCond.KeepDeprecated.
	if filterCond.VersionedFilterCond == nil {
		if filterCond.Operation == FilterOpIgnore {
			return versions, false
		}
		for i, v := range filter.Versions {
			if filterCond.KeepDeprecated || !v.Deprecated {
				versions = append(versions, i)
			}
		}
		return versions, len(versions) != 0
	}
	// If operation=keep, a version can be kept as long as it satisfies a certain filter function
	// If operation=ignore, a version is kept only if all filter functions are not satisfied.
	// Then, based on filterCond.KeepDeprecated, determine whether to keep the version or not.
	for i, v := range filter.Versions {
		keep = filterCond.Operation == FilterOpIgnore
		for _, f := range funcs {
			if f(filterCond, v.Version) {
				if filterCond.Operation == FilterOpKeep {
					keep = true
				}
				if filterCond.Operation == FilterOpIgnore {
					keep = false
				}
				break
			}
		}
		if keep {
			if !filterCond.KeepDeprecated && v.Deprecated {
				keep = false
			}
		}
		if keep {
			versions = append(versions, i)
		}
	}

	return versions, len(versions) != 0
}

func IsCondSame(c1, c2 FilterCond) bool {
	return c1.Name == c2.Name && c1.KeepDeprecated == c2.KeepDeprecated && c1.Operation == c2.Operation &&
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
