// Copyright 2021 Intel Corporation. All Rights Reserved.
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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const (
	iaaPluginKind = "IaaDevicePlugin"
)

var (
	// iaadevicepluginlog is for logging in this package.
	iaadevicepluginlog = logf.Log.WithName("iaadeviceplugin-resource")

	iaaMinVersion = controllers.ImageMinVersion
)

// SetupWebhookWithManager sets up a webhook for IaaDevicePlugin custom resources.
func (r *IaaDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-iaadeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=iaadeviceplugins,verbs=create;update,versions=v1,name=miaadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Defaulter = &IaaDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *IaaDevicePlugin) Default() {
	iaadevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-iaa-plugin:" + iaaMinVersion.String()
	}

	if len(r.Spec.InitImage) == 0 {
		r.Spec.InitImage = "intel/intel-idxd-config-initcontainer:" + iaaMinVersion.String()
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-iaadeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=iaadeviceplugins,versions=v1,name=viaadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &IaaDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *IaaDevicePlugin) ValidateCreate() (admission.Warnings, error) {
	iaadevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(iaaPluginKind) > 0 {
		return nil, errors.Errorf("an instance of %q already exists in the cluster", iaaPluginKind)
	}

	return nil, r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *IaaDevicePlugin) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	iaadevicepluginlog.Info("validate update", "name", r.Name)

	return nil, r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *IaaDevicePlugin) ValidateDelete() (admission.Warnings, error) {
	iaadevicepluginlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func (r *IaaDevicePlugin) validatePlugin() error {
	if err := validatePluginImage(r.Spec.Image, "intel-iaa-plugin", iaaMinVersion); err != nil {
		return err
	}

	if len(r.Spec.ProvisioningConfig) > 0 && len(r.Spec.InitImage) == 0 {
		return errors.Errorf("ProvisioningConfig is set with no InitImage")
	}

	if len(r.Spec.InitImage) > 0 {
		return validatePluginImage(r.Spec.InitImage, "intel-idxd-config-initcontainer", iaaMinVersion)
	}

	return nil
}
