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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ComponentPlanKey                    = Group + "/componentplan"
	ComponentPlanConfigMapRetryLabelKey = "retry"
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

// AddComponentPlanLabel add label against an unstructured object and derived resource.
// inspire by https://github.com/argoproj/argo-cd/blob/50b2f03657026a0987e4910eca4778e8950e6d87/util/kube/kube.go#L20
func AddComponentPlanLabel(target *unstructured.Unstructured, planName string) error {
	// Do not use target.GetLabels(), https://github.com/argoproj/argo-cd/issues/13730
	labels, _, err := unstructured.NestedStringMap(target.Object, "metadata", "labels")
	if err != nil {
		return err
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[ComponentPlanKey] = planName
	target.SetLabels(labels)

	gvk := schema.FromAPIVersionAndKind(target.GetAPIVersion(), target.GetKind())
	// special case for deployment and job types: make sure that derived replicaset, and pod has
	// the application label
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
		case "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet":
			templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
			if !ok || templateLabels == nil {
				templateLabels = make(map[string]interface{})
			}
			templateLabels[ComponentPlanKey] = planName
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
			// The following is a workaround for issue #335. In API version extensions/v1beta1 or
			// apps/v1beta1, if a spec omits spec.selector then k8s will default the
			// spec.selector.matchLabels to match spec.template.metadata.labels. This means Argo CD
			// labels can potentially make their way into spec.selector.matchLabels, which is a bad
			// thing. The following logic prevents this behavior.
			switch target.GetAPIVersion() {
			case "apps/v1beta1", "extensions/v1beta1":
				selector, _, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "selector")
				if err != nil {
					return err
				}
				if len(selector) == 0 {
					// If we get here, user did not set spec.selector in their manifest. We do not want
					// our Argo CD labels to get defaulted by kubernetes, so we explicitly set the labels
					// for them (minus the Argo CD labels).
					delete(templateLabels, ComponentPlanKey)
					err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "selector", "matchLabels")
					if err != nil {
						return err
					}
				}
			}
		}
	case "batch":
		switch gvk.Kind {
		case "Job":
			templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
			if !ok || templateLabels == nil {
				templateLabels = make(map[string]interface{})
			}
			templateLabels[ComponentPlanKey] = planName
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
		}
	}
	return nil
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
