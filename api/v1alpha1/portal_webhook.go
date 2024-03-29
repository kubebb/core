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
var portallog = logf.Log.WithName("portal-resource")

func (p *Portal) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		WithDefaulter(p).
		WithValidator(p).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-core-kubebb-k8s-com-cn-v1alpha1-portal,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=portals,verbs=create;update,versions=v1alpha1,name=mportal.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &Portal{}

func (p *Portal) Default(ctx context.Context, obj runtime.Object) error {
	portallog.Info("default", "name", p.Name)

	return nil
}

//+kubebuilder:webhook:path=/validate-core-kubebb-k8s-com-cn-v1alpha1-portal,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=portals,verbs=create;update;delete,versions=v1alpha1,name=vportal.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &Portal{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (p *Portal) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	portallog.Info("validate create", "name", p.Name)

	// TODO: validate entry & path conflicts

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (p *Portal) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) error {
	portallog.Info("validate update", "name", p.Name)

	// TODO: validate entry & path conflicts
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (p *Portal) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	portallog.Info("validate delete", "name", p.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
