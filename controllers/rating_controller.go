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
	arcadiav1 "github.com/kubeagi/arcadia/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
	"github.com/kubebb/core/pkg/evaluator"
	"github.com/kubebb/core/pkg/utils"
)

// RatingReconciler reconciles a Rating object
type RatingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=ratings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=ratings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubebb.k8s.com.cn,resources=ratings/finalizers,verbs=update

//+kubebuilder:rbac:groups=arcadia.kubeagi.k8s.com.cn,resources=llms,verbs=get;list;watch
//+kubebuilder:rbac:groups=arcadia.kubeagi.k8s.com.cn,resources=llms/status,verbs=get
//+kubebuilder:rbac:groups=arcadia.kubeagi.k8s.com.cn,resources=prompts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=arcadia.kubeagi.k8s.com.cn,resources=prompts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=arcadia.kubeagi.k8s.com.cn,resources=prompts/finalizers,verbs=update

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

	if err := r.CreatePipelineRun(logger, ctx, &instance); err != nil {
		logger.Error(err, "")
		return reconcile.Result{}, err
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

func (r *RatingReconciler) CreatePipelineRun(logger logr.Logger, ctx context.Context, instance *corev1alpha1.Rating) error {
	if err := r.DeletePipeline(ctx, instance); err != nil {
		return err
	}
	component := instance.Labels[corev1alpha1.RatingComponentLabel]
	repository := instance.Labels[corev1alpha1.RatingRepositoryLabel]
	namespace, err := utils.GetNamespace()
	if err != nil {
		return err
	}
	nextCreate := make([]v1beta1.PipelineRun, len(instance.Spec.PipelineParams))
	pipelineRunStatus := make(map[string]corev1alpha1.PipelineRunStatus)

	for idx, pipelineDef := range instance.Spec.PipelineParams {
		if _, ok := pipelineRunStatus[pipelineDef.Dimension]; ok {
			logger.Error(fmt.Errorf("repeatedly defined pipeline %s", pipelineDef.PipelineName), "")
			continue
		}
		pipeline := v1beta1.Pipeline{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: pipelineDef.PipelineName, Namespace: namespace}, &pipeline); err != nil {
			return err
		}
		pipelineRunName := corev1alpha1.PipelineRunName(instance.Name, pipelineDef.Dimension)

		pipelineRunStatus[pipelineDef.Dimension] = corev1alpha1.PipelineRunStatus{
			PipelineRunName: pipelineRunName,
			PipelineName:    pipelineDef.PipelineName,
		}

		ppr := v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: instance.GetNamespace(),
				Name:      pipelineRunName,
				Labels: map[string]string{
					corev1alpha1.PipelineRun2RatingLabel:     instance.Name,
					corev1alpha1.PipelineRun2ComponentLabel:  component,
					corev1alpha1.PipelineRun2RepositoryLabel: repository,
					corev1alpha1.PipelineRunDimensionLabel:   pipelineDef.Dimension,
				},
			},
			Spec: v1beta1.PipelineRunSpec{
				ServiceAccountName: corev1alpha1.GetRatingServiceAccount(),
				PipelineRef: &v1beta1.PipelineRef{
					ResolverRef: v1beta1.ResolverRef{
						Resolver: "cluster",
						Params: []v1beta1.Param{
							{
								Name:  "kind",
								Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "pipeline"},
							},
							{
								Name:  "name",
								Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: pipelineDef.PipelineName},
							},
							{
								Name:  "namespace",
								Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: namespace},
							},
						},
					},
				},
				Params: corev1alpha1.Params2PipelinrunParams(pipelineDef.Params),
			},
		}
		nextCreate[idx] = ppr
	}

	instanceDeepCopy := instance.DeepCopy()
	instanceDeepCopy.Status.PipelineRuns = pipelineRunStatus
	if err := r.Client.Status().Patch(ctx, instanceDeepCopy, client.MergeFrom(instance)); err != nil {
		return err
	}

	for idx := range nextCreate {
		_ = controllerutil.SetOwnerReference(instance, &nextCreate[idx], r.Scheme)
		if err := r.Client.Create(ctx, &nextCreate[idx]); err != nil {
			for i := idx - 1; i >= 0; i-- {
				_ = r.Client.Delete(ctx, &nextCreate[idx])
			}
			return err
		}
	}

	return nil
}

// Before creating pipelinerun, we shoulde delete all the existing pipelineruns.
func (r *RatingReconciler) DeletePipeline(ctx context.Context, instance *corev1alpha1.Rating) error {
	for _, pipelineDef := range instance.Spec.PipelineParams {
		name := pipelineDef.PipelineName
		pipelineRun := v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      corev1alpha1.PipelineRunName(instance.Name, name),
			},
		}
		if err := r.Client.Delete(ctx, &pipelineRun); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *RatingReconciler) PipelineRunUpdate(logger logr.Logger) func(event.UpdateEvent, workqueue.RateLimitingInterface) {
	return func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
		pipelinerun := ue.ObjectNew.(*v1beta1.PipelineRun)
		ratingName := ""
		for _, own := range pipelinerun.OwnerReferences {
			if own.Kind == "Rating" {
				ratingName = own.Name
				break
			}
		}
		if ratingName == "" {
			ratingName = pipelinerun.Labels[corev1alpha1.PipelineRun2RatingLabel]
			if ratingName == "" {
				logger.Info(fmt.Sprintf("unable to find the Rating resource associated with the pipelinerune: %s", pipelinerun.Name))
				return
			}
		}

		conditions := pipelinerun.Status.Conditions
		if len(conditions) == 0 {
			logger.Info(fmt.Sprintf("pipelinerun %s currently does not have any conditions, waiting for conditions", pipelinerun.Name))
			return
		}
		rating := &corev1alpha1.Rating{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: pipelinerun.Namespace, Name: ratingName}, rating); err != nil {
			logger.Error(err, "")
			return
		}

		deepCopyRating := rating.DeepCopy()
		var curCond apis.Condition

		for _, cond := range conditions {
			if cond.Type == apis.ConditionSucceeded {
				curCond = cond
			}
		}

		dimension := corev1alpha1.GetPipelineRunDimension(pipelinerun)

		pipelineRunStatus := deepCopyRating.Status.PipelineRuns[dimension]
		pipelineRunStatus.Conditions = corev1alpha1.ConvertPipelineRunCondition(pipelinerun)
		deepCopyRating.Status.PipelineRuns[dimension] = pipelineRunStatus

		if curCond.Reason == string(corev1alpha1.RatingResolvingPipelineRef) || curCond.Reason == string(corev1alpha1.RatingResolvingTaskRef) {
			logger.Info(fmt.Sprintf("pipelinerun %s currently in {reason: %v, status: %v, msg: %s}, please wait a moment...", pipelinerun.Name, curCond.Reason, curCond.Status, curCond.Message))
		} else if curCond.Reason == string(corev1alpha1.RatingRunning) || curCond.Reason == string(corev1alpha1.RatingSucceeded) {
			corev1alpha1.WhenRunningOrSucceeded(pipelinerun, deepCopyRating, curCond.Reason)
			logger.Info(fmt.Sprintf("pipelinerun %s currently in {reason: %v, status: %v, msg: %s}",
				pipelinerun.Name, curCond.Reason, curCond.Status, curCond.Message))
		}

		// When pipelinerun succeeded and llm is set,evaluate this Rating status
		if curCond.Reason == string(corev1alpha1.RatingSucceeded) && rating.Spec.LLM != "" {
			arcEval, err := evaluator.NewEvaluator(logger, context.TODO(), r.Client, r.Scheme, types.NamespacedName{Namespace: rating.Namespace, Name: string(rating.Spec.Evaluator.LLM)})
			if err != nil {
				logger.Error(err, "failed to create arcadia evaluator")
			} else {
				data := &evaluator.Data{
					Owner:     rating.DeepCopy(),
					FromRun:   pipelinerun.Name,
					Dimension: dimension,
					Tasks:     deepCopyRating.GetPipelineRunStatus(dimension).Tasks,
				}
				err = arcEval.EvaluateWithData(context.TODO(), data)
				if err != nil {
					logger.Error(err, "")
				}
			}
		}

		if err := r.Client.Status().Patch(context.TODO(), deepCopyRating, client.MergeFrom(rating)); err != nil {
			logger.Error(err, "")
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RatingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := log.FromContext(context.TODO())
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Rating{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(ue event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
		})).
		Watches(
			&source.Kind{
				Type: &v1beta1.PipelineRun{},
			}, handler.Funcs{
				UpdateFunc: r.PipelineRunUpdate(logger),
			}).
		Watches(
			&source.Kind{
				Type: &arcadiav1.Prompt{},
			}, handler.Funcs{
				UpdateFunc: evaluator.OnPromptUpdate(logger, r.Client),
			}).
		Complete(r)
}
