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

// ParamType indicates the type of an input parameter;
// Used to distinguish between a single string and an array of strings.
type ParamType string

// Valid ParamTypes:
const (
	ParamTypeString ParamType = "string"
	ParamTypeArray  ParamType = "array"
	ParamTypeObject ParamType = "object"
)

// ParamValue is a type that can hold a single string or string array.
// Used in JSON unmarshalling so that a single JSON field can accept
// either an individual string or an array of strings.
type ParamValue struct {
	// +kubebuilder:validation:Enum:=string;array;object
	Type      ParamType `json:"type"` // Represents the stored type of ParamValues.
	StringVal string    `json:"stringVal,omitempty"`
	// +listType=atomic
	ArrayVal  []string          `json:"arrayVal,omitempty"`
	ObjectVal map[string]string `json:"objectVal,omitempty"`
}

type Param struct {
	Name  string     `json:"name"`
	Value ParamValue `json:"value"`
}

type PipelineParam struct {
	// PipelineName the name of pipeline
	PipelineName string `json:"pipelineName"`

	// Params List of parameters defined in the pipeline
	// +listType=atomic
	Params []Param `json:"params,omitempty"`
}

type Task struct {
	Name        string `json:"name"`
	TaskRunName string `json:"taskRunName,omitempty"`

	Type string `json:"type,omitempty"`

	ConditionedStatus `json:",inline"`
}

type RatingSpec struct {
	// ComponentName Each Rating corresponds to a component
	ComponentName string `json:"componentName"`

	PipelineParams []PipelineParam `json:"pipelineParams"`
}

type RatingStatus struct {
	PipelineRuns map[string]PipelineRunStatus `json:"pipelineRuns,omitempty"`

	// ExpectWeight Each pipeline contains multiple tasks. The default weight of each task is 1.
	// This field describes the sum of the weights of all tasks included in the pipeline defined in Rating.
	ExpectWeight int `json:"expectWeight,omitempty"`

	// ActualWeight The sum of all successful task weights.
	ActualWeight int `json:"actualWeight,omitempty"`
}

type PipelineRunStatus struct {
	Tasks        []Task `json:"tasks,omitempty"`
	PipelineName string `json:"pipelineName"`

	ConditionedStatus `json:",inline"`

	// ExpectWeight Each pipeline contains multiple tasks. The default weight of each task is 1.
	// This field describes the sum of the weights of all tasks included in the pipeline defined in Rating.
	ExpectWeight int `json:"expectWeight,omitempty"`

	// ActualWeight The sum of all successful task weights.
	ActualWeight int `json:"actualWeight,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

type Rating struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RatingSpec   `json:"spec,omitempty"`
	Status RatingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type RatingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Rating `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rating{}, &RatingList{})
}
