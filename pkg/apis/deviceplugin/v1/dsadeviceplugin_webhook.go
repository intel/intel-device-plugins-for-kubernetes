// Copyright 2020-2021 Intel Corporation. All Rights Reserved.
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
	dsaPluginKind = "DsaDevicePlugin"
)

var (
	// dsadevicepluginlog is for logging in this package.
	dsadevicepluginlog = logf.Log.WithName("dsadeviceplugin-resource")

	dsaMinVersion = controllers.ImageMinVersion
)

// SetupWebhookWithManager sets up a webhook for DsaDevicePlugin custom resources.
func (r *DsaDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-dsadeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dsadeviceplugins,verbs=create;update,versions=v1,name=mdsadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Defaulter = &DsaDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *DsaDevicePlugin) Default() {
	dsadevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-dsa-plugin:" + dsaMinVersion.String()
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-dsadeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dsadeviceplugins,versions=v1,name=vdsadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &DsaDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *DsaDevicePlugin) ValidateCreate() error {
	dsadevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(dsaPluginKind) > 0 {
		return errors.Errorf("an instance of %q already exists in the cluster", dsaPluginKind)
	}

	return r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *DsaDevicePlugin) ValidateUpdate(old runtime.Object) error {
	dsadevicepluginlog.Info("validate update", "name", r.Name)

	return r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *DsaDevicePlugin) ValidateDelete() error {
	dsadevicepluginlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *DsaDevicePlugin) validatePlugin() error {
	if err := validatePluginImage(r.Spec.Image, "intel-dsa-plugin", dsaMinVersion); err != nil {
		return err
	}

	if len(r.Spec.ProvisioningConfig) > 0 && len(r.Spec.InitImage) == 0 {
		return errors.Errorf("ProvisioningConfig is set with no InitImage")
	}

	if len(r.Spec.InitImage) > 0 {
		return validatePluginImage(r.Spec.InitImage, "intel-idxd-config-initcontainer", dsaMinVersion)
	}

	return nil
}
