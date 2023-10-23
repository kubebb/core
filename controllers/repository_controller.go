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
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
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

	err := r.checkInitial(ctx, logger, repo)
	if err != nil {
		logger.Error(err, "Failed to check initiali settings")
		return reconcile.Result{}, err
	}

	if err := r.syncURL(ctx, logger, repo); err != nil {
		logger.Error(err, "Failed to sync url history")
		return reconcile.Result{}, err
	}

	if err := r.ensureRatingServiceAccount(ctx, repo.GetNamespace()); err != nil {
		logger.Error(err, "Failed to ensure rating service accounts")
		return reconcile.Result{}, err
	}

	if err := r.restart(ctx, logger, repo, key); err != nil {
		logger.Error(err, "Failed to start repository watcher")
		return reconcile.Result{}, err
	}

	logger.Info("Reconciled repository successfully")

	return reconcile.Result{}, nil
}

func (r *RepositoryReconciler) checkInitial(ctx context.Context, logger logr.Logger, instance *corev1alpha1.Repository) error {
	instanceDeepCopy := instance.DeepCopy()
	l := len(instanceDeepCopy.Finalizers)

	var update bool

	instanceDeepCopy.Finalizers = utils.AddString(instanceDeepCopy.Finalizers, corev1alpha1.Finalizer)
	if l != len(instanceDeepCopy.Finalizers) {
		logger.V(1).Info("Add Finalizer for repository", "Finalizer", corev1alpha1.Finalizer)
		update = true
	}

	if instanceDeepCopy.Labels == nil {
		instanceDeepCopy.Labels = make(map[string]string)
	}
	url := instanceDeepCopy.Spec.URL + "/api/charts"
	data := []byte(``)

	req, err := http.NewRequest("GET", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	if instanceDeepCopy.Spec.AuthSecret != "" {
		secret := v1.Secret{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.AuthSecret}, &secret); err != nil {
			return err
		}
		username := string(secret.Data["username"])
		password := string(secret.Data["password"])
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if v, ok := instanceDeepCopy.Labels[corev1alpha1.RepositoryTypeLabel]; !ok || v != instanceDeepCopy.Spec.RepositoryType {
		if resp.StatusCode == http.StatusOK {
			instanceDeepCopy.Labels[corev1alpha1.RepositoryTypeLabel] = "chartmuseum"
		}
		instanceDeepCopy.Labels[corev1alpha1.RepositoryTypeLabel] = instanceDeepCopy.Spec.RepositoryType
		update = true
	}
	if v, ok := instanceDeepCopy.Labels[corev1alpha1.RepositorySourceLabel]; !ok || (v != string(corev1alpha1.Official) && v != string(corev1alpha1.Unknown)) {
		instanceDeepCopy.Labels[corev1alpha1.RepositorySourceLabel] = string(corev1alpha1.Unknown)
		update = true
	}
	if update {
		err := r.Client.Update(ctx, instanceDeepCopy)
		if err != nil {
			logger.Error(err, "")
		}
		return err
	}

	return nil
}
func (r *RepositoryReconciler) syncURL(ctx context.Context, logger logr.Logger, repo *corev1alpha1.Repository) error {
	instanceDeepCopy := repo.DeepCopy()
	cond := corev1alpha1.Condition{
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             corev1alpha1.ReasonCreating,
		Message:            "prepare to launch watcher",
		Type:               corev1alpha1.TypeReady,
	}

	if changed, history := utils.AddOrSwapString(instanceDeepCopy.Status.URLHistory, instanceDeepCopy.Spec.URL); changed {
		logger.V(1).Info("Add URL for repository", "URL", repo.Spec.URL)
		instanceDeepCopy.Status.URLHistory = history
		instanceDeepCopy.Status.ConditionedStatus = corev1alpha1.ConditionedStatus{
			Conditions: []corev1alpha1.Condition{cond},
		}
		err := r.Client.Status().Patch(ctx, instanceDeepCopy, client.MergeFrom(repo))
		if err != nil {
			logger.Error(err, "")
		}
		return err
	}

	return nil
}

func (r *RepositoryReconciler) restart(ctx context.Context, logger logr.Logger, repo *corev1alpha1.Repository, key string) error {
	w, ok := r.C[key]
	if ok {
		logger.Info("Repository update, stop and recreate goroutine")
		w.Stop()
	}
	_ctx, _cancel := context.WithCancel(ctx)
	r.C[key] = repository.NewWatcher(_ctx, logger, r.Client, r.Scheme, repo, _cancel)
	if err := r.C[key].Start(); err != nil {
		r.Recorder.Event(repo, v1.EventTypeWarning, "StartFail", fmt.Sprintf("start %s fail", key))
		return err
	}
	return nil
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

	// only spec changes or lable `repository restart label` changes
	return !reflect.DeepEqual(oldRepo.Spec, newRepo.Spec) ||
		oldRepo.ObjectMeta.Labels[corev1alpha1.RepositoryRestartLabel] != newRepo.ObjectMeta.Labels[corev1alpha1.RepositoryRestartLabel] ||
		newRepo.DeletionTimestamp != nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	manager := ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Repository{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(ce event.CreateEvent) bool {
				obj := ce.Object.(*corev1alpha1.Repository)
				r.Recorder.Eventf(obj, v1.EventTypeNormal, "Created", "add new repository %s", obj.GetName())
				return true
			},
			UpdateFunc: r.OnRepositryUpdate,
			DeleteFunc: func(de event.DeleteEvent) bool {
				obj := de.Object.(*corev1alpha1.Repository)
				r.Recorder.Eventf(obj, v1.EventTypeNormal, "Deleted", "delete repository %s", obj.GetName())
				return true
			},
		})).Watches(&source.Kind{Type: &v1.ServiceAccount{}}, handler.Funcs{})

	if corev1alpha1.RatingEnabled() {
		manager = manager.Watches(&source.Kind{Type: &v1.ServiceAccount{}}, handler.Funcs{
			CreateFunc: func(ce event.CreateEvent, _ workqueue.RateLimitingInterface) {
				sa := ce.Object.(*v1.ServiceAccount)
				if sa.Name == corev1alpha1.GetRatingServiceAccount() {
					_ = corev1alpha1.AddSubjectToClusterRoleBinding(context.Background(), r.Client, sa.GetNamespace())
				}
			},
			UpdateFunc: func(ue event.UpdateEvent, _ workqueue.RateLimitingInterface) {
				sa := ue.ObjectNew.(*v1.ServiceAccount)
				if sa.Name == corev1alpha1.GetRatingServiceAccount() {
					_ = corev1alpha1.AddSubjectToClusterRoleBinding(context.Background(), r.Client, sa.GetNamespace())
				}
			},
			DeleteFunc: func(de event.DeleteEvent, _ workqueue.RateLimitingInterface) {
				sa := de.Object.(*v1.ServiceAccount)
				if sa.Name == corev1alpha1.GetRatingServiceAccount() {
					repoList := corev1alpha1.RepositoryList{}
					if err := r.List(context.Background(), &repoList, client.InNamespace(sa.GetNamespace())); err == nil {
						if len(repoList.Items) > 0 {
							_ = r.ensureRatingServiceAccount(context.Background(), sa.GetNamespace())
						} else {
							_ = corev1alpha1.RemoveSubjectFromClusterRoleBinding(context.Background(), r.Client, sa.GetNamespace())
						}
					}
				}
			},
		})
	}
	return manager.Complete(r)
}

func (r RepositoryReconciler) ensureRatingServiceAccount(ctx context.Context, namespace string) error {
	if !corev1alpha1.RatingEnabled() {
		return nil
	}

	ratingUsedServiceAccount := corev1alpha1.GetRatingServiceAccount()
	sa := v1.ServiceAccount{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ratingUsedServiceAccount}, &sa); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		sa.Name = ratingUsedServiceAccount
		sa.Namespace = namespace
		if err = r.Create(ctx, &sa); err != nil && !errors.IsConflict(err) {
			return err
		}
	}

	return nil
}
