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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	maxRetryErr = errors.New("max retry reached, will not retry")
)

const (
	revisionNoExist = -1
)

// ComponentPlanReconciler reconciles a ComponentPlan object
type ComponentPlanReconciler struct {
	client.Client
	mu               sync.Mutex
	helmPending      sync.Map
	helmUninstalling sync.Map
	Recorder         record.EventRecorder
	Scheme           *runtime.Scheme
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
	logger = logger.WithValues("Generation", plan.GetGeneration(), "ObservedGeneration", plan.Status.ObservedGeneration)
	logger.V(1).Info("Get ComponentPlan instance")

	// Get watched component
	component := &corev1alpha1.Component{}
	err = r.Get(ctx, types.NamespacedName{Namespace: plan.Spec.ComponentRef.Namespace, Name: plan.Spec.ComponentRef.Name}, component)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Failed to get Component, wait 1 minute for Component to be found", "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		} else {
			logger.Error(err, "Failed to get Component, wait 1 minute for Component to be found", "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		}
		return reconcile.Result{RequeueAfter: time.Minute}, utils.IgnoreNotFound(err)
	}
	if component.Status.RepositoryRef == nil || component.Status.RepositoryRef.Name == "" || component.Status.RepositoryRef.Namespace == "" {
		logger.Info("Failed to get Component.Status.RepositoryRef, wait 1 minute to retry", "obj", klog.KObj(component))
		return reconcile.Result{RequeueAfter: time.Minute}, nil
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

	// Get RESTClientGetter for helm stuff
	getter, err := r.buildRESTClientGetter(plan.GetNamespace())
	if err != nil {
		logger.Error(err, "Failed to build RESTClientGetter")
		// if we return err, reconcile will retry immediately.
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// updateLatest try to update all componentplan's status.Latest
	go r.updateLatest(ctx, logger, getter, plan)

	// Check if the ComponentPlan instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPlanMarkedToBeDeleted := plan.GetDeletionTimestamp() != nil
	if isPlanMarkedToBeDeleted && controllerutil.ContainsFinalizer(plan, corev1alpha1.Finalizer) {
		logger.Info("Performing Finalizer Operations for Plan before delete CR")
		// Note: In the original Helm source code, when helm is uninstalling, there is no check to
		// see if the current helm release is in installing or any other state, just do uninstall.
		// We add this check here just for the controller doesn't duplicate the uninstallation logic to cause fail
		if plan.InActionedReason(corev1alpha1.ComponentPlanReasonUnInstalling) || r.inUninstallingMap(plan) {
			logger.Info("another operation (uninstall) is in progress... skip for 1 minute")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		if plan.InActionedReason(corev1alpha1.ComponentPlanReasonUnInstallFailed) {
			// TODO do we need uninstall retry like install or upgrade done?
			logger.Info("the last one is uninstallfailed, just skip this one")
			return ctrl.Result{}, nil
		}
		if plan.InActionedReason(corev1alpha1.ComponentPlanReasonUnInstallSuccess) {
			logger.Info("Removing Finalizer for ComponentPlan after successfully performing the operations")
			if ok := controllerutil.RemoveFinalizer(plan, corev1alpha1.Finalizer); !ok {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
			}

			if err = r.Update(ctx, plan); err != nil {
				logger.Error(err, "Failed to remove finalizer for ComponentPlan")
				return ctrl.Result{}, err
			}
			logger.Info("Remove ComponentPlan done")
			return ctrl.Result{}, nil
		}

		_ = r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanUnInstalling())
		// Perform all operations required before remove the finalizer and allow
		// the Kubernetes API to remove ComponentPlan
		go func() {
			r.mu.Lock()
			if r.inUninstallingMap(plan) {
				logger.Info("another uninstall running, skip")
				r.mu.Unlock()
				return
			}
			r.addToUninstallingMap(plan)
			r.mu.Unlock()
			err = r.doFinalizerOperationsForPlan(ctx, logger, getter, plan)
			r.removeFromUninstallingMap(plan)
			r.removeFromPendingMap(plan)
			if err != nil {
				logger.Error(err, "Failed to uninstall ComponentPlan")
				_ = r.PatchCondition(ctx, plan, logger, revisionNoExist, true, true, corev1alpha1.ComponentPlanUnInstallFailed(err))
			} else {
				logger.Info("Uninstall ComponentPlan succeeded")
				_ = r.PatchCondition(ctx, plan, logger, revisionNoExist, true, false, corev1alpha1.ComponentPlanUnInstallSuccess())
			}
		}()
		return ctrl.Result{}, nil
	}

	if plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeSucceeded).Status == corev1.ConditionTrue {
		if r.isGenerationUpdate(plan) {
			logger.Info("ComponentPlan.Spec is changed, need to install or upgrade...")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, plan.InitCondition()...)
		} else {
			logger.Info("ComponentPlan is unchanged and has been successful, no need to reconcile")
			return ctrl.Result{}, nil
		}
	}

	repo := &corev1alpha1.Repository{}
	if err = r.Get(ctx, types.NamespacedName{Name: component.Status.RepositoryRef.Name, Namespace: component.Status.RepositoryRef.Namespace}, repo); err != nil {
		logger.Error(err, "Failed to get Repository, wait 15 seconds for another try")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	chartName := component.Status.Name
	if chartName == "" {
		logger.Info("Failed to get Component's chart name in status.name, wait 15 seconds for another try", "Component", klog.KObj(component))
		return ctrl.Result{Requeue: true, RequeueAfter: 15 * time.Second}, nil
	}

	// Check its helm template configmap exist
	manifest := &corev1.ConfigMap{}
	manifest.Name = corev1alpha1.GenerateComponentPlanManifestConfigMapName(plan)
	manifest.Namespace = plan.Namespace
	err = r.Get(ctx, types.NamespacedName{Name: manifest.Name, Namespace: manifest.Namespace}, manifest)
	if (err != nil && apierrors.IsNotFound(err)) || r.isGenerationUpdate(plan) {
		data, err := helm.GetManifests(ctx, getter, r.Client, logger, plan, repo, chartName)
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
		newPlan.Status.Resources, newPlan.Status.Images, err = utils.GetResourcesAndImages(ctx, logger, r.Client, data, plan.GetNamespace())
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
	if r.statusShowDone(plan) {
		return ctrl.Result{}, nil
	}
	doing, rel, err := r.isHelmDoing(logger, getter, plan)
	if err != nil {
		logger.Error(err, "Failed to check if helm is doing")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, false, false, corev1alpha1.ComponentPlanWaitDo(err))
	}
	if doing {
		logger.Info("another operation (install/upgrade/rollback/uninstall) is in progress...skip")
		return ctrl.Result{}, nil
	}
	if !r.needRetry(plan) {
		logger.Info(maxRetryErr.Error())
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, revisionNoExist, true, false, corev1alpha1.ComponentPlanUnSucceeded(maxRetryErr))
	}

	revision := revisionNoExist
	if rel == nil {
		logger.Info("no release found, will install the component plan")
		_ = r.PatchCondition(ctx, plan, logger, revision, false, false, corev1alpha1.ComponentPlanInstalling())
	} else {
		revision = rel.Version
		logger.Info(fmt.Sprintf("in cluster find release version:%d, will upgrade", revision), helm.ReleaseLog(rel)...)
		_ = r.PatchCondition(ctx, plan, logger, revision, false, false, corev1alpha1.ComponentPlanUpgrading())
	}

	go func(revision int) {
		r.mu.Lock()
		if r.inPendingMap(plan, revision) {
			logger.Info("same revision is doing, skip", "revision", revision)
			r.mu.Unlock()
			return
		}
		r.addToPendingMap(plan, revision)
		r.mu.Unlock()
		logger.Info("will handle this revision", "revision", revision)
		rel, err := helm.InstallOrUpgrade(ctx, getter, r.Client, logger, plan, repo, chartName)
		if err != nil {
			r.removeFromPendingMap(plan)
			installedRevision := revisionNoExist
			if rel != nil {
				installedRevision = rel.Version
			}
			if revision == revisionNoExist {
				logger.Error(err, "componentplan install failed")
				_ = r.PatchCondition(ctx, plan, logger, installedRevision, false, true, corev1alpha1.ComponentPlanInstallFailed(err))
				r.Recorder.Eventf(plan, corev1.EventTypeWarning, "InstallationFailure", "%s install failed", plan.GetReleaseName())

			} else {
				logger.Error(err, "componentplan upgrade failed")
				_ = r.PatchCondition(ctx, plan, logger, installedRevision, false, true, corev1alpha1.ComponentPlanUpgradeFailed(err))
				r.Recorder.Eventf(plan, corev1.EventTypeWarning, "UpgradeFailure", "%s upgrade failed", plan.GetReleaseName())
			}
			return
		}
		r.addToPendingMap(plan, rel.Version)
		if revision == revisionNoExist {
			logger.Info("componentplan install successfully", helm.ReleaseLog(rel)...)
			_ = r.PatchCondition(ctx, plan, logger, rel.Version, true, false, corev1alpha1.ComponentPlanInstallSuccess())
			r.Recorder.Eventf(plan, corev1.EventTypeNormal, "InstallationSuccess", "%s install successfully", rel.Name)

		} else {
			logger.Info("componentplan upgrade successfully", helm.ReleaseLog(rel)...)
			_ = r.PatchCondition(ctx, plan, logger, rel.Version, true, false, corev1alpha1.ComponentPlanUpgradeSuccess())
			r.Recorder.Eventf(plan, corev1.EventTypeNormal, "UpgradeSuccess", "%s upgrade successfully", rel.Name)
		}
	}(revision)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.ComponentPlan{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

// doFinalizerOperationsForPlan performs all operations required before remove the finalizer
func (r *ComponentPlanReconciler) doFinalizerOperationsForPlan(ctx context.Context, logger logr.Logger, getter genericclioptions.RESTClientGetter, plan *corev1alpha1.ComponentPlan) (err error) {
	return helm.Uninstall(ctx, getter, logger, plan)
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
		newPlan.Status.SetConditions(corev1alpha1.ComponentPlanUnSucceeded(nil))
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

func (r *ComponentPlanReconciler) buildRESTClientGetter(ns string) (genericclioptions.RESTClientGetter, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config for in-cluster REST client: %w", err)
	}
	return &genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &ns,
	}, nil
}

func (r *ComponentPlanReconciler) isGenerationUpdate(plan *corev1alpha1.ComponentPlan) bool {
	return plan.Status.ObservedGeneration != 0 && plan.Status.ObservedGeneration < plan.GetGeneration()
}

func (r *ComponentPlanReconciler) statusShowDone(plan *corev1alpha1.ComponentPlan) bool {
	installedCondition := plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeActioned)
	// if successed or max retry times reached, no need to update retry times
	if installedCondition.Status == corev1.ConditionTrue || installedCondition.Message == maxRetryErr.Error() {
		return true
	}
	return false
}

func (r *ComponentPlanReconciler) isHelmDoing(logger logr.Logger, getter genericclioptions.RESTClientGetter, plan *corev1alpha1.ComponentPlan) (doing bool, rel *release.Release, err error) {
	if plan.InActionedReason(corev1alpha1.ComponentPlanReasonInstalling) || plan.InActionedReason(corev1alpha1.ComponentPlanReasonUpgrading) || plan.InActionedReason(corev1alpha1.ComponentPlanReasonUnInstalling) {
		logger.Info("plan status in pending")
		doing = true
		return
	}
	rel, err = helm.GetLastRelease(getter, logger, plan)
	if err == nil && rel != nil && rel.Info != nil && (rel.Info.Status.IsPending() || rel.Info.Status == release.StatusUninstalling) {
		logger.Info("helm release in pending")
		doing = true
		return
	}
	revision := revisionNoExist
	if rel != nil {
		revision = rel.Version
	}
	if r.inPendingMap(plan, revision) {
		logger.Info("plan in pending cache map")
		doing = true
	}
	return
}

func (r *ComponentPlanReconciler) updateLatest(ctx context.Context, logger logr.Logger, getter genericclioptions.RESTClientGetter, cpl *corev1alpha1.ComponentPlan) {
	releaseName := cpl.GetReleaseName()
	list := &corev1alpha1.ComponentPlanList{}
	if err := r.List(ctx, list, client.MatchingLabels(map[string]string{corev1alpha1.ComponentPlanReleaseNameLabel: releaseName})); err != nil {
		logger.Error(err, "Failed to list ComponentPlan")
		return
	}
	rel, err := helm.GetLastRelease(getter, logger, cpl)
	if err != nil {
		logger.Error(err, "Failed to get last release")
		return
	}
	if rel == nil {
		logger.Info("no release found, just skip")
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

func (r *ComponentPlanReconciler) cacheMapKey(plan *corev1alpha1.ComponentPlan) string {
	return plan.Namespace + "/" + plan.GetReleaseName()
}

func (r *ComponentPlanReconciler) inPendingMap(plan *corev1alpha1.ComponentPlan, watchedLastRevision int) bool {
	key := r.cacheMapKey(plan)
	val, exist := r.helmPending.Load(key)
	if !exist {
		return false
	}
	v, ok := val.(*pendingMapVal)
	if !ok {
		return false
	}
	if v.Name != plan.GetName() || plan.GetGeneration() > v.Generation || watchedLastRevision > v.Revision {
		return false
	}
	return true
}

type pendingMapVal struct {
	Name       string
	Revision   int
	Generation int64
}

func (r *ComponentPlanReconciler) addToPendingMap(plan *corev1alpha1.ComponentPlan, lastRevision int) {
	key := r.cacheMapKey(plan)
	r.helmPending.Store(key, &pendingMapVal{
		Name:       plan.GetName(),
		Revision:   lastRevision,
		Generation: plan.GetGeneration(),
	})
}

func (r *ComponentPlanReconciler) removeFromPendingMap(plan *corev1alpha1.ComponentPlan) {
	key := r.cacheMapKey(plan)
	r.helmPending.Delete(key)
}

func (r *ComponentPlanReconciler) inUninstallingMap(plan *corev1alpha1.ComponentPlan) bool {
	key := r.cacheMapKey(plan)
	_, exist := r.helmUninstalling.Load(key)
	return exist
}

func (r *ComponentPlanReconciler) addToUninstallingMap(plan *corev1alpha1.ComponentPlan) {
	key := r.cacheMapKey(plan)
	r.helmUninstalling.Store(key, true)
}

func (r *ComponentPlanReconciler) removeFromUninstallingMap(plan *corev1alpha1.ComponentPlan) {
	key := r.cacheMapKey(plan)
	r.helmUninstalling.Delete(key)
}
