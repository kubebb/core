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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MenuSpec defines the desired state of Menu
type MenuSpec struct {
	// 菜单组中文名称
	Id string `json:"id,omitempty"`
	// 菜单组中文名称
	Text string `json:"text,omitempty"`
	// 菜单组英文名称
	TextEn string `json:"textEn"`
	/** 菜单组所在列序号 */
	// +optional
	Column uint32 `json:"column,omitempty"`
	// 菜单在当前组中的排序，数字越小越靠前
	// +optional
	RankingInColumn uint32 `json:"rankingInColumn,omitempty"`
	// 菜单图标
	// +optional
	Icon string `json:"icon,omitempty"`

	// 给替换菜单的返回按钮使用，当新的 pathname 是替换菜单，且替换菜单的返回按钮需要返回到当前 pathname 时，配置此属性；
	// 其值得为新的 pathname，同时需要注意⚠️，如果新的地址有多个，则应该取多个地址的公共部分，例如，/oidc/management/projects/:id/role
	// 和 /oidc/management/projects/:id/member 都需要支持，则应配置为/oidc/management/projects/:id
	// +optional
	ReplaceSiderBackNextPathnamePattern string `json:"replaceSiderBackNextPathnamePattern,omitempty"`
	// 菜单路由
	// +optional
	Pathname string `json:"pathname,omitempty"`
	//跳转菜单路由，优先级高于 pathname，指定后点击菜单会跳转到 redirect 相应路由
	// +optional
	Redirect string `json:"redirect,omitempty"`
	// 同 a 标签的 target 属性
	// +optional
	Target string `json:"target,omitempty"`
	// 菜单可见需要的角色
	// +optional
	RequiredRoles []string `json:"requiredRoles,omitempty"`
	//菜单可对应的 module 二进制位 (有一个满足即可见)
	// +optional
	RequiredModuleBits []int32 `json:"requiredModuleBits,omitempty"`
	//菜单对应路由是否可以切换租户
	// +optional
	Tenant bool `json:"tenant,omitempty"`
	// 菜单对应路由是否可以切换项目
	// +optional
	Project bool `json:"project,omitempty"`
	//菜单对应路由是否可以切换集群
	// +optional
	Cluster bool `json:"cluster,omitempty"`
	// 是否渲染选择项目、集群
	// +optional
	IsRenderSelectCurrent bool `json:"isRenderSelectCurrent,omitempty"`
	// 是否在进入子页面后将 sider 替换
	// +optional
	UseChildrenReplaceSider bool `json:"useChildrenReplaceSider,omitempty"`
	// 获取 title 的函数
	// +optional
	GetTitleForReplaceSider GetTitleForReplaceSider `json:"getTitleForReplaceSider,omitempty"`
	// 父菜单 ID
	// +optional
	Parent string `json:"parent,omitempty"`
	//对应 k8s 资源的 name
	// +optional
	ParentOwnerReferences metav1.OwnerReference `json:"parentOwnerReferences,omitempty"`
	// menu 显示控制
	// +optional
	Disabled bool `json:"disabled,omitempty"`
}

type GetTitleForReplaceSider struct {
	// 方法
	Method string `json:"method,omitempty"`
	// 参数
	Params string `json:"params,omitempty"`
	// 获取数据的路径
	ResponseDataPath []string `json:"responseDataPath,omitempty"`
}

// MenuStatus defines the observed state of Menu
type MenuStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// Menu is the Schema for the menus API
type Menu struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MenuSpec   `json:"spec,omitempty"`
	Status MenuStatus `json:"status,omitempty"`
}

type MenuReference struct {
	// Name of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
}

//+kubebuilder:object:root=true

// MenuList contains a list of Menu
type MenuList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Menu `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Menu{}, &MenuList{})
}
