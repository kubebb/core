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
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubebb/core/api/v1alpha1"
	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/repository"
	"github.com/kubebb/core/pkg/utils"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

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
		repo.Finalizers = utils.RemoveString(repo.Finalizers, corev1alpha1.Finalizer)
		err := r.Client.Update(ctx, repo)
		if err != nil {
			logger.Error(err, "Failed to remove repo finalizer")
		}
		return reconcile.Result{}, err
	}

	done, err := r.UpdateRepository(ctx, logger, repo)
	if !done {
		return reconcile.Result{}, err
	}

	w, ok := r.C[key]
	if ok {
		logger.Info("Repository update, stop and recreate goroutine")
		w.Stop()
	}
	_ctx, _cancel := context.WithCancel(ctx)
	r.C[key] = repository.NewChartmuseum(_ctx, logger, r.Client, r.Scheme, repo, _cancel)
	r.C[key].Start()

	logger.Info("Synchronized repository successfully")
	return ctrl.Result{}, nil
}

func (r *RepositoryReconciler) UpdateRepository(ctx context.Context, logger logr.Logger, instance *corev1alpha1.Repository) (bool, error) {
	instanceDeepCopy := instance.DeepCopy()
	l := len(instanceDeepCopy.Finalizers)

	instanceDeepCopy.Finalizers = utils.AddString(instanceDeepCopy.Finalizers, corev1alpha1.Finalizer)
	if l != len(instanceDeepCopy.Finalizers) {
		logger.V(1).Info("Add Finalizer for repository", "Finalizer", corev1alpha1.Finalizer)
		err := r.Client.Update(ctx, instanceDeepCopy)
		if err != nil {
			logger.Error(err, "")
		}
		return false, err
	}

	cond := corev1alpha1.Condition{
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             corev1alpha1.ReasonCreating,
		Message:            "prepare to launch watcher",
		Type:               corev1alpha1.TypeReady,
	}

	if changed, history := utils.AddOrSwapString(instanceDeepCopy.Status.URLHistory, instanceDeepCopy.Spec.URL); changed {
		logger.V(1).Info("Add URL for repository", "URL", instance.Spec.URL)
		instanceDeepCopy.Status.URLHistory = history
		instanceDeepCopy.Status.ConditionedStatus = v1alpha1.ConditionedStatus{
			Conditions: []v1alpha1.Condition{cond},
		}
		err := r.Client.Status().Patch(ctx, instanceDeepCopy, client.MergeFrom(instance))
		if err != nil {
			logger.Error(err, "")
		}
		return false, err
	}
	if instanceDeepCopy.Labels == nil {
		instanceDeepCopy.Labels = make(map[string]string)
	}
	if v, ok := instanceDeepCopy.Labels[v1alpha1.RepositoryTypeLabel]; !ok || v != instanceDeepCopy.Spec.RepositoryType {
		instanceDeepCopy.Labels[v1alpha1.RepositoryTypeLabel] = instanceDeepCopy.Spec.RepositoryType
		err := r.Client.Update(ctx, instanceDeepCopy)
		if err != nil {
			logger.Error(err, "")
		}
		return false, err
	}

	return true, nil
}

func (r *RepositoryReconciler) OnRepositryUpdate(u event.UpdateEvent) bool {
	oldRepo := u.ObjectOld.(*corev1alpha1.Repository)
	newRepo := u.ObjectNew.(*corev1alpha1.Repository)

	buf := strings.Builder{}
	if oldRepo.Spec.URL != newRepo.Spec.URL {
		buf.WriteString(fmt.Sprintf(" 'url' changes from %s to %s.", oldRepo.Spec.URL, newRepo.Spec.URL))
	}
	if oldRepo.Spec.AuthSecret != newRepo.Spec.AuthSecret {
		buf.WriteString(fmt.Sprintf(" 'authSecret' changes from %s to %s.", oldRepo.Spec.AuthSecret, newRepo.Spec.AuthSecret))
	}
	if str := buf.String(); len(str) > 0 {
		r.Recorder.Event(newRepo, v1.EventTypeNormal, "Update", str)
	}

	return oldRepo.Spec.URL != newRepo.Spec.URL ||
		oldRepo.Spec.AuthSecret != newRepo.Spec.AuthSecret ||
		!corev1alpha1.IsPullStrategySame(oldRepo.Spec.PullStategy, newRepo.Spec.PullStategy) ||
		!reflect.DeepEqual(oldRepo.Status.URLHistory, newRepo.Status.URLHistory) ||
		len(oldRepo.Finalizers) != len(newRepo.Finalizers) ||
		newRepo.DeletionTimestamp != nil ||
		!reflect.DeepEqual(oldRepo.Spec.Filter, newRepo.Spec.Filter) ||
		!reflect.DeepEqual(oldRepo.Labels, newRepo.Labels)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Repository{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(ce event.CreateEvent) bool {
				obj := ce.Object.(*v1alpha1.Repository)
				r.Recorder.Eventf(obj, v1.EventTypeNormal, "Created", "add new repository %s", obj.GetName())
				return true
			},
			UpdateFunc: r.OnRepositryUpdate,
			DeleteFunc: func(de event.DeleteEvent) bool {
				obj := de.Object.(*v1alpha1.Repository)
				r.Recorder.Eventf(obj, v1.EventTypeNormal, "Deleted", "delete repository %s", obj.GetName())
				return true
			},
		})).
		Complete(r)
}
