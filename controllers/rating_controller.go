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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

// RatingReconciler reconciles a Rating object
type RatingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=ratings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=ratings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=ratings/finalizers,verbs=update
//+kubebuilder:rbac:groups=tekton.dev,resources=tasks;taskruns;pipelines;pipelineruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tekton.dev,resources=tasks/status;taskruns/status;pipelines/status;pipelineruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tekton.dev,resources=tasks/finalizers;taskruns/finalizers;pipelines/finalizers;pipelineruns/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Rating object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *RatingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := corev1alpha1.Rating{}
	logger.Info("starting rating reconcile")
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	done, err := r.ratingChecker(ctx, &instance)
	if !done {
		return reconcile.Result{Requeue: true}, err
	}

	if err := corev1alpha1.CreatePipelineRun(ctx, r.Client, r.Scheme, &instance, logger); err != nil {
		logger.Error(err, "")
		return reconcile.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r RatingReconciler) ratingChecker(ctx context.Context, instance *corev1alpha1.Rating) (bool, error) {
	if instance.Labels == nil {
		instance.Labels = make(map[string]string)
	}

	updateLabel := false
	if v, ok := instance.Labels[corev1alpha1.RatingComponentLabel]; !ok || v != instance.Spec.ComponentName {
		instance.Labels[corev1alpha1.RatingComponentLabel] = instance.Spec.ComponentName
		updateLabel = true
	}

	component := corev1alpha1.Component{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.ComponentName}, &component); err != nil {
		return false, err
	}
	if v, ok := instance.Labels[corev1alpha1.RatingRepositoryLabel]; !ok || v != component.Labels[corev1alpha1.ComponentRepositoryLabel] {
		instance.Labels[corev1alpha1.RatingRepositoryLabel] = component.Labels[corev1alpha1.ComponentRepositoryLabel]
		updateLabel = true
	}
	if updateLabel {
		return false, r.Client.Update(ctx, instance)
	}

	// add other checker
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RatingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := log.FromContext(context.TODO())
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Rating{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(ue event.UpdateEvent) bool {
				oldRating := ue.ObjectOld.(*corev1alpha1.Rating)
				newRating := ue.ObjectNew.(*corev1alpha1.Rating)
				// Spec can't be updated
				if !reflect.DeepEqual(oldRating.Spec, newRating.Spec) {
					return false
				}
				if !reflect.DeepEqual(oldRating.Status, newRating.Status) {
					return false
				}
				return true
			},
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
		})).Watches(&source.Kind{Type: &v1beta1.PipelineRun{}}, handler.Funcs{
		UpdateFunc: corev1alpha1.PipelineRunUpdate(r.Client, logger),
	}).Complete(r)
}
