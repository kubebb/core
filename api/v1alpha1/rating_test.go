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
	"reflect"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestIsTaskSame(t *testing.T) {
	type input struct {
		name   string
		a, b   Task
		expect bool
	}
	for _, tc := range []input{
		{
			name:   "name is different",
			a:      Task{Name: "a"},
			b:      Task{Name: "b"},
			expect: false,
		},
		{
			name:   "type is different",
			a:      Task{Name: "task", Type: "a"},
			b:      Task{Name: "task", Type: "b"},
			expect: false,
		},
		{
			name:   "taskRunName is different",
			a:      Task{Name: "task", TaskRunName: "a"},
			b:      Task{Name: "task", TaskRunName: "b"},
			expect: false,
		},
		{
			name:   "task is same",
			a:      Task{Name: "task", TaskRunName: "taskrun", Type: "type"},
			b:      Task{Name: "task", TaskRunName: "taskrun", Type: "type"},
			expect: true,
		},
	} {
		if r := IsTaskSame(tc.a, tc.b); r != tc.expect {
			t.Fatalf("Test Failed. %s expect %v get %v", tc.name, tc.expect, r)
		}
	}
}
func TestAppendTask(t *testing.T) {
	type input struct {
		name     string
		taskList []Task
		task     Task
		expect   []Task
	}
	for _, tc := range []input{
		{
			name:     "task is same, don't append anything",
			taskList: []Task{{Name: "task"}},
			task:     Task{Name: "task"},
			expect:   []Task{{Name: "task"}},
		},
		{
			name:     "task is not same, append new task",
			taskList: []Task{{Name: "task"}},
			task:     Task{Name: "task1"},
			expect:   []Task{{Name: "task"}, {Name: "task1"}},
		},
		{
			name:     "task is same, update condition",
			taskList: []Task{{Name: "task", ConditionedStatus: ConditionedStatus{Conditions: []Condition{}}}},
			task:     Task{Name: "task", ConditionedStatus: ConditionedStatus{Conditions: []Condition{{Type: TypeReady, Status: v1.ConditionTrue, Reason: "done", Message: "done"}}}},
			expect:   []Task{{Name: "task", ConditionedStatus: ConditionedStatus{Conditions: []Condition{{Type: TypeReady, Status: v1.ConditionTrue, Reason: "done", Message: "done"}}}}},
		},
	} {
		AppendTask(&tc.taskList, tc.task)
		if !reflect.DeepEqual(tc.taskList, tc.expect) {
			t.Fatalf("Test Failed. %s expect %v get %v", tc.name, tc.expect, tc.taskList)
		}
	}
}

func TestPipelineRunName(t *testing.T) {
	type input struct {
		a, b   string
		expect string
	}
	for _, tc := range []input{
		{a: "aa", b: "bb", expect: "aa.bb"},
	} {
		if r := PipelineRunName(tc.a, tc.b); r != tc.expect {
			t.Fatalf("Test Failed. expect %s get %s", tc.expect, r)
		}
	}
}

func TestGetPipelineName(t *testing.T) {
	type input struct {
		pipelinerun *v1beta1.PipelineRun
		expect      string
	}

	for _, tc := range []input{
		{
			pipelinerun: &v1beta1.PipelineRun{Spec: v1beta1.PipelineRunSpec{PipelineRef: &v1beta1.PipelineRef{Name: "pipelinename"}}},
			expect:      "pipelinename",
		},
		{
			pipelinerun: &v1beta1.PipelineRun{
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						ResolverRef: v1beta1.ResolverRef{
							Resolver: "cluster",
							Params: []v1beta1.Param{
								{
									Name:  "name",
									Value: *v1beta1.NewArrayOrString("pipelinename"),
								},
							},
						},
					},
				},
			},
			expect: "pipelinename",
		},
	} {
		if r := GetPipelineName(tc.pipelinerun); r != tc.expect {
			t.Fatalf("Test Failed. expect %s get %s", tc.expect, r)
		}
	}
}

func TestParams2PipelinerunParams(t *testing.T) {
	type input struct {
		params []Param
		expect []v1beta1.Param
	}
	for _, tc := range []input{
		{
			params: []Param{
				{
					Name:  "a",
					Value: ParamValue{Type: ParamTypeString, StringVal: "a"},
				},
			},
			expect: []v1beta1.Param{
				{
					Name:  "a",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "a"},
				},
			},
		},
		{
			params: []Param{
				{
					Name:  "a",
					Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"a"}},
				},
			},
			expect: []v1beta1.Param{
				{
					Name:  "a",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"a"}},
				},
			},
		},
		{
			params: []Param{
				{
					Name:  "a",
					Value: ParamValue{Type: ParamTypeObject, ObjectVal: map[string]string{"a": "a"}},
				},
			},
			expect: []v1beta1.Param{
				{
					Name:  "a",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeObject, ObjectVal: map[string]string{"a": "a"}},
				},
			},
		},
		{
			params: []Param{
				{
					Name:  "a",
					Value: ParamValue{Type: ParamTypeString, StringVal: "a"},
				},
				{
					Name:  "b",
					Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b"}},
				},
				{
					Name:  "c",
					Value: ParamValue{Type: ParamTypeObject, ObjectVal: map[string]string{"c": "c"}},
				},
			},
			expect: []v1beta1.Param{
				{
					Name:  "a",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "a"},
				},
				{
					Name:  "b",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"b"}},
				},
				{
					Name:  "c",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeObject, ObjectVal: map[string]string{"c": "c"}},
				},
			},
		},
	} {
		if r := Params2PipelinrunParams(tc.params); !reflect.DeepEqual(tc.expect, r) {
			t.Fatalf("Test Failed. expect %v get %v", tc.expect, r)
		}
	}
}

func TestConvertPipelineRunCondition(t *testing.T) {
	type input struct {
		pipelinerun *v1beta1.PipelineRun
		expect      []Condition
	}
	for _, tc := range []input{
		{
			pipelinerun: &v1beta1.PipelineRun{
				Status: v1beta1.PipelineRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{
							{Type: apis.ConditionSucceeded, Status: v1.ConditionTrue, Reason: "done", Message: "done"},
						},
					},
				},
			},
			expect: []Condition{
				{
					Type:    ConditionType(apis.ConditionSucceeded),
					Status:  v1.ConditionTrue,
					Reason:  "done",
					Message: "done",
				},
			},
		},
	} {
		r := ConvertPipelineRunCondition(tc.pipelinerun)
		if r[0].Type != tc.expect[0].Type || r[0].Status != tc.expect[0].Status || r[0].Reason != tc.expect[0].Reason || r[0].Message != tc.expect[0].Message {
			t.Fatalf("Test Failed. expect %v get %v", tc.expect, r)
		}
	}
}

const (
	pipelinerunRunning = `apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  labels:
    description: A_PipelineRun
    rating: deff
    core.kubebb.k8s.com.cn/dimension: security
    tekton.dev/pipeline: component-rbac-gen-run
  name: component-rbac-gen-run
  namespace: kube-system
spec:
  params:
  - name: URL
    value: https://github.com/kubebb/components/releases/download/kubebb-v0.0.1/kubebb-v0.0.1.tgz
  - name: COMPONENT_NAME
    value: kubebb
  - name: VERSION
    value: v0.0.1
  - name: REPOSITORY_NAME
    value: kubebb
  pipelineRef:
    params:
    - name: kind
      value: pipeline
    - name: name
      value: component-rbac-gen
    - name: namespace
      value: default
    resolver: cluster
  serviceAccountName: pipelinerun-service-account
  taskRunSpecs:
  - pipelineTaskName: rback
    taskServiceAccountName: task-service-account
  timeout: 1h0m0s
status:
  conditions:
  - lastTransitionTime: "2023-08-17T08:09:31Z"
    message: 'Tasks Completed: 1 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped:
      0'
    reason: Running
    status: Unknown
    type: Succeeded
  pipelineSpec:
    description: convert image to base64
    params:
    - description: component name
      name: COMPONENT_NAME
      type: string
    - default: kubebb
      description: repository name
      name: REPOSITORY_NAME
      type: string
    - description: component version
      name: VERSION
      type: string
    - description: the full URL of the component tgz file.
      name: URL
      type: string
    results:
    - description: ""
      name: HELM_LINT
      value: $(tasks.helm-lint.results.LINT)
    - description: ""
      name: RBACCM
      value: $(tasks.rback.results.RBACCM)
    tasks:
    - name: rback
      params:
      - name: url
        value: https://github.com/kubebb/components/releases/download/kubebb-v0.0.1/kubebb-v0.0.1.tgz
      - name: component
        value: kubebb
      - name: version
        value: v0.0.1
      - name: repository
        value: kubebb
      retries: 2
      taskRef:
        kind: Task
        params:
        - name: kind
          value: task
        - name: name
          value: rback
        - name: namespace
          value: default
        resolver: cluster
    - name: helm-lint
      params:
      - name: url
        value: https://github.com/kubebb/components/releases/download/kubebb-v0.0.1/kubebb-v0.0.1.tgz
      - name: component
        value: kubebb
      - name: version
        value: v0.0.1
      retries: 2
      taskRef:
        kind: Task
        params:
        - name: kind
          value: task
        - name: name
          value: helm-lint
        - name: namespace
          value: default
        resolver: cluster
  startTime: "2023-08-17T08:09:20Z"
  taskRuns:
    component-rbac-gen-run-helm-lint:
      pipelineTaskName: helm-lint
      status:
        completionTime: "2023-08-17T08:09:31Z"
        conditions:
        - lastTransitionTime: "2023-08-17T08:09:31Z"
          message: All Steps have completed executing
          reason: Succeeded
          status: "True"
          type: Succeeded
        podName: component-rbac-gen-run-helm-lint-pod
        startTime: "2023-08-17T08:09:20Z"
        steps:
        - container: step-helm-lint
          imageID: docker.io/library/import-2023-08-09@sha256:5ab09b34fe77de5fb519c83ec342b1dd6da24fa279d217766706304f4528d098
          name: helm-lint
          terminated:
            containerID: containerd://add01964322ceb70ef950c5f030d4d2faa2413ba0a9757684b31d0b2729b7efa
            exitCode: 0
            finishedAt: "2023-08-17T08:09:31Z"
            message: '[{"key":"LINT","value":"0\n","type":1}]'
            reason: Completed
            startedAt: "2023-08-17T08:09:28Z"
        taskResults:
        - name: LINT
          type: string
          value: |
            0
        taskSpec:
          params:
          - name: url
            type: string
          - name: component
            type: string
          - name: version
            type: string
          results:
          - name: LINT
            type: string
          steps:
          - image: alpine:v4
            name: helm-lint
            resources: {}
            script: |
              #!/usr/bin/env sh
    component-rbac-gen-run-rback:
      pipelineTaskName: rback
      status:
        conditions:
        - lastTransitionTime: "2023-08-17T08:09:26Z"
          message: Not all Steps in the Task have finished executing
          reason: Running
          status: Unknown
          type: Succeeded
        podName: component-rbac-gen-run-rback-pod
        startTime: "2023-08-17T08:09:20Z"
        steps:
        - container: step-rback
          imageID: docker.io/library/import-2023-08-09@sha256:5ab09b34fe77de5fb519c83ec342b1dd6da24fa279d217766706304f4528d098
          name: rback
          running:
            startedAt: "2023-08-17T08:09:26Z"
        taskSpec:
          params:
          - name: url
            type: string
          - name: component
            type: string
          - name: version
            type: string
          - name: repository
            type: string
          results:
          - name: RBACCM
            type: string
          steps:
          - image: alpine:v4
            name: rback
            resources: {}
            script: |
              #!/usr/bin/env sh`
)

func TestWhenRunningOrSucceeded(t *testing.T) {
	var dimension = "security"
	pipelinerun := v1beta1.PipelineRun{}
	if err := yaml.Unmarshal([]byte(pipelinerunRunning), &pipelinerun); err != nil {
		t.Fatalf("Test Failed. unmarshal pipelinerun failed:  %s", err.Error())
	}
	rating := Rating{
		Status: RatingStatus{
			PipelineRuns: map[string]PipelineRunStatus{},
		},
	}
	WhenRunningOrSucceeded(&pipelinerun, &rating, string(RatingRunning))
	x := rating.GetPipelineRunStatus(dimension)
	now := metav1.Now()
	x.Tasks[0].Conditions[0].LastTransitionTime = now
	x.Tasks[1].Conditions[0].LastTransitionTime = now
	rating.Status.PipelineRuns[dimension] = x

	expPipelineRunStatus := map[string]PipelineRunStatus{
		dimension: {
			Tasks: []Task{
				{Name: "rback", TaskRunName: "component-rbac-gen-run-rback", ConditionedStatus: ConditionedStatus{Conditions: []Condition{
					{
						Type:               ConditionType(RatingSucceeded),
						Status:             v1.ConditionUnknown,
						Reason:             RatingRunning,
						Message:            "Not all Steps in the Task have finished executing",
						LastTransitionTime: now,
					},
				}}},
				{Name: "helm-lint", TaskRunName: "component-rbac-gen-run-helm-lint", ConditionedStatus: ConditionedStatus{Conditions: []Condition{
					{
						Type:               ConditionType(RatingSucceeded),
						Status:             v1.ConditionTrue,
						Reason:             RatingSucceeded,
						Message:            "All Steps have completed executing",
						LastTransitionTime: now,
					},
				}}},
			},
			ConditionedStatus: ConditionedStatus{Conditions: []Condition{}},
		},
	}
	if !reflect.DeepEqual(expPipelineRunStatus, rating.Status.PipelineRuns) {
		t.Fatalf("Test Failed.expect %+v get %+v", expPipelineRunStatus, rating.Status.PipelineRuns)
	}
}
