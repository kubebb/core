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
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
		if k8serror.IsNotFound(err) {
			list, err := r.getList(ctx, logger)
			if err != nil {
				return reconcile.Result{}, err
			}
			for _, item := range list {
				_, err = r.Reconcile(ctx, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: item.Namespace,
						Name:      item.Name,
					},
				})
				if err != nil {
					logger.Error(err, "error while reconciling other portals")
				}
			}

			// Portal has been deleted.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	err := r.updateDuplicate(ctx, logger, &portal, true)
	return ctrl.Result{}, err
}

func (r *PortalReconciler) updateDuplicate(ctx context.Context, logger logr.Logger, portal *corev1alpha1.Portal, init bool) error {
	portalCopy := portal.DeepCopy()
	list, err := r.getDuplicateList(ctx, logger, portalCopy)
	if err != nil {
		return err
	}

	logger.Info("updating duplicate for " + portalCopy.Name + " which has " + portalCopy.Status.ConflictedPortals)

	conflicts := ""
	for _, p := range list {
		conflicts = conflicts + ":/" + p.Name
	}
	conflicts = strings.TrimPrefix(conflicts, ":")

	status := corev1alpha1.PortalStatus{}
	status.Duplicated = false
	if len(conflicts) > 0 {
		logger.Error(errors.New("duplicate error"), "found portals with duplicate path and entry", "duplicates", conflicts)
		status.Duplicated = true
	}
	status.ConflictedPortals = conflicts
	portalCopy.Status = status

	logger.Info("Update to " + fmt.Sprint(portalCopy.Status.Duplicated) + " and " + fmt.Sprint(portalCopy.Status.ConflictedPortals))
	err = r.Client.Status().Update(ctx, portalCopy)
	if err != nil {
		logger.Error(err, "failed to add duplicate record")
		return err
	}
	return nil
}

func (r *PortalReconciler) getList(ctx context.Context, logger logr.Logger) ([]corev1alpha1.Portal, error) {
	var list = corev1alpha1.PortalList{}
	if err := r.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list portals")
		return nil, err
	}
	return list.Items, nil
}

func (r *PortalReconciler) getDuplicateList(ctx context.Context, logger logr.Logger, portal *corev1alpha1.Portal) ([]corev1alpha1.Portal, error) {
	list, err := r.getList(ctx, logger)
	if err != nil {
		return nil, err
	}
	var result []corev1alpha1.Portal
	for _, p := range list {
		if portal.Namespace == p.Namespace && portal.Name == p.Name {
			continue
		}
		if portal.Spec.Entry == p.Spec.Entry && portal.Spec.Path == p.Spec.Path {
			result = append(result, p)
		}
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PortalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Portal{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(ce event.CreateEvent) bool {
				obj := ce.Object.(*corev1alpha1.Portal)
				r.Recorder.Eventf(obj, v1.EventTypeNormal, "Created", "add new portal %s", obj.GetName())
				return true
			},
			UpdateFunc: func(ue event.UpdateEvent) bool {
				oldObj := ue.ObjectOld.(*corev1alpha1.Portal)
				newObj := ue.ObjectNew.(*corev1alpha1.Portal)
				return oldObj.Spec.Entry != newObj.Spec.Entry || oldObj.Spec.Path != newObj.Spec.Path
			},
			DeleteFunc: func(de event.DeleteEvent) bool {
				obj := de.Object.(*corev1alpha1.Portal)
				r.Recorder.Eventf(obj, v1.EventTypeNormal, "Deleted", "delete portal %s", obj.GetName())
				return true
			},
		})).
		Complete(r)
}
