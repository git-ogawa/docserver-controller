/*
Copyright 2023.

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

package v1beta1

import (
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var docserverlog = logf.Log.WithName("docserver-resource")

func (r *DocServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-update-git-ogawa-github-io-v1beta1-docserver,mutating=true,failurePolicy=fail,sideEffects=None,groups=update.git-ogawa.github.io,resources=docservers,verbs=create;update,versions=v1beta1,name=mdocserver.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &DocServer{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DocServer) Default() {
	docserverlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "squidfunk/mkdocs-material:latest"
	}

	if len(r.Spec.Target.Branch) == 0 {
		r.Spec.Target.Branch = "main"
	}

	if r.Spec.Target.SSLVerify == nil {
		verify := true
		r.Spec.Target.SSLVerify = &verify
	}

	if r.Spec.Target.Depth <= 0 {
		r.Spec.Target.Depth = 1
	}
}

//+kubebuilder:webhook:path=/validate-update-git-ogawa-github-io-v1beta1-docserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=update.git-ogawa.github.io,resources=docservers,verbs=create;update,versions=v1beta1,name=vdocserver.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DocServer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DocServer) ValidateCreate() error {
	docserverlog.Info("validate create", "name", r.Name)

	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DocServer) ValidateUpdate(old runtime.Object) error {
	docserverlog.Info("validate update", "name", r.Name)

	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DocServer) ValidateDelete() error {
	docserverlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *DocServer) validate() error {
	var errs field.ErrorList

	re := regexp.MustCompile(`^(https|ssh).*\.git`)
	if !re.MatchString(r.Spec.Target.Url) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "replicas"), r.Spec.Target.Url, "Url must start with https or ssh and end with .git."))
	}

	if len(errs) > 0 {
		err := apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "DocServer"}, r.Name, errs)
		docserverlog.Error(err, "validation error", "name", r.Name)
		return err
	}

	return nil
}
