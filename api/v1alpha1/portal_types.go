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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PortalSpec defines the desired state of Portal
type PortalSpec struct {
	// the path for request acccessing
	Path string `json:"path"`
	// the path of the static file
	Entry string `json:"entry"`
}

// PortalStatus defines the observed state of Portal
type PortalStatus struct {
	// conflicted portals with same Entry
	ConflictsInEntry []string `json:"conflictsInEntry"`
	// conflicted portals with same Path
	ConflictsInPath []string `json:"conflictsInPath"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// Portal is the Schema for the portals API
type Portal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PortalSpec   `json:"spec,omitempty"`
	Status PortalStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PortalList contains a list of Portal
type PortalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Portal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Portal{}, &PortalList{})
}
