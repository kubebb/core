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

const (
	ComponentPlanReleaseNameLabel     = Group + "/componentplan-release"
	ComponentPlanRetryTimesAnnotation = Group + "/componentplan-retry"
)

// ConditionType for ComponentPlan
const (
	ComponentPlanTypeSucceeded ConditionType = "Succeeded"
	ComponentPlanTypeApproved  ConditionType = "Approved"
	ComponentPlanTypeActioned  ConditionType = "Actioned"
)

// Condition resons for ComponentPlan
const (
	ComponentPlanReasonWaitDo           ConditionReason = "WaitDo"
	ComponentPlanReasonInstalling       ConditionReason = "Installing"
	ComponentPlanReasonUpgrading        ConditionReason = "Upgrading"
	ComponentPlanReasonUnInstalling     ConditionReason = "UnInstalling"
	ComponentPlanReasonInstallSuccess   ConditionReason = "InstallSuccess"
	ComponentPlanReasonInstallFailed    ConditionReason = "InstallFailed"
	ComponentPlanReasonUnInstallSuccess ConditionReason = "UnInstallSuccess"
	ComponentPlanReasonUnInstallFailed  ConditionReason = "UnInstallFailed"
	ComponentPlanReasonUpgradeSuccess   ConditionReason = "UpgradeSuccess"
	ComponentPlanReasonUpgradeFailed    ConditionReason = "UpgradeFailed"
)

// GenerateComponentPlanName generates the name of the component plan for a given subscription
func GenerateComponentPlanName(sub *Subscription, version string) string {
	return "sub." + sub.Name + "." + version
}

// GenerateComponentPlanManifestConfigMapName generates the name of the configmap of the component plan
func GenerateComponentPlanManifestConfigMapName(plan *ComponentPlan) string {
	return "manifest." + plan.Name
}

func ComponentPlanSucceeded() Condition {
	return componentPlanCondition(ComponentPlanTypeSucceeded, "", corev1.ConditionTrue, nil)
}

func ComponentPlanInitSucceded() Condition {
	return componentPlanCondition(ComponentPlanTypeSucceeded, "", corev1.ConditionFalse, nil)
}

func ComponentPlanUnSucceeded(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeSucceeded, "", corev1.ConditionFalse, err)
}

func ComponentPlanApproved() Condition {
	return componentPlanCondition(ComponentPlanTypeApproved, "", corev1.ConditionTrue, nil)
}

func ComponentPlanUnAppreoved() Condition {
	return componentPlanCondition(ComponentPlanTypeApproved, "", corev1.ConditionFalse, nil)
}

func ComponentPlanInstallSuccess() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonInstallSuccess, corev1.ConditionTrue, nil)
}

func ComponentPlanInstallFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonInstallFailed, corev1.ConditionFalse, err)
}

func ComponentPlanInstalling() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonInstalling, corev1.ConditionFalse, nil)
}

func ComponentPlanUnInstallSuccess() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUnInstallSuccess, corev1.ConditionTrue, nil)
}

func ComponentPlanUnInstallFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUnInstallFailed, corev1.ConditionFalse, err)
}

func ComponentPlanUnInstalling() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUnInstalling, corev1.ConditionFalse, nil)
}

func ComponentPlanUpgradeSuccess() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUpgradeSuccess, corev1.ConditionTrue, nil)
}

func ComponentPlanUpgradeFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUpgradeFailed, corev1.ConditionFalse, err)
}

func ComponentPlanUpgrading() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUpgrading, corev1.ConditionFalse, nil)
}
func ComponentPlanWaitDo(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonWaitDo, corev1.ConditionFalse, err)
}

func componentPlanCondition(ct ConditionType, reason ConditionReason, status corev1.ConditionStatus, err error) Condition {
	if status == "" {
		status = corev1.ConditionUnknown
	}
	c := Condition{
		Type:               ct,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
	}
	if err != nil {
		c.Message = err.Error()
	}
	return c
}

func (c *ComponentPlan) InitCondition() []Condition {
	if c.Spec.Approved {
		return []Condition{ComponentPlanApproved(), ComponentPlanWaitDo(nil), ComponentPlanInitSucceded()}
	} else {
		return []Condition{ComponentPlanUnAppreoved(), ComponentPlanWaitDo(nil), ComponentPlanInitSucceded()}
	}
}

func (c *ComponentPlan) InActionedReason(cr ConditionReason) bool {
	return c.Status.GetCondition(ComponentPlanTypeActioned).Reason == cr
}

func (c *ComponentPlan) GetReleaseName() string {
	return c.Spec.Name
}
