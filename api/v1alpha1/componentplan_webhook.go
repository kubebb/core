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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var componentplanlog = logf.Log.WithName("componentplan-webhook")

func (c *ComponentPlan) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(c).
		WithValidator(c).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-core-kubebb-k8s-com-cn-v1alpha1-componentplan,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=componentplans,verbs=create;update,versions=v1alpha1,name=mcomponentplan.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &ComponentPlan{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (c *ComponentPlan) Default(ctx context.Context, obj runtime.Object) error {
	log := componentplanlog.WithValues("name", c.Name, "method", "Default")
	p, ok := obj.(*ComponentPlan)
	if !ok {
		log.Error(ErrDecode, ErrDecode.Error())
		return ErrDecode
	}
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	if !isSuperUser(user) {
		p.Spec.Creator = user.Username
	}
	log.Info("set default value done")
	return nil
}

//+kubebuilder:webhook:path=/validate-core-kubebb-k8s-com-cn-v1alpha1-componentplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=componentplans,verbs=create;update;delete,versions=v1alpha1,name=vcomponentplan.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &ComponentPlan{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *ComponentPlan) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	log := componentplanlog.WithValues("name", c.Name, "method", "ValidateCreate")
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	log.Info("validate create done")
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *ComponentPlan) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) error {
	log := componentplanlog.WithValues("name", c.Name, "method", "ValidateUpdate")
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	p, ok := oldObj.(*ComponentPlan)
	if !ok {
		log.Error(ErrDecode, "oldObj "+ErrDecode.Error())
		return ErrDecode
	}
	np, ok := newObj.(*ComponentPlan)
	if !ok {
		log.Error(ErrDecode, "newObj "+ErrDecode.Error())
		return ErrDecode
	}
	if p.Spec.Name != np.Spec.Name {
		log.Info(ErrReleaseNameChange.Error(), "old", p.Spec.Name, "new", np.Spec.Name)
		return ErrReleaseNameChange
	}
	if p.Spec.ComponentRef.Namespace != np.Spec.ComponentRef.Namespace || p.Spec.ComponentRef.Name != np.Spec.ComponentRef.Name {
		log.Info(ErrComponentChange.Error(), "old", p.Spec.ComponentRef, "new", np.Spec.ComponentRef)
		return ErrComponentChange
	}
	if p.Spec.Creator != np.Spec.Creator {
		log.Info(ErrCreatorChange.Error(), "old", p.Spec.Creator, "new", np.Spec.Creator)
		return ErrCreatorChange
	}
	log.Info("validate update done")
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *ComponentPlan) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	log := componentplanlog.WithValues("name", c.Name, "method", "ValidateDelete")
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	log.Info("validate delete done")
	return nil
}
