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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
	"github.com/kubebb/core/pkg/utils"
)

var (
	errMaxRetry = errors.New("max retry reached, will not retry")
)

const (
	revisionNoExist = -1
	revisionInstall = 1
	waitLonger      = time.Minute
	waitSmaller     = time.Second * 3
)

// ComponentPlanReconciler reconciles a ComponentPlan object
type ComponentPlanReconciler struct {
	client.Client
	Recorder   record.EventRecorder
	Scheme     *runtime.Scheme
	WorkerPool helm.ReleaseWorkerPool
}

// +kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=componentplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=componentplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=componentplans/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ComponentPlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling ComponentPlan")

	// Fetch the ComponentPlan instance
	plan := &corev1alpha1.ComponentPlan{}
	err := r.Get(ctx, req.NamespacedName, plan)
	if err != nil {
		// There's no need to requeue if the resource no longer exists.
		// Otherwise, we'll be requeued implicitly because we return an error.
		logger.V(1).Info("Failed to get ComponentPlan")
		return reconcile.Result{}, utils.IgnoreNotFound(err)
	}
	logger = logger.WithValues("Generation", plan.GetGeneration(), "ObservedGeneration", plan.Status.ObservedGeneration, "creator", plan.Spec.Creator)
	logger.V(1).Info("Get ComponentPlan instance")

	// Add a finalizer. Then, we can define some operations which should
	// occur before the ComponentPlan to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if newAdded := controllerutil.AddFinalizer(plan, corev1alpha1.Finalizer); newAdded {
		logger.Info("Try to add Finalizer for ComponentPlan")
		if err = r.Update(ctx, plan); err != nil {
			logger.Error(err, "Failed to update ComponentPlan to add finalizer, will try again later")
			return ctrl.Result{}, err
		}
		logger.Info("Adding Finalizer for ComponentPlan done")
		return ctrl.Result{}, nil
	}

	// Check if the ComponentPlan instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPlanMarkedToBeDeleted := plan.GetDeletionTimestamp() != nil
	if isPlanMarkedToBeDeleted && controllerutil.ContainsFinalizer(plan, corev1alpha1.Finalizer) {
		logger.Info("Performing Finalizer Operations for Plan before delete CR")
		// Note: In the original Helm source code, when helm is uninstalling, there is no check to
		// see if the current helm release is in installing or any other state, just uninstall.
		if plan.IsActionedReason(corev1alpha1.ComponentPlanReasonUninstallFailed) {
			logger.Info("the last one is uninstall failed, just skip this one")
			return ctrl.Result{}, nil
		}
		if plan.IsActionedReason(corev1alpha1.ComponentPlanReasonUninstallSuccess) {
			logger.Info("Removing Finalizer for ComponentPlan after successfully performing the operations")
			controllerutil.RemoveFinalizer(plan, corev1alpha1.Finalizer)
			if err = r.Update(ctx, plan); err != nil {
				logger.Error(err, "Failed to remove finalizer for ComponentPlan")
				return ctrl.Result{}, err
			}
			logger.Info("Remove ComponentPlan done")
			return ctrl.Result{}, nil
		}
		doing, err := r.WorkerPool.Uninstall(ctx, plan)
		if doing {
			return ctrl.Result{RequeueAfter: waitSmaller}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanUninstalling())
		}
		if err != nil {
			logger.Error(err, "Failed to uninstall ComponentPlan")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, true, true, corev1alpha1.ComponentPlanUninstallFailed(err))
		}
		logger.Info("Uninstall ComponentPlan succeeded")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, true, false, corev1alpha1.ComponentPlanUninstallSuccess())
	}

	if plan.Spec.ComponentRef == nil || plan.Spec.ComponentRef.Namespace == "" || plan.Spec.ComponentRef.Name == "" {
		logger.Info("Failed to get Componentplan's Component ref, stop")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)

	// Get related component
	component := &corev1alpha1.Component{}
	err = r.Get(ctx, types.NamespacedName{Namespace: plan.Spec.ComponentRef.Namespace, Name: plan.Spec.ComponentRef.Name}, component)
	if err != nil {
		msg := fmt.Sprintf("Failed to get Component, wait %s for Component to be found", waitLonger)
		if apierrors.IsNotFound(err) {
			logger.Info(msg, "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		} else {
			logger.Error(err, msg, "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		}
		return reconcile.Result{RequeueAfter: waitLonger}, utils.IgnoreNotFound(err)
	}
	if component.Status.RepositoryRef == nil || component.Status.RepositoryRef.Name == "" || component.Status.RepositoryRef.Namespace == "" {
		logger.Info(fmt.Sprintf("Failed to get Component.Status.RepositoryRef, wait %s to retry", waitLonger), "obj", klog.KObj(component))
		return reconcile.Result{RequeueAfter: waitLonger}, nil
	}
	logger.V(1).Info("Get Component instance", "Component", klog.KObj(component))

	if plan.Labels[corev1alpha1.ComponentPlanReleaseNameLabel] != plan.Spec.Name {
		if plan.GetLabels() == nil {
			plan.Labels = make(map[string]string)
		}
		plan.Labels[corev1alpha1.ComponentPlanReleaseNameLabel] = plan.Spec.Name
		err = r.Update(ctx, plan)
		if err != nil {
			logger.Error(err, "Failed to update ComponentPlan release label")
		}
		return ctrl.Result{}, err
	}

	// Set the status as Unknown when no status are available.
	if len(plan.Status.Conditions) == 0 {
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, plan.InitCondition()...)
	}

	// updateLatest try to update all componentplan's status.Latest
	go r.updateLatest(ctx, logger, plan)

	if plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeSucceeded).Status == corev1.ConditionTrue {
		switch {
		case r.isGenerationUpdate(plan):
			logger.Info("ComponentPlan.Spec is changed, need to install or upgrade ...")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, plan.InitCondition()...)
		case r.needRetry(plan):
			logger.Info("ComponentPlan need to retry...")
		default:
			logger.Info("ComponentPlan is unchanged and has been successful, no need to reconcile")
			return ctrl.Result{}, nil
		}
	}

	repo := &corev1alpha1.Repository{}
	if err = r.Get(ctx, types.NamespacedName{Name: component.Status.RepositoryRef.Name, Namespace: component.Status.RepositoryRef.Namespace}, repo); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get Repository, wait %s for another try", waitSmaller))
		return ctrl.Result{RequeueAfter: waitSmaller}, nil
	}

	chartName := repo.NamespacedName() + "/" + component.Status.Name
	if chartName == "" {
		logger.Info(fmt.Sprintf("Failed to get Component's chart name in status.name, wait %s for another try", waitSmaller), "Component", klog.KObj(component))
		return ctrl.Result{Requeue: true, RequeueAfter: waitSmaller}, nil
	}
	if repo.IsOCI() {
		if pullURL := component.Annotations[corev1alpha1.OCIPullURLAnnotation]; pullURL != "" {
			chartName = pullURL
		} else {
			chartName = repo.Spec.URL
		}
	}

	// Check its helm template configmap exist
	manifest := &corev1.ConfigMap{}
	manifest.Name = corev1alpha1.GenerateComponentPlanManifestConfigMapName(plan)
	manifest.Namespace = plan.Namespace
	err = r.Get(ctx, types.NamespacedName{Name: manifest.Name, Namespace: manifest.Namespace}, manifest)
	if (err != nil && apierrors.IsNotFound(err)) || r.isGenerationUpdate(plan) {
		data, err := r.WorkerPool.GetManifests(ctx, plan, repo, chartName)
		if err != nil {
			logger.Error(err, "Failed to get manifest")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
		}

		if err = r.GenerateManifestConfigMap(plan, manifest, data); err != nil {
			logger.Error(err, "Failed to generate manifest configmap")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
		}
		logger.Info("Generate a new template Configmap", "ConfigMap", klog.KObj(manifest))

		newPlan := plan.DeepCopy()
		newPlan.Status.Resources, newPlan.Status.Images, err = corev1alpha1.GetResourcesAndImages(ctx, logger, r.Client, data, plan.GetNamespace())
		if err != nil {
			logger.Error(err, "Failed to get resources")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
		}
		err = r.Status().Patch(ctx, newPlan, client.MergeFrom(plan))
		if err != nil {
			logger.Error(err, "Failed to update ComponentPlan status.Resources")
		}
		logger.Info("Update ComponentPlan status.Resources")

		res, err := controllerutil.CreateOrUpdate(ctx, r.Client, manifest, func() error {
			return nil
		})
		if err != nil {
			logger.Error(err, "Failed to create or update template Configmap", "ConfigMap", klog.KObj(manifest))
			r.Recorder.Eventf(plan, corev1.EventTypeWarning, "Fail", "failed to create or update template configmap %s with error %s", manifest.GetName(), err)

			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
		}
		r.Recorder.Eventf(plan, corev1.EventTypeNormal, "Success", "configmap %s created successfully", manifest.GetName())
		logger.Info(fmt.Sprintf("Reconcile ComponentPlan template Configmap, result:%s", res))
	} else if err != nil {
		logger.Error(err, "Failed to get template Configmap")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
	}

	// Install the component plan
	if !plan.Spec.Approved {
		logger.Info("component plan isn't approved, skip install or upgrade...")
		return ctrl.Result{}, nil
	}
	if r.needRollBack(plan) {
		logger.Info("find rollback label, will try to RollBack...", "rollbackRevision", plan.Status.InstalledRevision)
		if plan.IsActionedReason(corev1alpha1.ComponentPlanReasonRollBackFailed) {
			logger.Info("the last one is RollBack failed, just skip this one")
			return ctrl.Result{}, nil
		}
		if plan.IsActionedReason(corev1alpha1.ComponentPlanReasonRollBackSuccess) {
			r.removeRollBackLabel(plan)
			if err = r.Update(ctx, plan); err != nil {
				logger.Error(err, "Failed to remove RollBack label for ComponentPlan")
				return ctrl.Result{}, err
			}
			logger.Info("Remove RollBack label done")
			return ctrl.Result{}, nil
		}
		rel, doing, err := r.WorkerPool.RollBack(ctx, plan)
		if doing {
			return ctrl.Result{RequeueAfter: waitSmaller}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanRollingBack())
		}
		if err != nil {
			logger.Error(err, "Failed to RollBack ComponentPlan")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, true, true, corev1alpha1.ComponentPlanRollBackFailed(err))
		}
		logger.Info("RollBack ComponentPlan succeeded")
		revision := revisionNoExist
		if rel != nil {
			revision = rel.Version
		}
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revision, true, false, corev1alpha1.ComponentPlanRollBackSuccess())
	}
	if r.statusShowDone(plan) {
		return ctrl.Result{}, nil
	}
	if !r.needRetry(plan) {
		logger.Info(errMaxRetry.Error())
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, true, false, corev1alpha1.ComponentPlanFailed(errMaxRetry))
	}

	rel, err := r.WorkerPool.GetLastRelease(plan)
	if err != nil {
		logger.Error(err, "Failed to check if helm is doing")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
	}
	if rel == nil {
		logger.Info("no release found will install the component plan")
		_ = r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanInstalling())
	} else if rel.Info != nil {
		if rel.Info.Status.IsPending() || rel.Info.Status == release.StatusUninstalling {
			logger.Info(fmt.Sprintf("helm show another operation (install/upgrade/rollback/uninstall) is in progress...wait %s for another try", waitSmaller))
			return ctrl.Result{RequeueAfter: waitSmaller}, nil
		}
		_, _, uid, generation, _ := helm.ParseDescription(rel.Info.Description)
		if uid == string(plan.GetUID()) && generation == plan.GetGeneration() && rel.Info.Status == release.StatusDeployed {
			return ctrl.Result{}, r.updateReleaseStatus(ctx, logger, rel, nil, plan)
		}

		logger.Info(fmt.Sprintf("helm find release version:%d, will upgrade", rel.Version), helm.ReleaseLog(rel)...)
		_ = r.PatchCondition(ctx, plan, logger, rel.Version, false, false, corev1alpha1.ComponentPlanUpgrading())
	}

	rel, doing, err := r.WorkerPool.InstallOrUpgrade(ctx, plan, repo, chartName)
	if doing {
		logger.Info(fmt.Sprintf("another operation (install/upgrade/rollback/uninstall) is in progress...wait %s for another try", waitSmaller))
		return ctrl.Result{RequeueAfter: waitSmaller}, nil
	}
	return ctrl.Result{}, r.updateReleaseStatus(ctx, logger, rel, err, plan)
}
func (r *ComponentPlanReconciler) updateReleaseStatus(ctx context.Context, logger logr.Logger, rel *release.Release, inputErr error, plan *corev1alpha1.ComponentPlan) (err error) {
	revision := revisionNoExist
	if rel != nil {
		revision = rel.Version
	}
	if inputErr != nil {
		if revision == revisionNoExist || revision == revisionInstall {
			logger.Error(inputErr, "componentplan install failed")
			err = r.PatchCondition(ctx, plan, logger, revision, false, true, corev1alpha1.ComponentPlanInstallFailed(inputErr))
			r.Recorder.Eventf(plan, corev1.EventTypeWarning, "InstallationFailure", "%s install failed", plan.GetReleaseName())
		} else {
			logger.Error(inputErr, "componentplan upgrade failed")
			err = r.PatchCondition(ctx, plan, logger, revision, false, true, corev1alpha1.ComponentPlanUpgradeFailed(inputErr))
			r.Recorder.Eventf(plan, corev1.EventTypeWarning, "UpgradeFailure", "%s upgrade failed", plan.GetReleaseName())
		}
	} else {
		if revision == revisionInstall {
			logger.Info("componentplan install successfully", helm.ReleaseLog(rel)...)
			err = r.PatchCondition(ctx, plan, logger, revision, true, false, corev1alpha1.ComponentPlanInstallSuccess())
			r.Recorder.Eventf(plan, corev1.EventTypeNormal, "InstallationSuccess", "%s install successfully", rel.Name)
		} else {
			logger.Info("componentplan upgrade successfully", helm.ReleaseLog(rel)...)
			err = r.PatchCondition(ctx, plan, logger, revision, true, false, corev1alpha1.ComponentPlanUpgradeSuccess())
			r.Recorder.Eventf(plan, corev1.EventTypeNormal, "UpgradeSuccess", "%s upgrade successfully", rel.Name)
		}
	}
	return
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.ComponentPlan{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *ComponentPlanReconciler) GenerateManifestConfigMap(plan *corev1alpha1.ComponentPlan, manifest *corev1.ConfigMap, data string) (err error) {
	if manifest.Labels == nil {
		manifest.Labels = make(map[string]string)
	}
	manifest.Data = make(map[string]string)
	manifest.Data["manifest"] = data
	return controllerutil.SetOwnerReference(plan, manifest, r.Scheme)
}

// PatchCondition patch subscription status condition
func (r *ComponentPlanReconciler) PatchCondition(ctx context.Context, plan *corev1alpha1.ComponentPlan, logger logr.Logger, revision int, isDone, isFailed bool, condition ...corev1alpha1.Condition) (err error) {
	annotation := r.updateStatusRetryTimes(plan, isFailed)
	if len(annotation) > 0 {
		if err := r.Get(ctx, client.ObjectKeyFromObject(plan), plan); err != nil {
			logger.Error(err, "Failed to Prare Update ComponentPlan")
			return err
		}
		updated := plan.DeepCopy()
		updated.SetAnnotations(annotation)
		if err := r.Patch(ctx, updated, client.MergeFrom(plan)); err != nil {
			logger.Error(err, "Failed to Update ComponentPlan")
			return err
		}
	}
	newPlan := r.setCondition(plan, condition...)
	newPlan = r.updateStatusRevision(newPlan, revision)
	newPlan = r.updateStatusObservedGeneration(newPlan, isDone)
	if err := r.Status().Patch(ctx, newPlan, client.MergeFrom(plan)); err != nil {
		logger.Error(err, "Failed to patch ComponentPlan status")
		return err
	}
	return nil
}

// PatchCondition patch subscription status condition
func (r *ComponentPlanReconciler) setCondition(plan *corev1alpha1.ComponentPlan, condition ...corev1alpha1.Condition) (newPlan *corev1alpha1.ComponentPlan) {
	newPlan = plan.DeepCopy()
	newPlan.Status.SetConditions(condition...)
	ready := len(newPlan.Status.Conditions) > 0
	for _, cond := range newPlan.Status.Conditions {
		if cond.Type == corev1alpha1.ComponentPlanTypeSucceeded {
			continue
		}
		if cond.Status != corev1.ConditionTrue {
			ready = false
			break
		}
	}
	if ready {
		newPlan.Status.SetConditions(corev1alpha1.ComponentPlanSucceeded())
	} else {
		newPlan.Status.SetConditions(corev1alpha1.ComponentPlanFailed(nil))
	}
	return newPlan
}

func (r *ComponentPlanReconciler) updateStatusRevision(plan *corev1alpha1.ComponentPlan, revision int) (newPlan *corev1alpha1.ComponentPlan) {
	if revision <= 0 || revision == plan.Status.InstalledRevision {
		return plan
	}
	newPlan = plan.DeepCopy()
	newPlan.Status.InstalledRevision = revision
	newPlan.Status.Latest = pointer.Bool(true)
	return newPlan
}

func (r *ComponentPlanReconciler) updateStatusRetryTimes(plan *corev1alpha1.ComponentPlan, isFailed bool) (annotation map[string]string) {
	if !isFailed || r.statusShowDone(plan) {
		return
	}
	annotation = plan.GetAnnotations()
	if annotation == nil {
		annotation = make(map[string]string)
	}
	hasRetry, _ := strconv.Atoi(annotation[corev1alpha1.ComponentPlanRetryTimesAnnotation])
	annotation[corev1alpha1.ComponentPlanRetryTimesAnnotation] = strconv.Itoa(hasRetry + 1)
	return
}

func (r *ComponentPlanReconciler) updateStatusObservedGeneration(plan *corev1alpha1.ComponentPlan, isDone bool) (newPlan *corev1alpha1.ComponentPlan) {
	if isDone || r.statusShowDone(plan) {
		plan.Status.ObservedGeneration = plan.GetGeneration()
	}
	return plan
}

func (r *ComponentPlanReconciler) needRetry(plan *corev1alpha1.ComponentPlan) bool {
	hasRetry, _ := strconv.Atoi(plan.GetAnnotations()[corev1alpha1.ComponentPlanRetryTimesAnnotation])
	return hasRetry < plan.Spec.Config.GetMaxRetry()
}

func (r *ComponentPlanReconciler) isGenerationUpdate(plan *corev1alpha1.ComponentPlan) bool {
	return plan.Status.ObservedGeneration != 0 && plan.Status.ObservedGeneration < plan.GetGeneration()
}

func (r *ComponentPlanReconciler) statusShowDone(plan *corev1alpha1.ComponentPlan) bool {
	installedCondition := plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeActioned)
	// if successed or max retry times reached, no need to update retry times
	if installedCondition.Status == corev1.ConditionTrue || installedCondition.Message == errMaxRetry.Error() {
		return true
	}
	return false
}

func (r *ComponentPlanReconciler) updateLatest(ctx context.Context, logger logr.Logger, cpl *corev1alpha1.ComponentPlan) {
	releaseName := cpl.GetReleaseName()
	list := &corev1alpha1.ComponentPlanList{}
	if err := r.List(ctx, list, client.MatchingLabels(map[string]string{corev1alpha1.ComponentPlanReleaseNameLabel: releaseName})); err != nil {
		logger.Error(err, "Failed to list ComponentPlan")
		return
	}
	rel, err := r.WorkerPool.GetLastRelease(cpl)
	if err != nil {
		logger.Error(err, "Failed to get last release")
		return
	}
	if rel == nil {
		logger.Info("no release found, just skip update latest")
		return
	}
	for _, cur := range list.Items {
		if latestShouldBe := cur.Status.InstalledRevision == rel.Version; cur.Status.Latest == nil || *cur.Status.Latest != latestShouldBe {
			newCur := cur.DeepCopy()
			newCur.Status.Latest = pointer.Bool(latestShouldBe)
			if err := r.Status().Patch(ctx, newCur, client.MergeFrom(&cur)); err != nil {
				logger.Error(err, "Failed to update ComponentPlan status.Latest", "obj", klog.KObj(&cur))
			}
			logger.Info("Update ComponentPlan status.Latest", "obj", klog.KObj(&cur))
		}
	}
}

func (r *ComponentPlanReconciler) needRollBack(plan *corev1alpha1.ComponentPlan) bool {
	if _, ok := plan.GetLabels()[corev1alpha1.ComponentPlanRollBackLabel]; ok {
		if plan.Status.InstalledRevision != 0 {
			return true
		}
	}
	return false
}

func (r *ComponentPlanReconciler) removeRollBackLabel(plan *corev1alpha1.ComponentPlan) {
	delete(plan.GetLabels(), corev1alpha1.ComponentPlanRollBackLabel)
}
