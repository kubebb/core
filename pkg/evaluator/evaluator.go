/*
Copyright 2023 KubeAGI.

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

package evaluator

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"

	"github.com/go-logr/logr"
	arcadiav1 "github.com/kubeagi/arcadia/api/v1alpha1"
	"github.com/kubeagi/arcadia/pkg/llms"
	"github.com/kubeagi/arcadia/pkg/llms/zhipuai"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
)

const (
	EvaluateRatingLabel      = corev1alpha1.Group + "/rating"
	EvaluatePipelineRunLabel = corev1alpha1.Group + "/pipelinerun"
	EvaluateDimensionLabel   = corev1alpha1.Group + "/dimension"
)

var (
	ErrEmptyDimension = errors.New("empty dimension")
)

type EvaluateOptions struct {
	template     string
	outputFormat string
}

type EvaluateOptionsFunc func(*EvaluateOptions)

func WithTemplate(template string) EvaluateOptionsFunc {
	return func(o *EvaluateOptions) {
		o.template = template
	}
}

func WithOutputFormat(format string) EvaluateOptionsFunc {
	return func(o *EvaluateOptions) {
		o.outputFormat = format
	}
}

// Evaluator implements the Evaluator interface with the help of open
type Evaluator struct {
	c      client.Client
	scheme *runtime.Scheme

	logger logr.Logger

	llm *arcadiav1.LLM
}

func NewEvaluator(logger logr.Logger, ctx context.Context, c client.Client, scheme *runtime.Scheme, llm types.NamespacedName) (*Evaluator, error) {
	llmObj := &arcadiav1.LLM{}
	err := c.Get(ctx, llm, llmObj)
	if err != nil {
		return nil, err
	}
	reason, ready := llmObj.Status.LLMReady()
	if !ready {
		return nil, errors.New(reason)
	}

	return &Evaluator{
		c:      c,
		scheme: scheme,
		logger: logger,
		llm:    llmObj,
	}, nil
}

// +kubebuilder:object:generate=true
type Data struct {
	Owner   *corev1alpha1.Rating
	FromRun string

	Dimension    string
	Tasks        []corev1alpha1.Task
	OutputFormat string
}

func (evaluator *Evaluator) Evaluate(ctx context.Context, rating *corev1alpha1.Rating, opts ...EvaluateOptionsFunc) error {
	// evaluate with different pipelinerun
	for dimension, runStatus := range rating.Status.PipelineRuns {
		data := &Data{
			Owner:     rating.DeepCopy(),
			FromRun:   runStatus.PipelineRunName,
			Dimension: dimension,
			Tasks:     runStatus.Tasks,
		}
		err := evaluator.EvaluateWithData(ctx, data, opts...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (evaluator *Evaluator) EvaluateWithData(ctx context.Context, data *Data, opts ...EvaluateOptionsFunc) error {
	if data.Dimension == "" {
		return ErrEmptyDimension
	}
	// process options
	options := &EvaluateOptions{
		template:     DefaultPromptTemplate,
		outputFormat: DefaultOutputFormat,
	}
	for _, optFunc := range opts {
		optFunc(options)
	}
	if data.OutputFormat == "" {
		data.OutputFormat = options.outputFormat
	}

	// initialize a arcadia Prompt
	tmpl := template.Must(template.New("Evaluate").Parse(options.template))
	var output strings.Builder
	err := tmpl.Execute(&output, data)
	if err != nil {
		return fmt.Errorf("invalid evaluate template: %s", err.Error())
	}
	prompt := &arcadiav1.Prompt{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: evaluator.llm.Namespace,
			Name:      data.Owner.Name + "-" + data.Dimension,
			Labels: map[string]string{
				EvaluateRatingLabel:      data.Owner.Name,
				EvaluatePipelineRunLabel: data.FromRun,
				EvaluateDimensionLabel:   data.Dimension,
			},
		},
		Spec: arcadiav1.PromptSpec{
			LLM: evaluator.llm.Name,
		},
	}
	_ = controllerutil.SetOwnerReference(data.Owner, prompt, evaluator.scheme)

	// build prompt params
	switch evaluator.llm.Spec.Type {
	case llms.ZhiPuAI:
		params := zhipuai.DefaultModelParams()
		params.Model = zhipuai.ZhiPuAILite
		params.Prompt = []zhipuai.Prompt{
			{Role: zhipuai.User, Content: output.String()},
		}
		prompt.Spec.ZhiPuAIParams = &params
	default:
		return fmt.Errorf("unsupported LLM type: %s", evaluator.llm.Spec.Type)
	}

	// create or update a arcadia Prompt
	err = evaluator.c.Create(ctx, prompt)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return evaluator.c.Update(ctx, prompt)
}

func OnPromptUpdate(logger logr.Logger, c client.Client) func(event.UpdateEvent, workqueue.RateLimitingInterface) {
	return func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
		var err error

		// check prompt rating
		newPrompt := e.ObjectNew.(*arcadiav1.Prompt)
		ratingName, ok := newPrompt.Labels[EvaluateRatingLabel]
		if !ok {
			// not a rating prompt,do nothing
			return
		}
		// check rating
		rating := &corev1alpha1.Rating{}
		err = c.Get(context.TODO(), types.NamespacedName{Name: ratingName, Namespace: newPrompt.Namespace}, rating)
		if err != nil {
			logger.Error(err, "failed to get rating", "rating", ratingName)
			return
		}

		dimension, ok := newPrompt.Labels[EvaluateDimensionLabel]
		if !ok {
			// dimension is empty.not a rating prompt,do nothing
			return
		}

		deepCopyRating := rating.DeepCopy()
		// update rating
		if deepCopyRating.Status.Evaluations == nil {
			deepCopyRating.Status.Evaluations = make(map[string]corev1alpha1.EvaluatorStatus)
		}
		evaluationStatus := corev1alpha1.EvaluatorStatus{
			Prompt:            newPrompt.Name,
			ConditionedStatus: newPrompt.Status.ConditionedStatus,
		}
		// TODO: get the final rating when succ
		// condition := newPrompt.Status.ConditionedStatus.GetCondition(arcadiav1.TypeDone)
		// if condition.Status == corev1.ConditionTrue {
		// 	switch rating.Spec.LLM {
		// 	case llms.ZhiPuAI:
		// 		resp := &zhipuai.Response{}
		// 		err = json.Unmarshal(newPrompt.Status.Data, resp)
		// 		if err != nil {
		// 			logger.Error(err, "failed to unmarshal evaluation response")
		// 			break
		// 		}
		// 		if resp.Data != nil && len(resp.Data.Choices) != 0 {
		// 			output := &Output{}
		// 			err = json.Unmarshal([]byte(resp.Data.Choices[0].Content), output)
		// 			if err != nil {
		// 				logger.Error(err, "failed to unmarshal evaluation response")
		// 			}
		// 			evaluationStatus.FinalRating = output.Rating
		// 		}
		// 	default:
		// 	}
		// }

		deepCopyRating.Status.Evaluations[dimension] = evaluationStatus
		err = c.Status().Patch(context.TODO(), deepCopyRating, client.MergeFrom(rating))
		if err != nil {
			logger.Error(err, "failed to update rating status", "rating", ratingName)
			return
		}
	}
}
