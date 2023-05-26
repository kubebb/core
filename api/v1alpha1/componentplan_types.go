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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ComponentPlanSpec defines the desired state of ComponentPlan
type ComponentPlanSpec struct {
	// ComponentRef is a reference to the Component
	ComponentRef *corev1.ObjectReference `json:"component"`
	// InstallVersion represents the version that is to be installed by this ComponentPlan
	InstallVersion string `json:"version"`
	// Approved indicates whether the ComponentPlan has been approved
	Approved bool `json:"approved"`
	// Override defines the override settings for the component
	Override []Override `json:"override,omitempty"`
}

// ComponentPlanStatus defines the observed state of ComponentPlan
type ComponentPlanStatus struct {
	ConditionedStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// ComponentPlan is the Schema for the componentplans API
type ComponentPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentPlanSpec   `json:"spec,omitempty"`
	Status ComponentPlanStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentPlanList contains a list of ComponentPlan
type ComponentPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentPlan{}, &ComponentPlanList{})
}
