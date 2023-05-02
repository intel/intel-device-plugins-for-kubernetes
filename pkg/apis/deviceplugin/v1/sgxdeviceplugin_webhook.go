// Copyright 2020 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const (
	sgxPluginKind = "SgxDevicePlugin"
)

var (
	// sgxdevicepluginlog is for logging in this package.
	sgxdevicepluginlog = logf.Log.WithName("sgxdeviceplugin-resource")

	sgxMinVersion = controllers.ImageMinVersion
)

// SetupWebhookWithManager sets up a webhook for SgxDevicePlugin custom resources.
func (r *SgxDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-sgxdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=sgxdeviceplugins,verbs=create;update,versions=v1,name=msgxdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1,reinvocationPolicy=IfNeeded

var _ webhook.Defaulter = &SgxDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *SgxDevicePlugin) Default() {
	sgxdevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-sgx-plugin:" + sgxMinVersion.String()
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-sgxdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=sgxdeviceplugins,versions=v1,name=vsgxdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &SgxDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *SgxDevicePlugin) ValidateCreate() error {
	sgxdevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(sgxPluginKind) > 0 {
		return errors.Errorf("an instance of %q already exists in the cluster", sgxPluginKind)
	}

	return r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *SgxDevicePlugin) ValidateUpdate(old runtime.Object) error {
	sgxdevicepluginlog.Info("validate update", "name", r.Name)

	return r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *SgxDevicePlugin) ValidateDelete() error {
	sgxdevicepluginlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *SgxDevicePlugin) validatePlugin() error {
	if err := validatePluginImage(r.Spec.Image, "intel-sgx-plugin", sgxMinVersion); err != nil {
		return err
	}

	if r.Spec.InitImage == "" {
		return nil
	}

	return validatePluginImage(r.Spec.InitImage, "intel-sgx-initcontainer", sgxMinVersion)
}
