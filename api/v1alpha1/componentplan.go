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
	"context"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubebb/core/pkg/utils"
)

const (
	ComponentPlanReleaseNameLabel     = Group + "/componentplan-release"
	ComponentPlanRetryTimesAnnotation = Group + "/componentplan-retry"
	ComponentPlanRollBackLabel        = Group + "/rollback"
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
	ComponentPlanReasonUninstalling     ConditionReason = "Uninstalling"
	ComponentPlanReasonRollingBack      ConditionReason = "RollingBack"
	ComponentPlanReasonInstallSuccess   ConditionReason = "InstallSuccess"
	ComponentPlanReasonInstallFailed    ConditionReason = "InstallFailed"
	ComponentPlanReasonUninstallSuccess ConditionReason = "UninstallSuccess"
	ComponentPlanReasonUninstallFailed  ConditionReason = "UninstallFailed"
	ComponentPlanReasonUpgradeSuccess   ConditionReason = "UpgradeSuccess"
	ComponentPlanReasonUpgradeFailed    ConditionReason = "UpgradeFailed"
	ComponentPlanReasonRollBackSuccess  ConditionReason = "RollBackSuccess"
	ComponentPlanReasonRollBackFailed   ConditionReason = "RollBackFailed"
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

func ComponentPlanInitSucceeded() Condition {
	return componentPlanCondition(ComponentPlanTypeSucceeded, "", corev1.ConditionFalse, nil)
}

func ComponentPlanFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeSucceeded, "", corev1.ConditionFalse, err)
}

func ComponentPlanApproved() Condition {
	return componentPlanCondition(ComponentPlanTypeApproved, "", corev1.ConditionTrue, nil)
}

func ComponentPlanUnapproved() Condition {
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

func ComponentPlanUninstallSuccess() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUninstallSuccess, corev1.ConditionTrue, nil)
}

func ComponentPlanUninstallFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUninstallFailed, corev1.ConditionFalse, err)
}

func ComponentPlanUninstalling() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonUninstalling, corev1.ConditionFalse, nil)
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

func ComponentPlanRollBackSuccess() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonRollBackSuccess, corev1.ConditionTrue, nil)
}

func ComponentPlanRollBackFailed(err error) Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonRollBackFailed, corev1.ConditionFalse, err)
}

func ComponentPlanRollingBack() Condition {
	return componentPlanCondition(ComponentPlanTypeActioned, ComponentPlanReasonRollingBack, corev1.ConditionFalse, nil)
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
		return []Condition{ComponentPlanApproved(), ComponentPlanWaitDo(nil), ComponentPlanInitSucceeded()}
	} else {
		return []Condition{ComponentPlanUnapproved(), ComponentPlanWaitDo(nil), ComponentPlanInitSucceeded()}
	}
}

func (c *ComponentPlan) IsActionedReason(cr ConditionReason) bool {
	return c.Status.GetCondition(ComponentPlanTypeActioned).Reason == cr
}

func (c *ComponentPlan) GetReleaseName() string {
	return c.Spec.Name
}

// ComponentPlanDiffIgnorePaths is the list of paths to ignore when comparing
// These fields will almost certainly change when componentplan is updated, and displaying these
// changes will only result in more invalid information, so they need to be ignored
var ComponentPlanDiffIgnorePaths = []string{
	"metadata.generation",
	"metadata.resourceVersion",
	"metadata.labels.helm.sh/chart",
	"spec.template.metadata.labels.helm.sh/chart",
}

// GetResourcesAndImages get resource slices, image lists from manifests
func GetResourcesAndImages(ctx context.Context, logger logr.Logger, c client.Client, data, namespace string) (resources []Resource, images []string, err error) {
	manifests, err := utils.SplitYAML([]byte(data))
	if err != nil {
		return nil, nil, err
	}
	resources = make([]Resource, len(manifests))
	for i, manifest := range manifests {
		obj := manifest
		if len(obj.GetNamespace()) == 0 {
			if rs, err := c.RESTMapper().RESTMapping(obj.GroupVersionKind().GroupKind()); err != nil {
				logger.Error(err, "get RESTMapping err, just ignore and continue", "obj", klog.KObj(obj))
				continue
			} else {
				if rs.Scope.Name() == meta.RESTScopeNameNamespace {
					obj.SetNamespace(namespace)
				} else {
					obj.SetNamespace("")
				}
			}
		}
		has := &unstructured.Unstructured{}
		has.SetKind(obj.GetKind())
		has.SetAPIVersion(obj.GetAPIVersion())
		err = c.Get(ctx, client.ObjectKeyFromObject(obj), has)
		var isNew bool
		if err != nil && apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			isNew = true
		} else if err != nil {
			logger.Error(err, "Resource get error, no notFound", "manifest", manifest, "obj", klog.KObj(obj))
			return nil, nil, err
		}
		r := Resource{
			Kind:       obj.GetKind(),
			Name:       obj.GetName(),
			APIVersion: obj.GetAPIVersion(),
		}
		if isNew {
			r.NewCreated = &isNew
		} else {
			diff, err := utils.ResourceDiffStr(ctx, obj, has, ComponentPlanDiffIgnorePaths, c)
			switch {
			case err != nil:
				logger.Error(err, "failed to get diff", "obj", klog.KObj(obj))
				diffMsg := "diff with exist"
				r.SpecDiffwithExist = &diffMsg
			case diff == "":
				ignore := "no spec diff, but some fields like resourceVersion will update"
				r.SpecDiffwithExist = &ignore
			default:
				r.SpecDiffwithExist = &diff
			}
		}
		resources[i] = r
		gvk := obj.GroupVersionKind()
		switch gvk.Group {
		case "":
			switch gvk.Kind { // nolint
			case "Pod":
				images = append(images, utils.GetPodImage(obj)...)
			}
		case "apps":
			switch gvk.Kind {
			case "Deployment":
				images = append(images, utils.GetDeploymentImage(obj)...)
			case "StatefulSet":
				images = append(images, utils.GetStatefulSetImage(obj)...)
			}
		case "batch":
			switch gvk.Kind {
			case "Job":
				images = append(images, utils.GetJobImage(obj)...)
			case "CronJob":
				images = append(images, utils.GetCronJobImage(obj)...)
			}
		}
	}
	imageMap := make(map[string]bool)
	for _, i := range images {
		imageMap[i] = true
	}
	images = make([]string, 0, len(imageMap))
	for k := range imageMap {
		images = append(images, k)
	}
	sort.Strings(images)
	return resources, images, nil
}
