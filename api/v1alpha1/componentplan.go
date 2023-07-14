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
	ComponentPlanTypeInstalled ConditionType = "Installed"
)

// Condition resons for ComponentPlan
const (
	ComponentPlanReasonWaitInstall    ConditionReason = "WaitInstall"
	ComponentPlanReasonInstalling     ConditionReason = "Installing"
	ComponentPlanReasonInstallSuccess ConditionReason = "InstallSuccess"
	ComponentPlanReasonInstallFailed  ConditionReason = "InstallFailed"
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

func ComponentPlanUnSucceeded(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeSucceeded, "", corev1.ConditionFalse, err)
}

func ComponentPlanAppreoved() Condition {
	return componentPlanCondition(ComponentPlanTypeApproved, "", corev1.ConditionTrue, nil)
}

func ComponentPlanUnAppreoved() Condition {
	return componentPlanCondition(ComponentPlanTypeApproved, "", corev1.ConditionFalse, nil)
}

func ComponentPlanInstallSuccess() Condition {
	return componentPlanCondition(ComponentPlanTypeInstalled, "", corev1.ConditionTrue, nil)
}

func ComponentPlanInstallFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeInstalled, ComponentPlanReasonInstallFailed, corev1.ConditionFalse, err)
}

func ComponentPlanInstalling() Condition {
	return componentPlanCondition(ComponentPlanTypeInstalled, ComponentPlanReasonInstalling, corev1.ConditionFalse, nil)
}

func ComponentPlanWaitInstall() Condition {
	return componentPlanCondition(ComponentPlanTypeInstalled, ComponentPlanReasonWaitInstall, corev1.ConditionFalse, nil)
}

func componentPlanCondition(ct ConditionType, reason ConditionReason, status corev1.ConditionStatus, err error) Condition {
	if status == "" {
		status = corev1.ConditionUnknown
	}
	switch ct {
	case ComponentPlanTypeSucceeded:
	case ComponentPlanTypeApproved:
	case ComponentPlanTypeInstalled:
		switch reason {
		case ComponentPlanReasonInstallSuccess:
			status = corev1.ConditionTrue
		case ComponentPlanReasonInstallFailed, ComponentPlanReasonWaitInstall, ComponentPlanReasonInstalling:
			status = corev1.ConditionFalse
		}
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

func (c *ComponentPlan) GetReleaseName() string {
	return c.Spec.Name
}
