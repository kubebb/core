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
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
	"github.com/kubebb/core/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	maxRetryErr = errors.New("max retry reached, will not retry")
)

// ComponentPlanReconciler reconciles a ComponentPlan object
type ComponentPlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
	logger.V(4).Info("Reconciling ComponentPlan")

	// Fetch the ComponentPlan instance
	plan := &corev1alpha1.ComponentPlan{}
	err := r.Get(ctx, req.NamespacedName, plan)
	if err != nil {
		// There's no need to requeue if the resource no longer exists.
		// Otherwise, we'll be requeued implicitly because we return an error.
		logger.V(4).Info("Failed to get ComponentPlan")
		return reconcile.Result{}, utils.IgnoreNotFound(err)
	}
	logger.V(4).Info("Get ComponentPlan instance")

	// Get watched component
	component := &corev1alpha1.Component{}
	err = r.Get(ctx, types.NamespacedName{Namespace: plan.Spec.ComponentRef.Namespace, Name: plan.Spec.ComponentRef.Name}, component)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Failed to get Component, wait 1 minute for Component to be found", "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		} else {
			logger.Error(err, "Failed to get Component, wait 1 minute for Component to be found", "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		}
		return reconcile.Result{Requeue: true, RequeueAfter: time.Minute}, utils.IgnoreNotFound(err)
	}
	if component.Status.RepositoryRef == nil || component.Status.RepositoryRef.Name == "" || component.Status.RepositoryRef.Namespace == "" {
		logger.Info("Failed to get Component.Status.RepositoryRef, wait 30s to retry", "obj", klog.KObj(component))
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 30}, nil
	}
	logger.V(4).Info("Get Component instance", "Component", klog.KObj(component))

	if plan.Labels[corev1alpha1.ComponentPlanReleaseNameLabel] != plan.Spec.Name {
		if plan.GetLabels() == nil {
			plan.Labels = make(map[string]string)
		}
		plan.Labels[corev1alpha1.ComponentPlanReleaseNameLabel] = plan.Spec.Name
		err = r.Update(ctx, plan)
		if err != nil {
			logger.Error(err, "Failed to update ComponentPlan release label")
		}
		return ctrl.Result{Requeue: true}, err
	}

	// Set the status as Unknown when no status are available.
	if plan.Status.Conditions == nil || len(plan.Status.Conditions) == 0 {
		return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, plan.InitCondition()...)
	}

	// Add a finalizer. Then, we can define some operations which should
	// occur before the ComponentPlan to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(plan, corev1alpha1.Finalizer) {
		logger.Info("Try to add Finalizer for ComponentPlan")
		if ok := controllerutil.AddFinalizer(plan, corev1alpha1.Finalizer); !ok {
			logger.Info("Finalizer for ComponentPlan has already been added")
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}

		if err = r.Update(ctx, plan); err != nil {
			logger.Error(err, "Failed to update ComponentPlan to add finalizer, will try again later")
			return ctrl.Result{}, err
		}
		logger.Info("Adding Finalizer for ComponentPlan done")
		return ctrl.Result{}, nil
	}

	// Get RESTClientGetter for helm stuff
	getter, err := r.buildRESTClientGetter()
	if err != nil {
		logger.Error(err, "Failed to build RESTClientGetter")
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}

	// updateLatest try to update all componentplan's status.Latest
	go r.updateLatest(ctx, logger, getter, plan)

	// Check if the ComponentPlan instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPlanMarkedToBeDeleted := plan.GetDeletionTimestamp() != nil
	if isPlanMarkedToBeDeleted && controllerutil.ContainsFinalizer(plan, corev1alpha1.Finalizer) {
		logger.Info("Performing Finalizer Operations for Plan before delete CR")
		if plan.InActionedReason(corev1alpha1.ComponentPlanReasonUnInstalling) {
			logger.Info("the last one is uninstalling... skip for 1 minute")
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
				return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
			}

			if err = r.Update(ctx, plan); err != nil {
				logger.Error(err, "Failed to remove finalizer for ComponentPlan")
				return ctrl.Result{}, err
			}
			logger.Info("Remove ComponentPlan done")
			return ctrl.Result{}, nil
		}

		_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnInstalling())
		// Perform all operations required before remove the finalizer and allow
		// the Kubernetes API to remove ComponentPlan
		go func() {
			if err = r.doFinalizerOperationsForPlan(ctx, logger, getter, plan); err != nil {
				logger.Error(err, "Failed to uninstall ComponentPlan")
				_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnInstallFailed(err))
			}
			_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnInstallSuccess())
		}()
		return ctrl.Result{}, nil
	}

	if plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeSucceeded).Status == corev1.ConditionTrue {
		if r.isGenerationUpdate(plan) {
			logger.Info("ComponentPlan.Spec is changed, need to install or upgrade...")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, plan.InitCondition()...)
		} else {
			logger.Info("ComponentPlan is unchanged and has been successful, no need to reconcile")
			return ctrl.Result{}, nil
		}
	}

	repo := &corev1alpha1.Repository{}
	if err = r.Get(ctx, types.NamespacedName{Name: component.Status.RepositoryRef.Name, Namespace: component.Status.RepositoryRef.Namespace}, repo); err != nil {
		logger.Error(err, "Failed to get Repository, wait 15 seconds for another try")
		return ctrl.Result{Requeue: true, RequeueAfter: 15 * time.Second}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanWaitDo(err))
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
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanWaitDo(err))
		}

		if err = r.GenerateManifestConfigMap(plan, manifest, data); err != nil {
			logger.Error(err, "Failed to generate manifest configmap")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanWaitDo(err))
		}
		logger.Info("Generate a new template Configmap", "ConfigMap", klog.KObj(manifest))

		newPlan := plan.DeepCopy()
		newPlan.Status.Resources, newPlan.Status.Images, err = utils.GetResourcesAndImages(ctx, logger, r.Client, data)
		if err != nil {
			logger.Error(err, "Failed to get resources")
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanWaitDo(err))
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
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanWaitDo(err))
		}
		logger.Info(fmt.Sprintf("Reconcile ComponentPlan template Configmap, result:%s", res))
	} else if err != nil {
		logger.Error(err, "Failed to get template Configmap")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanWaitDo(err))
	}

	// Install the component plan
	if !plan.Spec.Approved {
		logger.Info("component plan isn't approved, skip install or upgrade...")
		return ctrl.Result{}, nil
	}
	if r.isDone(plan) {
		return ctrl.Result{}, nil
	}
	if r.isHelmDoing(plan) {
		logger.Info("the last one is installing/upgrding... skip for 1 minute")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	if !r.needRetry(plan) {
		logger.Info(maxRetryErr.Error())
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnSucceeded(maxRetryErr))
	}

	rel, err := helm.GetLastRelease(getter, logger, plan)
	if err != nil {
		logger.Error(err, "Failed to get last release")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnSucceeded(err))
	}
	var install bool
	if rel == nil {
		install = true
		logger.Info("no release found, will install the component plan")
		_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstalling())
	} else {
		logger.Info(fmt.Sprintf("in cluster find release version:%d, will upgrade", rel.Version), helm.ReleaseLog(rel)...)
		_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUpgrading())
	}

	go func(install bool) {
		rel, err := helm.InstallOrUpgrade(ctx, getter, r.Client, logger, plan, repo, chartName)
		if err != nil {
			if install {
				logger.Error(err, "install failed")
				_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
			} else {
				logger.Error(err, "upgrade failed")
				_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUpgradeFailed(err))
			}
			return
		}
		if install {
			logger.Info("install successfully", helm.ReleaseLog(rel)...)
			_ = r.PatchConditionWithRevision(ctx, plan, logger, rel.Version, corev1alpha1.ComponentPlanInstallSuccess())
		} else {
			logger.Info("upgrade successfully", helm.ReleaseLog(rel)...)
			_ = r.PatchConditionWithRevision(ctx, plan, logger, rel.Version, corev1alpha1.ComponentPlanUpgradeSuccess())
		}
	}(install)
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
func (r *ComponentPlanReconciler) PatchCondition(ctx context.Context, plan *corev1alpha1.ComponentPlan, logger logr.Logger, condition ...corev1alpha1.Condition) (err error) {
	return r.patchConditionWithFunc(ctx, plan, logger, []func(plan *corev1alpha1.ComponentPlan) (newPlan *corev1alpha1.ComponentPlan){
		r.updateObservedGeneration,
		r.updatePlanRetryTimes,
	}, condition...)
}

// PatchCondition patch subscription status condition
func (r *ComponentPlanReconciler) patchConditionWithFunc(ctx context.Context, plan *corev1alpha1.ComponentPlan, logger logr.Logger, changes []func(plan *corev1alpha1.ComponentPlan) (newPlan *corev1alpha1.ComponentPlan), condition ...corev1alpha1.Condition) (err error) {
	newPlan := r.setCondition(plan, condition...)
	for _, change := range changes {
		newPlan = change(newPlan)
	}
	if err := r.Status().Patch(ctx, newPlan, client.MergeFrom(plan)); err != nil {
		logger.Error(err, "Failed to patch ComponentPlan status")
		return err
	}
	return nil
}

// PatchConditionWithRevision patch subscription status condition
func (r *ComponentPlanReconciler) PatchConditionWithRevision(ctx context.Context, plan *corev1alpha1.ComponentPlan, logger logr.Logger, revision int, condition ...corev1alpha1.Condition) (err error) {
	return r.patchConditionWithFunc(ctx, plan, logger, []func(plan *corev1alpha1.ComponentPlan) (newPlan *corev1alpha1.ComponentPlan){
		r.updateObservedGeneration,
		r.updatePlanRetryTimes,
		func(plan *corev1alpha1.ComponentPlan) (newPlan *corev1alpha1.ComponentPlan) {
			newPlan = plan.DeepCopy()
			newPlan.Status.InstalledRevision = revision
			newPlan.Status.Latest = pointer.Bool(true)
			return newPlan
		},
	}, condition...)
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

func (r *ComponentPlanReconciler) updatePlanRetryTimes(plan *corev1alpha1.ComponentPlan) (newPlan *corev1alpha1.ComponentPlan) {
	newPlan = plan.DeepCopy()
	if r.isDone(plan) {
		return plan
	}
	annotation := plan.GetAnnotations()
	if annotation == nil {
		annotation = make(map[string]string)
	}
	hasRetry, _ := strconv.Atoi(annotation[corev1alpha1.ComponentPlanRetryTimesAnnotation])
	annotation[corev1alpha1.ComponentPlanRetryTimesAnnotation] = strconv.Itoa(hasRetry + 1)
	newPlan.SetAnnotations(annotation)
	return newPlan
}

func (r *ComponentPlanReconciler) updateObservedGeneration(plan *corev1alpha1.ComponentPlan) (newPlan *corev1alpha1.ComponentPlan) {
	if !r.isDone(plan) {
		return plan
	}
	plan.Status.ObservedGeneration = plan.GetGeneration()
	return plan
}

func (r *ComponentPlanReconciler) needRetry(plan *corev1alpha1.ComponentPlan) bool {
	hasRetry, _ := strconv.Atoi(plan.GetAnnotations()[corev1alpha1.ComponentPlanRetryTimesAnnotation])
	return hasRetry < plan.Spec.Config.GetMaxRetry()
}

func (r *ComponentPlanReconciler) buildRESTClientGetter() (genericclioptions.RESTClientGetter, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config for in-cluster REST client: %w", err)
	}
	config := genericclioptions.NewConfigFlags(true).WithDiscoveryBurst(cfg.Burst).WithDiscoveryQPS(cfg.QPS)
	return config, nil
}

func (r *ComponentPlanReconciler) isGenerationUpdate(plan *corev1alpha1.ComponentPlan) bool {
	return plan.Status.ObservedGeneration != 0 && plan.Status.ObservedGeneration != plan.GetGeneration()
}

func (r *ComponentPlanReconciler) isDone(plan *corev1alpha1.ComponentPlan) bool {
	installedCondition := plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeActioned)
	// if successed or max retry times reached, no need to update retry times
	if installedCondition.Status == corev1.ConditionTrue || installedCondition.Message == maxRetryErr.Error() {
		return true
	}
	return false
}

func (r *ComponentPlanReconciler) isHelmDoing(plan *corev1alpha1.ComponentPlan) bool {
	return plan.InActionedReason(corev1alpha1.ComponentPlanReasonInstalling) || plan.InActionedReason(corev1alpha1.ComponentPlanReasonUpgrading)
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
