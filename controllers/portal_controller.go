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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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

// PortalReconciler reconciles a Portal object
type PortalReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn.kubebb.k8s.com.cn,resources=portals,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn.kubebb.k8s.com.cn,resources=portals/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn.kubebb.k8s.com.cn,resources=portals/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *PortalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting portal reconcile")

	var portal = corev1alpha1.Portal{}
	if err := r.Client.Get(ctx, req.NamespacedName, &portal); err != nil {
		if k8serrors.IsNotFound(err) {
			// Portal has been deleted.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// handle conflicts
	entryConflicts, pathConflicts, err := GetConflicts(r.Client, ctx, &portal)
	if err != nil {
		logger.Error(err, "Failed to get duplicate portals")
		return reconcile.Result{}, err
	}

	status := corev1alpha1.PortalStatus{}
	status.ConflictsInEntry = entryConflicts
	status.ConflictsInPath = pathConflicts

	portalCopy := portal.DeepCopy()
	portalCopy.Status = status
	err = r.Client.Status().Update(ctx, portalCopy)
	if err != nil {
		logger.Error(err, "Failed to update portal's status")
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PortalReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Portal{}, builder.WithPredicates(PortalPredicate{})).
		Watches(&source.Kind{Type: &corev1alpha1.Portal{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			portal := o.(*corev1alpha1.Portal)
			cEntrys, cPaths, err := GetConflicts(r.Client, ctx, portal)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "Failed to get duplicate portals")
				return nil
			}

			var reqs []reconcile.Request
			var dupPortalMap = make(map[string]bool)
			for _, p := range append(cEntrys, cPaths...) {
				if p == portal.Name {
					continue
				}
				if _, ok := dupPortalMap[p]; !ok {
					dupPortalMap[p] = true
					reqs = append(reqs, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: p,
						},
					})
				}
			}

			return reqs
		})).
		Complete(r)
}

type PortalPredicate struct {
	predicate.Funcs
}

func (p PortalPredicate) Update(ue event.UpdateEvent) bool {
	oldObj := ue.ObjectOld.(*corev1alpha1.Portal)
	newObj := ue.ObjectNew.(*corev1alpha1.Portal)

	return oldObj.Spec.Entry != newObj.Spec.Entry || oldObj.Spec.Path != newObj.Spec.Path
}

// GetConflicts returns the entry and path conflicts for a given portal.
func GetConflicts(c client.Client, ctx context.Context, portal *corev1alpha1.Portal) ([]string, []string, error) {
	portalCopy := portal.DeepCopy()

	var list = corev1alpha1.PortalList{}
	err := c.List(ctx, &list)
	if err != nil {
		return nil, nil, err
	}

	entryConflicts := make([]string, 0)
	pathConflicts := make([]string, 0)
	for _, p := range list.Items {
		if portalCopy.Name == p.Name {
			continue
		}

		// entry conflicts
		if portalCopy.Spec.Entry == p.Spec.Entry {
			entryConflicts = append(entryConflicts, p.Name)
		}

		// path conflicts
		if portalCopy.Spec.Path == p.Spec.Path {
			pathConflicts = append(pathConflicts, p.Name)
		}
	}

	return entryConflicts, pathConflicts, nil
}
