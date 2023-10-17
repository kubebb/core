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

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var subscriptionlog = logf.Log.WithName("subscription-webhook")

func (r *Subscription) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-core-kubebb-k8s-com-cn-v1alpha1-subscription,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=subscriptions,verbs=create;update,versions=v1alpha1,name=msubscription.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &Subscription{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Subscription) Default(ctx context.Context, obj runtime.Object) error {
	log := subscriptionlog.WithValues("name", r.Name, "method", "Default")
	p, ok := obj.(*Subscription)
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
	if p.Labels == nil {
		p.Labels = make(map[string]string, 1)
	}
	p.Labels[ComponentNameLabel] = p.Spec.ComponentRef.Name
	p.Labels[ComponentNamespaceLabel] = p.Spec.ComponentRef.Namespace
	log.Info("set default value done")
	return nil
}

//+kubebuilder:webhook:path=/validate-core-kubebb-k8s-com-cn-v1alpha1-subscription,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=subscriptions,verbs=create;update;delete,versions=v1alpha1,name=vsubscription.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &Subscription{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Subscription) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	log := subscriptionlog.WithValues("name", r.Name, "method", "ValidateCreate")
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	s, ok := obj.(*Subscription)
	if !ok {
		log.Error(ErrDecode, "oldObj "+ErrDecode.Error())
		return ErrDecode
	}
	if err = s.validateSpec(); err != nil {
		log.Info(err.Error())
		return err
	}
	log.Info("validate create done")
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Subscription) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) error {
	log := subscriptionlog.WithValues("name", r.Name, "method", "ValidateUpdate")
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	s, ok := oldObj.(*Subscription)
	if !ok {
		log.Error(ErrDecode, "oldObj "+ErrDecode.Error())
		return ErrDecode
	}
	ns, ok := newObj.(*Subscription)
	if !ok {
		log.Error(ErrDecode, "newObj "+ErrDecode.Error())
		return ErrDecode
	}
	if s.Spec.Name != ns.Spec.Name {
		log.Info(ErrReleaseNameChange.Error(), "old", s.Spec.Name, "new", ns.Spec.Name)
		return ErrReleaseNameChange
	}
	if s.Spec.ComponentRef.Namespace != ns.Spec.ComponentRef.Namespace || s.Spec.ComponentRef.Name != ns.Spec.ComponentRef.Name {
		log.Info(ErrComponentChange.Error(), "old", s.Spec.ComponentRef, "new", ns.Spec.ComponentRef)
		return ErrComponentChange
	}
	if s.Spec.Creator != ns.Spec.Creator {
		log.Info(ErrCreatorChange.Error(), "old", s.Spec.Creator, "new", ns.Spec.Creator)
		return ErrCreatorChange
	}
	if err = ns.validateSpec(); err != nil {
		log.Info(err.Error())
		return err
	}
	log.Info("validate update done")
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Subscription) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	log := subscriptionlog.WithValues("name", r.Name, "method", "ValidateDelete")
	user, err := getReqUserInfo(ctx)
	if err != nil {
		log.Error(err, "get ReqUser err")
		return err
	}
	log = log.WithValues("user", user)
	log.Info("validate delete done")
	return nil
}

func (r *Subscription) validateSpec() error {
	if r.Spec.ComponentRef == nil || r.Spec.ComponentRef.Namespace == "" || r.Spec.ComponentRef.Name == "" {
		return ErrComponentMissing
	}
	if r.Spec.ComponentPlanInstallMethod == InstallMethodAuto && r.Spec.Schedule != "" {
		if _, err := cron.ParseStandard(r.Spec.Schedule); err != nil {
			return ErrUnParseableSchedule
		}
	}
	return nil
}
