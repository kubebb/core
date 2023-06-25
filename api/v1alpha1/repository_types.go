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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FilterOp string

const (
	FilterOpKeep   FilterOp = "keep"
	FilterOpIgnore FilterOp = "ignore"
)

// PullStategy for pulling components in repository
type PullStategy struct {
	// Interval for pulling
	IntervalSeconds int `json:"intervalSeconds,omitempty"`

	// Timeout for pulling
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`

	// Retry upon timeout
	Retry int `json:"retry,omitempty"`
}

type VersionedFilterCond struct {
	// Accurately match each item in the versions
	Versions []string `json:"versions,omitempty"`
	// Filter version by regexp
	VersionRegexp string `json:"regexp,omitempty"`
	// VersionConstraint Support for user-defined version ranges, etc.
	// Refer to the documentation for more details
	// https://github.com/Masterminds/semver#semver
	VersionConstraint string `json:"versionConstraint,omitempty"`
}

type FilterCond struct {
	// Name of the component
	Name string `json:"name,omitempty"`

	// default is keep
	// +kubebuilder:validation:Enum=keep;ignore
	// +kubebuilder:default:=keep
	Operation FilterOp `json:"operation,omitempty"`

	// If True, the current version will be retained even if it is deprecated.
	Deprecated bool `json:"deprecated,omitempty"`

	// VersionedFilterCond filters which version in component are pulled/ignored from the repository
	VersionedFilterCond *VersionedFilterCond `json:"versionedFilterCond,omitempty"`
}

// RepositorySpec defines the desired state of Repository
type RepositorySpec struct {
	// URL chart repository address
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// AuthSecret if the chart repository requires auth authentication,
	// set the username and password to secret, with the fields user and password respectively.
	AuthSecret string `json:"authSecret,omitempty"`

	Insecure bool `json:"insecure,omitempty"`

	RepositoryType string `json:"repositoryType,omitempty"`

	// PullStategy pullStategy for this repository
	PullStategy *PullStategy `json:"pullStategy,omitempty"`

	Filter []FilterCond `json:"filter,omitempty"`
	// ImageOverride means replaced images rules for this repository
	ImageOverride []ImageOverride `json:"imageOverride,omitempty"`
}

type PathOverride struct {
	// The path consists of slash-separated components.
	// Each component may contain lowercase letters, digits and separators.
	// A separator is defined as a period, one or two underscores, or one or more hyphens.
	// A component may not start or end with a separator.
	// While the OCI Distribution Specification supports more than two slash-separated components, most registries only support two slash-separated components.
	// For Docker’s public registry, the path format is as follows: [NAMESPACE/]REPOSITORY:
	//   The first, optional component is typically a user’s or an organization’s namespace.
	//   The second, mandatory component is the repository name. When the namespace is not present, Docker uses library as the default namespace.
	Path    string `json:"path,omitempty"`
	NewPath string `json:"newPath,omitempty"`
}

type ImageOverride struct {
	// Registry include host and port number, like `registry-1.docker.io` or `registry-1.docker.io:5000`
	Registry string `json:"registry,omitempty"`
	// NewRegistry means replaced one
	NewRegistry string `json:"newRegistry,omitempty"`
	// PathOverride means replaced path
	PathOverride *PathOverride `json:"pathOverride,omitempty"`
}

// RepositoryStatus defines the observed state of Repository
type RepositoryStatus struct {
	// URLHistory URL change history
	URLHistory []string `json:"urlHistory,omitempty"`
	// ConditionedStatus is the current status
	ConditionedStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=repo;repos

// Repository is the Schema for the repositories API
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec   `json:"spec,omitempty"`
	Status RepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RepositoryList contains a list of Repository
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Repository{}, &RepositoryList{})
}
