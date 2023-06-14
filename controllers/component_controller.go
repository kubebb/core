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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Component object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Component")

	// Fetch the Component instance
	instance := &corev1alpha1.Component{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !instance.DeletionTimestamp.IsZero() {
		// The object is being deleted.
		logger.Info("Component is being deleted")
	}

	done, err := r.UpdateComponent(ctx, logger, instance)
	if err != nil {
		return reconcile.Result{}, err
	} else if !done {
		return reconcile.Result{}, nil
	}

	logger.Info("Synchronized component successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Component{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: r.OnComponentUpdate,
		})).
		Complete(r)
}

// UpdateComponent updates new component, add finalizer if necessary.
func (r *ComponentReconciler) UpdateComponent(ctx context.Context, logger logr.Logger, instance *corev1alpha1.Component) (bool, error) {
	done := true

	var repoName string
	// check if ownerReferences exist, report done (nothing to do) if it doesn't.
	for _, owner := range instance.OwnerReferences {
		if owner.Kind == "Repository" {
			repoName = owner.Name
			break
		}
	}
	if repoName == "" {
		return done, nil
	}

	if instance.Labels == nil {
		instance.Labels = make(map[string]string)
	}
	// check label, report not done (need another update event)  if it doesn't exist or not equal to the name of the repository.
	if v, ok := instance.Labels[corev1alpha1.ComponentRepositoryLabel]; !ok || v != repoName {
		// add component.repository=<repository-name> to labels
		done = false
		instance.Labels[corev1alpha1.ComponentRepositoryLabel] = repoName
		logger.V(4).Info("Component repository label added", "Label", corev1alpha1.ComponentRepositoryLabel)
		err := r.Client.Update(ctx, instance)
		if err != nil {
			logger.Error(err, "Failed to add component repository label")
		}
		return done, err
	}

	return done, nil
}

// OnComponentUpdate checks if a reconcile process is needed when updating. Default true.
func (r *ComponentReconciler) OnComponentUpdate(event event.UpdateEvent) bool {
	oldObj := event.ObjectOld.(*corev1alpha1.Component)
	newObj := event.ObjectNew.(*corev1alpha1.Component)

	return oldObj.ResourceVersion != newObj.ResourceVersion || !reflect.DeepEqual(oldObj.Status, newObj.Status)
}
