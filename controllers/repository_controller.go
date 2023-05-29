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
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/repository"
	"github.com/kubebb/core/pkg/utils"
)

// Fixed, no lower configuration allowed
const (
	statusLen = 2
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	C map[string]repository.IWatcher
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=repositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=repositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=repositories/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Repository object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting repository reconcile")

	repo := &corev1alpha1.Repository{}
	if err := r.Client.Get(ctx, req.NamespacedName, repo); err != nil {
		if errors.IsNotFound(err) {
			// Repository has been deleted.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	key := fmt.Sprintf("%s/%s", repo.GetNamespace(), repo.GetName())
	if repo.DeletionTimestamp != nil {
		logger.Info("Delete repository")
		// since the Repository has been deleted, we first need to stop the associated goroutine.
		if w, ok := r.C[key]; ok {
			delete(r.C, key)
			w.Stop()
		}

		// remove the finalizer to complete the delete action
		repo.Finalizers = utils.RemoveString(repo.Finalizers, corev1alpha1.RepositoryFinalizer)
		err := r.Client.Update(ctx, repo)
		if err != nil {
			logger.Error(err, "Failed to remove repo finalizer")
		}
		return reconcile.Result{}, err
	}

	requeue, err := r.UpdateRepository(ctx, logger, repo)
	if requeue || err != nil {
		logger.Info("need requeue", "Requeue", requeue, "Err", err)
		return reconcile.Result{Requeue: requeue}, err
	}

	w, ok := r.C[key]
	if ok {
		logger.Info("Repository update, stop and recreate goroutine")
		w.Stop()
	}
	_ctx, _cancel := context.WithCancel(ctx)
	r.C[key] = repository.NewChartmuseum(_ctx, logger, r.Client, r.Scheme, repo, statusLen, _cancel)
	r.C[key].Start()

	logger.Info("Synchronized repository successfully")
	return ctrl.Result{}, nil
}

func (r *RepositoryReconciler) UpdateRepository(ctx context.Context, logger logr.Logger, instance *corev1alpha1.Repository) (bool, error) {
	requeue := false
	l := len(instance.Finalizers)
	instance.Finalizers = utils.AddString(instance.Finalizers, corev1alpha1.RepositoryFinalizer)
	if l != len(instance.Finalizers) {
		requeue = true
		logger.V(4).Info("Add Finalizer for repository", "Finalizer", corev1alpha1.RepositoryFinalizer)
		err := r.Client.Update(ctx, instance)
		if err != nil {
			logger.Error(err, "")
		}
		return requeue, err
	}

	l = len(instance.Status.URLHistory)
	cond := corev1alpha1.Condition{
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             corev1alpha1.ReasonCreating,
		Message:            "prepare to launch watcher",
		Type:               corev1alpha1.TypeReady,
	}
	instance.Status.URLHistory = utils.AddString(instance.Status.URLHistory, instance.Spec.URL)
	if len(instance.Status.URLHistory) != l {
		requeue = true
		logger.V(4).Info("Add URL for repository", "URL", instance.Spec.URL)
		corev1alpha1.UpdateCondWithFixedLen(statusLen, &instance.Status.ConditionedStatus, cond)
		err := r.Client.Status().Update(ctx, instance)
		if err != nil {
			logger.Error(err, "")
		}
		return requeue, err
	}

	return requeue, nil
}

func (r *RepositoryReconciler) OnRepositryUpdate(u event.UpdateEvent) bool {
	oldRepo := u.ObjectOld.(*corev1alpha1.Repository)
	newRepo := u.ObjectNew.(*corev1alpha1.Repository)

	return oldRepo.Spec.URL != newRepo.Spec.URL ||
		oldRepo.Spec.AuthSecret != newRepo.Spec.AuthSecret ||
		!corev1alpha1.IsPullStrategySame(oldRepo.Spec.PullStategy, newRepo.Spec.PullStategy) ||
		len(oldRepo.Status.URLHistory) != len(newRepo.Status.URLHistory) ||
		len(oldRepo.Finalizers) != len(newRepo.Finalizers) ||
		newRepo.DeletionTimestamp != nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Repository{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: r.OnRepositryUpdate,
		})).
		Complete(r)
}
