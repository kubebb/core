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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/kubebb/core/pkg/utils"
)

const (
	RatingComponentLabel  = "rating.component"
	RatingRepositoryLabel = "rating.repository"

	PipelineRun2RatingLabel     = "rating.pipelinerun"
	PipelineRun2ComponentLabel  = "rating.pipelinerun.component"
	PipelineRun2RepositoryLabel = "rating.pipelinerun.repository"
)

func PipelineRunName(ratingName, pipelineName string) string {
	return fmt.Sprintf("%s.%s", ratingName, pipelineName)
}

func IsTaskSame(a, b Task) bool {
	return a.Name == b.Name && a.TaskRunName == b.TaskRunName && a.Type == b.Type
}

func AppendTask(tasks *[]Task, task Task) {
	for idx, t := range *tasks {
		if IsTaskSame(t, task) {
			(*tasks)[idx].Conditions = task.Conditions
			return
		}
	}
	*tasks = append(*tasks, task)
}

func GetPipelineName(pipelinerun *v1beta1.PipelineRun) string {
	if pipelinerun.Spec.PipelineRef.Name != "" {
		return pipelinerun.Spec.PipelineRef.Name
	}
	if pipelinerun.Spec.PipelineRef.Resolver == "cluster" {
		for _, v := range pipelinerun.Spec.PipelineRef.Params {
			if v.Name == "name" {
				return v.Value.StringVal
			}
		}
	}
	return ""
}

func Params2PipelinrunParams(params []Param) []v1beta1.Param {
	result := make([]v1beta1.Param, len(params))
	for idx, param := range params {
		p := v1beta1.Param{Name: param.Name}
		if param.Value.Type == ParamTypeArray {
			p.Value.ArrayVal = param.Value.ArrayVal
			p.Value.Type = v1beta1.ParamType(param.Value.Type)
		}
		if param.Value.Type == ParamTypeString {
			p.Value.StringVal = param.Value.StringVal
			p.Value.Type = v1beta1.ParamType(param.Value.Type)
		}
		if param.Value.Type == ParamTypeObject {
			p.Value.Type = v1beta1.ParamType(param.Value.Type)

			p.Value.ObjectVal = param.Value.ObjectVal
		}
		result[idx] = p
	}
	return result
}

func ConvertPipelineRunCondition(pipelinerun *v1beta1.PipelineRun) []Condition {
	var curCond apis.Condition

	now := metav1.Now()
	for _, cond := range pipelinerun.Status.Conditions {
		if cond.Type == apis.ConditionSucceeded {
			curCond = cond
		}
	}

	return []Condition{{
		Type:               ConditionType(curCond.Type),
		Status:             curCond.Status,
		Reason:             ConditionReason(curCond.Reason),
		Message:            curCond.Message,
		LastTransitionTime: now,
	}}
}

// Before creating pipelinerun, we shoulde delete all the existing pipelineruns.
func DeletePieline(ctx context.Context, c client.Client, instance *Rating) error {
	for _, pipelineDef := range instance.Spec.PipelineParams {
		name := pipelineDef.PipelineName
		pipelineRun := v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      PipelineRunName(instance.Name, name),
			},
		}
		if err := c.Delete(ctx, &pipelineRun); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func CreatePipelineRun(ctx context.Context, c client.Client, scheme *runtime.Scheme, instance *Rating, logger logr.Logger) error {
	if err := DeletePieline(ctx, c, instance); err != nil {
		return err
	}
	component := instance.Labels[RatingComponentLabel]
	repository := instance.Labels[RatingRepositoryLabel]
	namespace, err := utils.GetNamespace()
	if err != nil {
		return err
	}
	nextCreate := make([]v1beta1.PipelineRun, len(instance.Spec.PipelineParams))
	pipelineRunStatus := make(map[string]PipelineRunStatus)

	for idx, pipelineDef := range instance.Spec.PipelineParams {
		pipelineRunName := PipelineRunName(instance.Name, pipelineDef.PipelineName)
		if _, ok := pipelineRunStatus[pipelineRunName]; ok {
			logger.Error(fmt.Errorf("repeatedly defined pipeline %s", pipelineDef.PipelineName), "")
			continue
		}

		pipeline := v1beta1.Pipeline{}
		if err := c.Get(ctx, types.NamespacedName{Name: pipelineDef.PipelineName, Namespace: namespace}, &pipeline); err != nil {
			return err
		}
		pipelineRunStatus[pipelineRunName] = PipelineRunStatus{ExpectWeight: len(pipeline.Spec.Tasks), PipelineName: pipelineDef.PipelineName}

		ppr := v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: instance.GetNamespace(),
				Name:      pipelineRunName,
				Labels: map[string]string{
					PipelineRun2RatingLabel:     instance.Name,
					PipelineRun2ComponentLabel:  component,
					PipelineRun2RepositoryLabel: repository,
				},
			},
			Spec: v1beta1.PipelineRunSpec{
				ServiceAccountName: GetRatingServiceAccount(),
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
				Params: Params2PipelinrunParams(pipelineDef.Params),
			},
		}
		nextCreate[idx] = ppr
	}

	instanceDeepCopy := instance.DeepCopy()
	instanceDeepCopy.Status.PipelineRuns = pipelineRunStatus
	if err := c.Status().Patch(ctx, instanceDeepCopy, client.MergeFrom(instance)); err != nil {
		return err
	}

	for idx := range nextCreate {
		_ = controllerutil.SetOwnerReference(instance, &nextCreate[idx], scheme)
		if err := c.Create(ctx, &nextCreate[idx]); err != nil {
			for i := idx - 1; i >= 0; i-- {
				_ = c.Delete(ctx, &nextCreate[idx])
			}
			return err
		}
	}

	return nil
}

func WhenRunningOrSucceeded(pipelinerun *v1beta1.PipelineRun, instance *Rating, ratingState string) {
	pipelineSpec := pipelinerun.Status.PipelineSpec
	taskState := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	task2taskRun := make(map[string]string)
	for taskRunName, tr := range pipelinerun.Status.TaskRuns {
		taskState[tr.PipelineTaskName] = tr
		task2taskRun[tr.PipelineTaskName] = taskRunName
	}

	pipelineStatus := instance.Status.PipelineRuns[pipelinerun.Name]
	if pipelineSpec != nil {
		now := metav1.Now()
		for _, task := range pipelineSpec.Tasks {
			t := Task{Name: task.Name}
			/*
			   1. task.Conditions == 0 means the task is not yet running
			   2. reason=Succeeded && status=true means the task ran successfully
			   3. reason=Succeeded && status=false  means that the task failed to run.
			   4. reason=Running means the task is running
			*/
			if taskRun, ok := taskState[task.Name]; ok {
				t.TaskRunName = task2taskRun[task.Name]
				for _, cond := range taskRun.Status.Conditions {
					if cond.Type == apis.ConditionSucceeded {
						tc := Condition{
							Type:               ConditionType(cond.Type),
							Status:             cond.Status,
							Reason:             ConditionReason(cond.Reason),
							Message:            cond.Message,
							LastTransitionTime: now,
						}
						t.Conditions = []Condition{tc}
						break
					}
				}
			}
			AppendTask(&pipelineStatus.Tasks, t)
		}
		pipelineStatus.ActualWeight = 0
		for _, t := range pipelineStatus.Tasks {
			if len(t.Conditions) == 0 {
				continue
			}

			cond := t.Conditions[0]
			if cond.Reason == RatingSucceeded && cond.Status == corev1.ConditionTrue {
				pipelineStatus.ActualWeight++
			}
		}
		instance.Status.PipelineRuns[pipelinerun.Name] = pipelineStatus
	}
}

func PipelineRunUpdate(c client.Client, logger logr.Logger) func(event.UpdateEvent, workqueue.RateLimitingInterface) {
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
			ratingName = pipelinerun.Labels[PipelineRun2RatingLabel]
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
		rating := &Rating{}
		if err := c.Get(context.TODO(), types.NamespacedName{Namespace: pipelinerun.Namespace, Name: ratingName}, rating); err != nil {
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
		pipelineRunStatus := deepCopyRating.Status.PipelineRuns[pipelinerun.Name]
		pipelineRunStatus.Conditions = ConvertPipelineRunCondition(pipelinerun)
		deepCopyRating.Status.PipelineRuns[pipelinerun.Name] = pipelineRunStatus

		if curCond.Reason == string(RatingResolvingPipelineRef) || curCond.Reason == string(RatingResolvingTaskRef) {
			logger.Info(fmt.Sprintf("pipelinerun %s currently in {reason: %v, status: %v, msg: %s}, please wait a moment...", pipelinerun.Name, curCond.Reason, curCond.Status, curCond.Message))
		} else if curCond.Reason == string(RatingRunning) || curCond.Reason == string(RatingSucceeded) {
			WhenRunningOrSucceeded(pipelinerun, deepCopyRating, curCond.Reason)
			logger.Info(fmt.Sprintf("pipelinerun %s currently in {reason: %v, status: %v, msg: %s}, expectWeight: %d, actualWeight: %d",
				pipelinerun.Name, curCond.Reason, curCond.Status, curCond.Message, deepCopyRating.Status.ExpectWeight, deepCopyRating.Status.ActualWeight))
		}

		if err := c.Status().Patch(context.TODO(), deepCopyRating, client.MergeFrom(rating)); err != nil {
			logger.Error(err, "")
		}
	}
}
