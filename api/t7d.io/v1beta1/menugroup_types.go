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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MenuType string

const (
	//所有产品
	AllProduct MenuType = "all-product"
	//平台管理
	PlatformManage MenuType = "platform-manage"
	// 用户面板
	UserPanel MenuType = "user-panel"
)

// MenuGroupSpec defines the Position of the menu
type MenuGroupSpec struct {
	Type MenuType `json:"type"`
	// Menu name
	Name string `json:"name"`
	// menu column postion
	Column uint8 `json:"column"`
	// ranking  in Column, the smaller the number, the higher postion 菜单组在当前列中的排序，数字越小越靠前
	Ranking uint8 `json:"ranking"`
}

// MenuGroupStatus defines the observed state of MenuGroup
type MenuGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// MenuGroup is the Schema for the menugroups API
type MenuGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MenuGroupSpec   `json:"spec,omitempty"`
	Status MenuGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MenuGroupList contains a list of MenuGroup
type MenuGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MenuGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MenuGroup{}, &MenuGroupList{})
}
