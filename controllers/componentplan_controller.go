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
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
	"github.com/kubebb/core/pkg/repoimage"
	"github.com/kubebb/core/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kustomize/api/krusty"
	kustomize "sigs.k8s.io/kustomize/api/types"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

// ComponentPlanReconciler reconciles a ComponentPlan object
type ComponentPlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// For https://github.com/kubernetes-sigs/kustomize/issues/3659
	kustomizeRenderMutex sync.Mutex
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
		// There's no need to requeue if the resource no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		logger.V(4).Info("Failed to get ComponentPlan")
		return reconcile.Result{}, utils.IgnoreNotFound(err)
	}
	logger.V(4).Info("Get ComponentPlan instance")

	// Get watched component
	component := &corev1alpha1.Component{}
	err = r.Get(ctx, types.NamespacedName{Namespace: plan.Spec.ComponentRef.Namespace, Name: plan.Spec.ComponentRef.Name}, component)
	if err != nil {
		logger.Error(err, "Failed to get Component", "Component.Namespace", plan.Spec.ComponentRef.Namespace, "Component.Name", plan.Spec.ComponentRef.Name)
		return reconcile.Result{Requeue: true, RequeueAfter: time.Minute}, utils.IgnoreNotFound(err)
	}
	logger.V(4).Info("Get Component instance", "Component.Namespace", component.Namespace, "Component.Name", component.Name)

	// Update spec.repositoryRef
	if plan.Spec.RepositoryRef == nil || plan.Spec.RepositoryRef.Name == "" {
		newPlan := plan.DeepCopy()
		for _, o := range component.GetOwnerReferences() {
			if o.Kind == "Repository" {
				newPlan.Spec.RepositoryRef = &corev1.ObjectReference{
					Name:      o.Name,
					Namespace: component.Namespace,
				}
				break
			}
		}
		if err = r.Patch(ctx, newPlan, client.MergeFrom(plan)); err != nil {
			logger.Error(err, "Failed to patch ComponentPlan.Spec.RepositoryRef", "Repository.Namespace", newPlan.Spec.RepositoryRef.Namespace, "Repository.Name", newPlan.Spec.RepositoryRef.Name)
			return ctrl.Result{Requeue: true}, err
		}
		logger.V(4).Info("Patch ComponentPlan.Spec.RepositoryRef")
		return ctrl.Result{Requeue: true}, nil
	}

	// Set the status as Unknown when no status are available, and add a finalizer
	if plan.Status.Conditions == nil || len(plan.Status.Conditions) == 0 {
		if plan.Spec.Approved {
			_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanAppreoved(), corev1alpha1.ComponentPlanWaitInstall())
		} else {
			_ = r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnAppreoved(), corev1alpha1.ComponentPlanWaitInstall())
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Add a finalizer. Then, we can define some operations which should
	// occurs before the ComponentPlan to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(plan, corev1alpha1.Finalizer) {
		logger.Info("Adding Finalizer for ComponentPlan")
		if ok := controllerutil.AddFinalizer(plan, corev1alpha1.Finalizer); !ok {
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}

		if err = r.Update(ctx, plan); err != nil {
			logger.Error(err, "Failed to update ComponentPlan to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if the ComponentPlan instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPlanMarkedToBeDeleted := plan.GetDeletionTimestamp() != nil
	if isPlanMarkedToBeDeleted && controllerutil.ContainsFinalizer(plan, corev1alpha1.Finalizer) {
		logger.Info("Performing Finalizer Operations for Plan before delete CR")

		// Perform all operations required before remove the finalizer and allow
		// the Kubernetes API to remove ComponentPlan
		if err = r.doFinalizerOperationsForPlan(ctx, plan); err != nil {
			logger.Error(err, "Failed to re-fetch ComponentPlan")
			return ctrl.Result{}, err
		}

		logger.Info("Removing Finalizer for ComponentPlan after successfully perform the operations")
		if ok := controllerutil.RemoveFinalizer(plan, corev1alpha1.Finalizer); !ok {
			return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}

		if err = r.Update(ctx, plan); err != nil {
			logger.Error(err, "Failed to remove finalizer for ComponentPlan")
			return ctrl.Result{}, err
		}
	}

	if plan.Status.GetCondition(corev1alpha1.ComponentPlanTypeSucceeded).Status == corev1.ConditionTrue {
		logger.Info("ComponentPlan is already succeeded, no need to reconcile")
		return ctrl.Result{}, nil
	}

	// Check its helm template configmap exist
	manifest := &corev1.ConfigMap{}
	manifest.Name = corev1alpha1.GenerateComponentPlanManifestConfigMapName(plan)
	manifest.Namespace = plan.Namespace
	err = r.Get(ctx, types.NamespacedName{Name: manifest.Name, Namespace: manifest.Namespace}, manifest)
	if err != nil && apierrors.IsNotFound(err) {
		component := &corev1alpha1.Component{}
		if err = r.Get(ctx, types.NamespacedName{Name: plan.Spec.ComponentRef.Name, Namespace: plan.Spec.ComponentRef.Namespace}, component); err != nil {
			logger.Error(err, "Failed to get Component")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}
		chartName := component.Status.Name
		if chartName == "" {
			err = errors.New("chart name is empty")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}

		repo := &corev1alpha1.Repository{}
		if err = r.Get(ctx, types.NamespacedName{Name: plan.Spec.RepositoryRef.Name, Namespace: plan.Spec.RepositoryRef.Namespace}, repo); err != nil {
			logger.Error(err, "Failed to get Repository")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}
		repoName := repo.Name
		repoUrl := repo.Spec.URL
		if repoUrl == "" {
			err = errors.New("repo url is empty")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}

		data, err := helm.GetManifests(ctx, logger, plan.Spec.Config.Name, plan.Namespace, repoName+"/"+chartName, plan.Spec.InstallVersion, repoName, repoUrl,
			plan.Spec.Config.Override.Set, plan.Spec.Config.Override.SetString, plan.Spec.Config.Override.SetFile, plan.Spec.Config.Override.SetJSON, plan.Spec.Config.Override.SetLiteral,
			plan.Spec.Config.SkipCRDs, false)
		if err != nil {
			logger.Error(err, "Failed to get manifest")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}

		data, err = r.UpdateImages(ctx, data, repo.Spec.ImageOverride, plan.Spec.Config.Override.Images)
		if err != nil {
			logger.Error(err, "Failed to Update Images")
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}

		if err = r.GenerateManifestConfigMap(plan, manifest, data); err != nil {
			logger.Error(err, "Failed to generate manifest configmap")
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}
		logger.Info("Generate a new template Configmap", "ConfigMap.Namespace", manifest.Namespace, "ConfigMap.Name", manifest.Name)

		newPlan := plan.DeepCopy()
		newPlan.Status.Resources, newPlan.Status.Images, err = utils.GetResourcesAndImages(ctx, logger, r.Client, data)
		if err != nil {
			logger.Error(err, "Failed to get resources")
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}
		err = r.Status().Patch(ctx, newPlan, client.MergeFrom(plan))
		if err != nil {
			logger.Error(err, "Failed to update ComponentPlan status.Resources")
		}
		logger.Info("Update ComponentPlan status.Resources")

		if err = r.Create(ctx, manifest); err != nil {
			logger.Error(err, "Failed to create new template Configmap", "ConfigMap.Namespace", manifest.Namespace, "ConfigMap.Name", manifest.Name)
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		}
		logger.Info("Create ComponentPlan template Configmap")
		// Configmap created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Second}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstalling())
	} else if err != nil {
		logger.Error(err, "Failed to get template Configmap")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
	}

	install := func() (ctrl.Result, error) {
		if !r.needRetry(plan.Spec.Config.MaxRetry, manifest.Labels) {
			err = errors.New("max retry reached")
			_ = r.UpdateManifestConfigMapLabel(ctx, plan, corev1alpha1.ComponentPlanReasonInstallFailed)
			logger.Error(err, "will not retry")
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanUnSucceeded(err))
		}
		var data []string
		for _, v := range manifest.Data {
			data = append(data, v)
		}
		_ = r.UpdateManifestConfigMapLabel(ctx, plan, corev1alpha1.ComponentPlanReasonInstalling)
		logger.Info("install the component plan now...")
		err = helm.Install(ctx, r.Client, plan.Name, data)
		if err != nil {
			_ = r.UpdateManifestConfigMapLabel(ctx, plan, corev1alpha1.ComponentPlanReasonInstallFailed)
			logger.Error(err, "install failed")
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallFailed(err))
		} else {
			logger.Info("install successfully")
			_ = r.UpdateManifestConfigMapLabel(ctx, plan, corev1alpha1.ComponentPlanReasonInstallSuccess)
			return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallSuccess())
		}
	}

	logger.Info("install the component plan...")
	switch manifest.Labels[string(corev1alpha1.ComponentPlanTypeInstalled)] {
	case "":
		fallthrough
	case string(corev1alpha1.ComponentPlanReasonWaitInstall):
		logger.Info("try to first install the component plan to cluster...")
		return install()
	case string(corev1alpha1.ComponentPlanReasonInstalling):
		logger.Info("last one is installing... skip for 1 minute")
		// Just installing the helm chart, wait one minute to recheck
		return ctrl.Result{RequeueAfter: time.Minute}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstalling())
	case string(corev1alpha1.ComponentPlanReasonInstallSuccess):
		logger.Info("last one install success. just return")
		return ctrl.Result{}, r.PatchCondition(ctx, plan, logger, corev1alpha1.ComponentPlanInstallSuccess())
	case string(corev1alpha1.ComponentPlanReasonInstallFailed):
		logger.Info("last one install failed.")
		return install()
	}
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
func (r *ComponentPlanReconciler) doFinalizerOperationsForPlan(ctx context.Context, plan *corev1alpha1.ComponentPlan) (err error) {
	return helm.UnInstallByResources(ctx, r.Client, plan.Namespace, plan.Name, plan.Status.Resources)
}

func (r *ComponentPlanReconciler) GenerateManifestConfigMap(plan *corev1alpha1.ComponentPlan, manifest *corev1.ConfigMap, data []string) (err error) {
	if manifest.Labels == nil {
		manifest.Labels = make(map[string]string)
	}
	manifest.Labels[string(corev1alpha1.ComponentPlanTypeInstalled)] = string(corev1alpha1.ComponentPlanReasonWaitInstall)
	manifest.Data = make(map[string]string)
	for i, d := range data {
		manifest.Data[fmt.Sprintf("%d", i)] = d
	}
	return controllerutil.SetOwnerReference(plan, manifest, r.Scheme)
}

// PatchCondition patch subscription status condition
func (r *ComponentPlanReconciler) PatchCondition(ctx context.Context, plan *corev1alpha1.ComponentPlan, logger logr.Logger, condition ...corev1alpha1.Condition) (err error) {
	newPlan := plan.DeepCopy()
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
	if err := r.Status().Patch(ctx, newPlan, client.MergeFrom(plan)); err != nil {
		logger.Error(err, "Failed to patch ComponentPlan status")
		return err
	}
	return nil
}

func (r *ComponentPlanReconciler) UpdateManifestConfigMapLabel(ctx context.Context, plan *corev1alpha1.ComponentPlan, val corev1alpha1.ConditionReason) (err error) {
	cm := &corev1.ConfigMap{}
	cm.Name = corev1alpha1.GenerateComponentPlanManifestConfigMapName(plan)
	cm.Namespace = plan.Namespace
	if err = r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, cm); err != nil {
		return err
	}
	newCm := cm.DeepCopy()
	if newCm.Labels == nil {
		newCm.Labels = make(map[string]string)
	}
	newCm.Labels[string(corev1alpha1.ComponentPlanTypeInstalled)] = string(val)
	if val == corev1alpha1.ComponentPlanReasonInstallFailed {
		oldN := int64(0)
		old := cm.Labels[corev1alpha1.ComponentPlanConfigMapRetryLabelKey]
		if old != "" {
			oldN, err = strconv.ParseInt(old, 10, 64)
			if err != nil {
				return err
			}
		}
		newCm.Labels[corev1alpha1.ComponentPlanConfigMapRetryLabelKey] = strconv.FormatInt(oldN+1, 10)
	}
	return r.Patch(ctx, newCm, client.MergeFrom(cm))
}

func (r *ComponentPlanReconciler) needRetry(maxRetry *int64, label map[string]string) bool {
	if maxRetry == nil {
		maxRetry = pointer.Int64(5)
	}
	v, ok := label[corev1alpha1.ComponentPlanConfigMapRetryLabelKey]
	if ok {
		retry, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return false
		}
		return retry < *maxRetry
	}
	return 1 < *maxRetry
}

func (r *ComponentPlanReconciler) UpdateImages(ctx context.Context, jsonManifests []string, repoOverride []corev1alpha1.ImageOverride, images []kustomize.Image) (jsonData []string, err error) {
	if len(repoOverride) == 0 && len(images) == 0 {
		return jsonManifests, nil
	}
	fs := filesys.MakeFsInMemory()
	cfg := kustypes.Kustomization{}
	cfg.APIVersion = kustypes.KustomizationVersion
	cfg.Kind = kustypes.KustomizationKind
	cfg.Images = images

	for i, manifest := range jsonManifests {
		fileName := fmt.Sprintf("%d.json", i)
		cfg.Resources = append(cfg.Resources, fileName)
		f, err := fs.Create(fileName)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if _, err = f.Write([]byte(manifest)); err != nil {
			return nil, err
		}
	}

	kustomization, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	f, err := fs.Create("kustomization.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err = f.Write(kustomization); err != nil {
		return nil, err
	}

	r.kustomizeRenderMutex.Lock()
	defer r.kustomizeRenderMutex.Unlock()

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	resMap, err := k.Run(fs, ".")
	if err != nil {
		return nil, err
	}
	path := corev1alpha1.GetImageOverridePath()
	if len(path) != 0 && len(repoOverride) != 0 {
		fsslice := make([]kustypes.FieldSpec, len(path))
		for i, p := range path {
			fsslice[i] = kustypes.FieldSpec{Path: p}
		}
		if err = resMap.ApplyFilter(repoimage.Filter{ImageOverride: repoOverride, FsSlice: fsslice}); err != nil {
			return nil, err
		}
	}
	yamlResults, err := resMap.AsYaml()
	if err != nil {
		return nil, err
	}
	separator := "\n---\n"
	results := bytes.Split(yamlResults, []byte(separator))
	for _, i := range results {
		if len(i) != 0 {
			j, err := yaml.YAMLToJSON(i)
			if err != nil {
				return nil, err
			}
			jsonData = append(jsonData, string(j))
		}
	}
	return jsonData, nil
}
