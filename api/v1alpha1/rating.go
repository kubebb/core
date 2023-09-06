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
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	RatingComponentLabel  = "rating.component"
	RatingRepositoryLabel = "rating.repository"

	PipelineRun2RatingLabel     = "rating.pipelinerun"
	PipelineRun2ComponentLabel  = "rating.pipelinerun.component"
	PipelineRun2RepositoryLabel = "rating.pipelinerun.repository"
	PipelineRunDimensionLabel   = Group + "/dimension"
)

func PipelineRunName(ratingName, dimension string) string {
	return fmt.Sprintf("%s.%s", ratingName, dimension)
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

func GetPipelineRunDimension(pipelinerun *v1beta1.PipelineRun) string {
	if pipelinerun.Labels != nil {
		return pipelinerun.Labels[PipelineRunDimensionLabel]
	}
	return ""
}

func (rating Rating) GetPipelineRunStatus(dimensionLabel string) PipelineRunStatus {
	status := rating.Status.PipelineRuns[dimensionLabel]
	if status.Tasks == nil {
		status.Tasks = make([]Task, 0)
	}
	if status.ConditionedStatus.Conditions == nil {
		status.ConditionedStatus.Conditions = make([]Condition, 0)
	}
	return status
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

func WhenRunningOrSucceeded(pipelinerun *v1beta1.PipelineRun, instance *Rating, ratingState string) {
	dimension := GetPipelineRunDimension(pipelinerun)
	if dimension == "" {
		return
	}

	pipelineSpec := pipelinerun.Status.PipelineSpec
	taskState := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	task2taskRun := make(map[string]string)
	for taskRunName, tr := range pipelinerun.Status.TaskRuns {
		taskState[tr.PipelineTaskName] = tr
		task2taskRun[tr.PipelineTaskName] = taskRunName
	}

	pipelineStatus := instance.GetPipelineRunStatus(dimension)
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
		instance.Status.PipelineRuns[dimension] = pipelineStatus
	}
}
