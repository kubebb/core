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
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var repositorylog = logf.Log.WithName("repository-resource")

func (r *Repository) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-core-kubebb-k8s-com-cn-v1alpha1-repository,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.kubebb.k8s.com.cn,resources=repositories,verbs=create;update,versions=v1alpha1,name=mrepository.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &Repository{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Repository) Default(ctx context.Context, obj runtime.Object) error {
	log := repositorylog.WithValues("name", r.Name, "method", "Default")
	p, ok := obj.(*Repository)
	if !ok {
		log.Error(ErrDecode, ErrDecode.Error())
		return ErrDecode
	}
	if p.IsOCI() {
		p.Spec.URL = strings.ToLower(p.Spec.URL)
	}
	log.Info("set default value done")
	return nil
}
