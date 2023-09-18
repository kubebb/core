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
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/env"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/helm"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=components/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
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
		return reconcile.Result{}, nil
	}

	done, err := r.UpdateComponent(ctx, logger, instance)
	if err != nil {
		return reconcile.Result{}, err
	} else if !done {
		return reconcile.Result{}, nil
	}
	if name, ok := instance.Labels[corev1alpha1.ComponentRepositoryLabel]; ok {
		repo := &corev1alpha1.Repository{}
		if err = r.Client.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: name}, repo); err != nil {
			logger.Error(err, "")
			return reconcile.Result{}, err
		}
		if err := r.UpdateValuesConfigmap(ctx, logger, instance, repo); err != nil {
			return reconcile.Result{}, err
		}
	}

	logger.Info("Synchronized component successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Component{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: r.OnComponentUpdate,
			CreateFunc: r.OnComponentCreate,
			DeleteFunc: r.OnComponentDel,
		})).
		Complete(r)
}

// UpdateComponent updates new component, add finalizer if necessary.
func (r *ComponentReconciler) UpdateComponent(ctx context.Context, logger logr.Logger, instance *corev1alpha1.Component) (bool, error) {
	var repoName string
	// check if ownerReferences exist, report done (nothing to do) if it doesn't.
	for _, owner := range instance.OwnerReferences {
		if owner.Kind == "Repository" {
			repoName = owner.Name
			break
		}
	}
	if repoName == "" {
		return true, nil
	}

	if instance.Labels == nil {
		instance.Labels = make(map[string]string)
	}
	// check label, report not done (need another update event) if it doesn't exist or not equal to the name of the repository.
	if v, ok := instance.Labels[corev1alpha1.ComponentRepositoryLabel]; !ok || v != repoName {
		// add component.repository=<repository-name> to labels
		instance.Labels[corev1alpha1.ComponentRepositoryLabel] = repoName
		logger.V(1).Info("Component repository label added", "Label", corev1alpha1.ComponentRepositoryLabel)
		err := r.Client.Update(ctx, instance)
		if err != nil {
			logger.Error(err, "Failed to add component repository label")
		}
		return false, err
	}

	return true, nil
}

// OnComponentUpdate checks if a reconcile process is needed when updating. Default true.
func (r *ComponentReconciler) OnComponentUpdate(event event.UpdateEvent) bool {
	oldObj := event.ObjectOld.(*corev1alpha1.Component)
	newObj := event.ObjectNew.(*corev1alpha1.Component)
	added, deleted, deprecated := corev1alpha1.ComponentVersionDiff(*oldObj, *newObj)
	if len(added) > 0 || len(deleted) > 0 || len(deprecated) > 0 {
		r.Recorder.Event(newObj, v1.EventTypeNormal, "Update",
			fmt.Sprintf(corev1alpha1.UpdateEventMsgTemplate, newObj.GetName(), len(added), len(deleted), len(deprecated)))
	}
	return oldObj.ResourceVersion != newObj.ResourceVersion || !reflect.DeepEqual(oldObj.Status, newObj.Status)
}

func (r *ComponentReconciler) OnComponentCreate(event event.CreateEvent) bool {
	o := event.Object.(*corev1alpha1.Component)
	r.Recorder.Event(o, v1.EventTypeNormal, "Create", fmt.Sprintf(corev1alpha1.AddEventMsgTemplate, o.GetName()))
	return true
}

func (r *ComponentReconciler) OnComponentDel(event event.DeleteEvent) bool {
	o := event.Object.(*corev1alpha1.Component)
	r.Recorder.Event(o, v1.EventTypeNormal, "Delete", fmt.Sprintf(corev1alpha1.DelEventMsgTemplate, o.GetName()))
	return true
}

func (r *ComponentReconciler) UpdateValuesConfigmap(ctx context.Context, logger logr.Logger, component *corev1alpha1.Component, repo *corev1alpha1.Repository) (err error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get config for in-cluster REST client: %w", err)
	}
	getter := &genericclioptions.ConfigFlags{
		APIServer:   &cfg.Host,
		CAFile:      &cfg.CAFile,
		BearerToken: &cfg.BearerToken,
		Namespace:   &component.Namespace,
	}
	g := new(errgroup.Group)
	workers, err := env.GetInt("OCI_PULL_WORKER", 5) // Increase this number will download faster, but also more likely to trigger '429 Too Many Requests' error.
	if err != nil {
		workers = 5
	}
	g.SetLimit(workers)
	for _, version := range component.Status.Versions {
		versionStr := version.Version // https://golang.org/doc/faq#closures_and_goroutines
		httpDonwloadURLs := version.URLs
		g.Go(func() error {
			cm := &v1.ConfigMap{}
			cm.Name = corev1alpha1.GetComponentChartValuesConfigmapName(component.Name, versionStr)
			cm.Namespace = component.Namespace
			err := r.Client.Get(ctx, client.ObjectKeyFromObject(cm), cm)
			createCm := false
			if err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
				createCm = true
			}

			_, ok1 := cm.Data[corev1alpha1.ValuesConfigMapKey]
			_, ok2 := cm.Data[corev1alpha1.ImagesConfigMapKey]
			if ok1 && ok2 {
				return nil
			}

			if cm.Data == nil {
				cm.Data = make(map[string]string)
			}
			var pullURL string
			u := strings.TrimSuffix(repo.Spec.URL, "/")
			if repo.IsOCI() {
				if v, ok := component.Annotations[corev1alpha1.OCIPullURLAnnotation]; ok {
					pullURL = v
				} else {
					pullURL = u + "/" + component.Status.Name
				}
			} else {
				if len(httpDonwloadURLs) == 0 {
					logger.Error(fmt.Errorf("not found %s's urls", component.Status.Name), "")
					return nil
				}
				if strings.HasPrefix(httpDonwloadURLs[0], "http") {
					pullURL = httpDonwloadURLs[0]
				} else {
					// chartmuseum charts/weaviate-16.3.0.tgz
					// github https://github.com/kubebb/components/releases/download/bc-apis-0.0.3/bc-apis-0.0.3.tgz
					pullURL = u + "/" + httpDonwloadURLs[0]
				}
			}

			h, err := helm.NewCoreHelmWrapper(getter, component.Namespace, logger, r.Client, nil, repo, component)
			if err != nil {
				return err
			}
			_, dir, entryName, err := h.Pull(ctx, pullURL, versionStr)
			if err != nil {
				return err
			}
			defer os.Remove(dir)
			b, err := os.ReadFile(dir + "/" + entryName + "/values.yaml")
			if err != nil {
				return err
			}
			cm.Data[corev1alpha1.ValuesConfigMapKey] = string(b)
			rel, err := h.Template(ctx, versionStr, dir+"/"+entryName)
			if err != nil {
				return err
			}
			_, images, err := corev1alpha1.GetResourcesAndImages(ctx, logger, r.Client, rel.Manifest, component.Namespace)
			if err != nil {
				return err
			}
			cm.Data[corev1alpha1.ImagesConfigMapKey] = strings.Join(images, ",")
			if createCm {
				return r.Client.Create(ctx, cm)
			}
			return r.Update(ctx, cm)
		})
	}
	return g.Wait()
}
