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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var componentlog = logf.Log.WithName("component-webhook")

func (c *Component) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(c).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-core-kubebb-k8s-com-cn-v1alpha1-component,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=components,verbs=create;update,versions=v1alpha1,name=component.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &Component{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (c *Component) Default(ctx context.Context, obj runtime.Object) error {
	log := componentlog.WithValues("name", c.Name, "method", "Default")
	var err error

	p, ok := obj.(*Component)
	if !ok {
		err = fmt.Errorf("runtime object %s isn't component", obj.GetObjectKind())
		log.Error(err, "")
		return err
	}

	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser error")
		return err
	}
	p.Spec.Creator = user.Username
	return nil
}
