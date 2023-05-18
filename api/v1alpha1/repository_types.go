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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RepositoryCondReason string

// Reasons a resource is or is not ready.
const (
	ReasonAvailable   RepositoryCondReason = "Available"
	ReasonUnavailable RepositoryCondReason = "Unavailable"
	ReasonCreating    RepositoryCondReason = "Creating"
	ReasonDeleting    RepositoryCondReason = "Deleting"
)

// Reasons a resource is or is not synced.
const (
	ReasonReconcileSuccess RepositoryCondReason = "ReconcileSuccess"
	ReasonReconcileError   RepositoryCondReason = "ReconcileError"
	ReasonReconcilePaused  RepositoryCondReason = "ReconcilePaused"
)

type RepositoryCondType string

const (
	// TypeReady resources are believed to be ready to handle work.
	TypeReady RepositoryCondType = "Ready"

	// TypeSynced Get index.yaml error, or other error occurred, add a record
	TypeSynced RepositoryCondType = "Synced"
)

/*
type RepositoryAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	CertData []byte `json:"certData,omitempty"`
	KeyData  []byte `json:"keyData,omitempty"`

	// CAData If certification is required and the certificate is self-signed,
	// you need to provide the server's certificate information.
	CaData []byte `json:"caData,omitempty"`

	//will not validate the repository's certificate
	Insecure bool `json:"insecure,omitempty"`
}
*/

// PullStategy for pulling components in repository
type PullStategy struct {
	// Interval for pulling
	IntervalSeconds int `json:"intervalSeconds,omitempty"`

	// Timeout for pulling
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`

	// Retry upon timeout
	Retry int `json:"retry,omitempty"`
}

// RepositorySpec defines the desired state of Repository
type RepositorySpec struct {
	// URL chart repository address
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// RepositoryAuth if the chart repository requires auth authentication,
	// set the username and password to secret, with the fields user and password respectively.
	RepositoryAuth string `json:"repositoryAuth,omitempty"`

	Insecure bool `json:"insecure,omitempty"`

	RepositoryType string `json:"repositoryType,omitempty"`

	// PullStategy pullStategy for this repository
	PullStategy *PullStategy `json:"pullStategy,omitempty"`
}

type RepositoryCondition struct {
	// Status of this condition; is it currently True, False, or Unknown?
	Status v1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time this condition transitioned from one
	// status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// A Reason for this condition's last transition from one status to another.
	Reason RepositoryCondReason `json:"reason"`

	// A Message containing details about this condition's last transition from
	// one status to another, if any.
	// +optional
	Message string `json:"message,omitempty"`

	// +kubebuilder:validation:Required
	Type RepositoryCondType `json:"type"`
}

// RepositoryStatus defines the observed state of Repository
type RepositoryStatus struct {
	// URLHistory URL change history
	URLHistory []string `json:"urlHistory,omitempty"`

	Conditions RepositoryCondition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

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
