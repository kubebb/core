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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	// DisplayNameAnnotationKey is the key of the annotation used to set the display name of the resource
	DisplayNameAnnotationKey = GroupVersion.Group + "/displayname"
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

// inspire by https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/pkg/chart/metadata.go#L26
// Maintainer describes a Chart maintainer.
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// Override defines the override settings for the component
// The value may be single-valued or multi-valued or one file
type Override struct {
	// Name is the name of the override setting
	Name string `json:"name"`
	// Value is the value of the override setting
	// +optional
	Value string `json:"value,omitempty"`
	// File is the file path of the override setting
	// +optional
	File string `json:"file,omitempty"`
	// Values is the values of the override setting
	// +optional
	Values []string `json:"values,omitempty"`
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

// GenerateComponentPlanName generates the name of the component plan for a given subscription
func GenerateComponentPlanName(sub *Subscription, version string) string {
	return "sub-" + sub.Name + "-" + sub.Spec.ComponentRef.Name + "-" + version
}

type Filter struct {
	Name, Version string
	Deprecated    bool
}

func filterMatchDeprecation(cond FilterCond, filter Filter) bool {
	return !(!cond.Deprecated && filter.Deprecated)
}
func filterMatchVersion(cond FilterCond, filter Filter) bool {
	for _, v := range cond.Versions {
		if filter.Version == v {
			return true
		}
	}
	return false
}

func filterMatchRegexp(cond FilterCond, filter Filter) bool {
	if len(cond.Regexp) > 0 {
		reg, err := regexp.Compile(cond.Regexp)
		if err == nil && reg.MatchString(filter.Version) {
			return true
		}
	}
	return false
}

func filterMatchVersionConstraint(cond FilterCond, filter Filter) bool {
	if len(cond.VersionConstraint) > 0 {
		constraint, err := semver.NewConstraint(cond.VersionConstraint)
		if err != nil {
			return false
		}
		v, err := semver.NewVersion(filter.Version)
		if err != nil {
			return false
		}
		return constraint.Check(v)
	}
	return false
}

func Match(filterCond map[string]FilterCond, filter Filter) bool {
	if len(filterCond) == 0 {
		return true
	}

	cond, ok := filterCond[filter.Name]
	if !ok {
		return true
	}

	if !filterMatchDeprecation(cond, filter) {
		return false
	}
	if filterMatchVersion(cond, filter) {
		return true
	}
	if filterMatchRegexp(cond, filter) {
		return true
	}
	if filterMatchVersionConstraint(cond, filter) {
		return true
	}
	return false
}

func IsCondSame(c1, c2 FilterCond) bool {
	return sets.NewString(c1.Versions...).Equal(sets.NewString(c2.Versions...)) &&
		c1.Deprecated == c2.Deprecated && c1.Regexp == c2.Regexp && c1.VersionConstraint == c2.VersionConstraint
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
