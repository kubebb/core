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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kubebb/core/pkg/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
)

// SubscriptionReconciler reconciles a Subscription object
type SubscriptionReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	mu                 sync.RWMutex
	ComponentWatchMap  map[string]sets.String // key:component-name/component-namespace val: subscription-name/subscription-namespace
	RepositoryWatchMap map[string]sets.String // key:repository-name/repository-namespace val: subscription-name/subscription-namespace
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=subscriptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=subscriptions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=subscriptions/finalizers,verbs=update
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components,verbs=get;list;watch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components/status,verbs=get
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=repositorys,verbs=get;list;watch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=repositorys/status,verbs=get
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=componentplans,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SubscriptionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(4).Info("Reconciling Subscription")

	// Fetch the Subscription instance
	sub := &corev1alpha1.Subscription{}
	err := r.Get(ctx, req.NamespacedName, sub)
	if err != nil {
		// There's no need to requeue if the resource no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		logger.Error(err, "Failed to get Subscription")
		return reconcile.Result{}, utils.IgnoreNotFound(err)
	}
	logger.V(4).Info("Get Subscription instance")

	// Add to cache map
	if component := sub.Spec.ComponentRef; component != nil {
		key := component.Name + "/" + component.Namespace
		val := sub.Name + "/" + sub.Namespace
		r.AddToComponentWatchMap(key, val)
		logger.V(4).Info("Add to Component Watch Map", "Component.Name", component.Name, "Component.Namespace", component.Namespace)
	}
	if repository := sub.Spec.RepositoryRef; repository != nil {
		key := repository.Name + "/" + repository.Namespace
		val := sub.Name + "/" + sub.Namespace
		r.AddToRepositoryWatchMap(key, val)
		logger.V(4).Info("Add to Repository Watch Map", "Repository.Name", repository.Name, "Repository.Namespace", repository.Namespace)
	}

	// Get watched component
	component := &corev1alpha1.Component{}
	err = r.Get(ctx, types.NamespacedName{Namespace: sub.Spec.ComponentRef.Namespace, Name: sub.Spec.ComponentRef.Name}, component)
	if err != nil {
		logger.Error(err, "Failed to get Component", "Component.Namespace", sub.Spec.ComponentRef.Namespace, "Component.Name", sub.Spec.ComponentRef.Name)
		return reconcile.Result{}, utils.IgnoreNotFound(err)
	}
	logger.V(4).Info("Get Component instance", "Component.Namespace", component.Namespace, "Component.Name", component.Name)

	// Update spec.repositoryRef
	if sub.Spec.RepositoryRef == nil || sub.Spec.RepositoryRef.Name == "" {
		newSub := sub.DeepCopy()
		for _, o := range component.GetOwnerReferences() {
			if o.Kind == "Repository" {
				newSub.Spec.RepositoryRef = &corev1.ObjectReference{
					Name:      o.Name,
					Namespace: component.Namespace,
				}
				break
			}
		}
		if err = r.Patch(ctx, newSub, client.MergeFrom(sub)); err != nil {
			logger.Error(err, "Failed to patch subscription.spec.RepositoryRef", "Repository.Namespace", newSub.Spec.RepositoryRef.Namespace, "Repository.Name", newSub.Spec.RepositoryRef.Name)
			return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileError(corev1alpha1.SubscriptionTypeReady, err))
		}
		logger.V(4).Info("Patch subscription.Spec.RepositoryRef")
		return ctrl.Result{Requeue: true}, nil
	}

	// update status.RepositoryHealth
	if err = r.UpdateStatusRepositoryHealth(ctx, logger, sub); err != nil {
		logger.Error(err, "Failed to update subscription status repositoryHealth", "RepositoryNamespace", sub.Spec.RepositoryRef.Namespace, "RepositoryName", sub.Spec.RepositoryRef.Name)
		return ctrl.Result{Requeue: true}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileError(corev1alpha1.SubscriptionTypeReady, err))
	}
	logger.V(4).Info("patch subscription status repositoryHealth")

	// compare component latest version with installed
	var latestVersionFetch, latestVersionInstalled corev1alpha1.ComponentVersion
	if versions := component.Status.Versions; len(versions) > 0 {
		latestVersionFetch = versions[0]
	} else {
		msg := "component has no versions, skip"
		logger.Info(msg)
		return ctrl.Result{}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileSuccess(corev1alpha1.SubscriptionTypeReady).WithMessage(msg))
	}
	logger.V(4).Info("get component latest fetch version")
	if plans := sub.Status.Installed; len(plans) > 0 {
		latestVersionInstalled = plans[0].InstalledVersion
	}
	logger.V(4).Info("get component latest installed version")
	if latestVersionFetch.Equal(&latestVersionInstalled) {
		msg := "component latest version is the same as installed, skip"
		logger.Info(msg)
		return ctrl.Result{}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileSuccess(corev1alpha1.SubscriptionTypeReady).WithMessage(msg))
	}

	// create componentplan
	if err = r.CreateComponentPlan(ctx, sub, latestVersionFetch); err != nil {
		logger.Error(err, "Failed to create component plan")
		return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileError(corev1alpha1.SubscriptionTypePlanSynce, err))
	}
	logger.V(4).Info("create component plan")

	// update status.Installed
	if err = r.UpdateStatusInstalled(ctx, logger, sub, latestVersionFetch); err != nil {
		logger.Error(err, "Failed to update subscription status installed")
		return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileError(corev1alpha1.SubscriptionTypeReady, err))
	}
	logger.V(4).Info("update subscription status installed")
	return ctrl.Result{}, r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionAvailable())
}

// ExistInComponentWatchMap check if subscription exist in component watch map
func (r *SubscriptionReconciler) ExistInComponentWatchMap(key, val string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if subs, ok := r.ComponentWatchMap[key]; ok && subs.Has(val) {
		return true
	}
	return false
}

// AddToComponentWatchMap add subscription to component watch map
func (r *SubscriptionReconciler) AddToComponentWatchMap(key, val string) {
	if r.ExistInComponentWatchMap(key, val) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ComponentWatchMap == nil {
		r.ComponentWatchMap = make(map[string]sets.String)
	}
	if subs, ok := r.ComponentWatchMap[key]; ok {
		subs.Insert(val)
	} else {
		r.ComponentWatchMap[key] = sets.NewString(val)
	}
}

// ExistInRepositoryWatchMap check if subscription exist in repository watch map
func (r *SubscriptionReconciler) ExistInRepositoryWatchMap(key, val string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if subs, ok := r.RepositoryWatchMap[key]; ok && subs.Has(val) {
		return true
	}
	return false
}

// AddToRepositoryWatchMap add subscription to repository watch map
func (r *SubscriptionReconciler) AddToRepositoryWatchMap(key, val string) {
	if r.ExistInRepositoryWatchMap(key, val) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.RepositoryWatchMap == nil {
		r.RepositoryWatchMap = make(map[string]sets.String)
	}
	if subs, ok := r.RepositoryWatchMap[key]; ok {
		subs.Insert(val)
	} else {
		r.RepositoryWatchMap[key] = sets.NewString(val)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SubscriptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Subscription{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: r.SubscriptionUpdate,
			DeleteFunc: r.SubscriptionDelete,
		})).
		Watches(&source.Kind{Type: &corev1alpha1.Component{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return r.GetReqs(o, true)
			})).
		Watches(&source.Kind{Type: &corev1alpha1.Repository{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (reqs []reconcile.Request) {
				return r.GetReqs(o, false)
			})).
		Complete(r)
}

// GetReqs get subscription reqs
func (r *SubscriptionReconciler) GetReqs(o client.Object, watchComponent bool) (reqs []reconcile.Request) {
	key := o.GetName() + "/" + o.GetNamespace()
	r.mu.RLock()
	defer r.mu.RUnlock()
	var subs sets.String
	var ok bool
	if watchComponent {
		subs, ok = r.ComponentWatchMap[key]
	} else {
		subs, ok = r.RepositoryWatchMap[key]
	}
	if !ok {
		return
	}
	all := subs.List()
	for _, val := range all {
		name, ns, ok := strings.Cut(val, "/")
		if !ok {
			continue
		}
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKey{Name: name, Namespace: ns}})
	}
	return reqs
}

// SubscriptionUpdate check if subscription spec or status is updated
func (r *SubscriptionReconciler) SubscriptionUpdate(event event.UpdateEvent) bool {
	oldSub, ok := event.ObjectOld.(*corev1alpha1.Subscription)
	if !ok {
		return false
	}
	newSub, ok := event.ObjectNew.(*corev1alpha1.Subscription)
	if !ok {
		return false
	}
	if !reflect.DeepEqual(oldSub.Spec, newSub.Spec) {
		return true
	}
	if !reflect.DeepEqual(oldSub.Status, newSub.Status) {
		return true
	}
	return false
}

// SubscriptionDelete clean Reconciler cache map when subscription is deleted
func (r *SubscriptionReconciler) SubscriptionDelete(event event.DeleteEvent) bool {
	sub, ok := event.Object.(*corev1alpha1.Subscription)
	if !ok {
		return false
	}
	val := sub.Name + "/" + sub.Namespace
	component := sub.Spec.ComponentRef.Name + "/" + sub.Spec.ComponentRef.Namespace
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ComponentWatchMap[component].Delete(val)
	if sub.Spec.RepositoryRef != nil {
		repository := sub.Spec.RepositoryRef.Name + "/" + sub.Spec.RepositoryRef.Namespace
		r.RepositoryWatchMap[repository].Delete(val)
	}
	return false
}

// UpdateStatusRepositoryHealth get repository CR, check if the repository is healthy and updates subscription status.RepositoryHealth
func (r *SubscriptionReconciler) UpdateStatusRepositoryHealth(ctx context.Context, logger logr.Logger, sub *corev1alpha1.Subscription) (err error) {
	repo := &corev1alpha1.Repository{}
	if err = r.Get(ctx, types.NamespacedName{Namespace: sub.Spec.RepositoryRef.Namespace, Name: sub.Spec.RepositoryRef.Name}, repo); err != nil {
		logger.Error(err, "Failed to get repository", "Repository.Namespace", sub.Spec.RepositoryRef.Namespace, "Repository.Name", sub.Spec.RepositoryRef.Name)
		if err1 := r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileError(corev1alpha1.SubscriptionTypePlanSynce, err)); err1 != nil {
			return errors.Wrap(err, err1.Error())
		}
		return err
	}
	// FIXME check if the repository is healthy
	healthy := true
	if repo.Status.GetCondition(corev1alpha1.TypeReady).Status != corev1.ConditionTrue {
		healthy = false
	}
	newSub := sub.DeepCopy()
	now := metav1.Now()
	newSub.Status.RepositoryHealth = corev1alpha1.RepositoryHealth{
		RepositoryRef: &corev1.ObjectReference{Name: repo.Name, Namespace: repo.Namespace},
		LastUpdated:   &now,
		Healthy:       &healthy,
	}
	newSub.Status.SetConditions(corev1alpha1.SubscriptionReconcileSuccess(corev1alpha1.SubscriptionTypeSourceSynced))
	if err = r.Status().Patch(ctx, newSub, client.MergeFrom(sub)); err != nil {
		logger.Error(err, "Failed to patch subscription status repositoryHealth")
		return err
	}

	return nil
}

// CreateComponentPlan create component plan if not exists or update component plan if exists
func (r *SubscriptionReconciler) CreateComponentPlan(ctx context.Context, sub *corev1alpha1.Subscription, fetch corev1alpha1.ComponentVersion) error {
	plan := &corev1alpha1.ComponentPlan{}
	plan.Name = corev1alpha1.GenerateComponentPlanName(sub, fetch.Version)
	plan.Namespace = sub.Namespace
	plan.Spec.Override = sub.Spec.Override
	plan.Spec.ComponentRef = sub.Spec.ComponentRef
	plan.Spec.InstallVersion = fetch.Version
	if sub.Spec.ComponentPlanInstallMethod.IsAuto() {
		plan.Spec.Approved = true
	}
	return r.Create(ctx, plan)
}

// UpdateStatusInstalled update subscription status installed
func (r *SubscriptionReconciler) UpdateStatusInstalled(ctx context.Context, logger logr.Logger, sub *corev1alpha1.Subscription, fetch corev1alpha1.ComponentVersion) (err error) {
	plan := &corev1alpha1.ComponentPlan{}
	planName := corev1alpha1.GenerateComponentPlanName(sub, fetch.Version)
	planNs := sub.Namespace
	if err = r.Get(ctx, types.NamespacedName{Namespace: planNs, Name: planName}, plan); err != nil {
		logger.Error(err, "Failed to get componentPlan", "ComponentPlan.Namespace", planNs, "ComponentPlan.Name", planName)
		if err1 := r.PatchCondition(ctx, sub, corev1alpha1.SubscriptionReconcileError(corev1alpha1.SubscriptionTypePlanSynce, err)); err1 != nil {
			return errors.Wrap(err, err1.Error())
		}
		return err
	}
	newSub := sub.DeepCopy()
	// latest should be first item
	newSub.Status.Installed = append([]corev1alpha1.Installed{
		{
			InstalledVersion: fetch,
			InstalledTime:    metav1.Now(),
			ComponentPlanRef: &corev1.ObjectReference{Name: plan.Name, Namespace: plan.Namespace},
		},
	}, sub.Status.Installed...)
	newSub.Status.SetConditions(corev1alpha1.SubscriptionReconcileSuccess(corev1alpha1.SubscriptionTypePlanSynce))
	return r.Status().Patch(ctx, newSub, client.MergeFrom(sub))
}

// PatchCondition patch subscription status condition
func (r *SubscriptionReconciler) PatchCondition(ctx context.Context, sub *corev1alpha1.Subscription, condition corev1alpha1.Condition) (err error) {
	newSub := sub.DeepCopy()
	newSub.Status.SetConditions(condition)
	ready := true
	for _, cond := range newSub.Status.Conditions {
		if cond.Type == corev1alpha1.TypeReady {
			continue
		}
		if cond.Status != corev1.ConditionTrue {
			ready = false
			break
		}
	}
	if ready {
		newSub.Status.SetConditions(corev1alpha1.SubscriptionAvailable())
	} else {
		newSub.Status.SetConditions(corev1alpha1.SubscriptionUnavaliable())
	}
	return r.Status().Patch(ctx, newSub, client.MergeFrom(sub))
}
