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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const (
	qatPluginKind = "QatDevicePlugin"
)

var (
	// qatdevicepluginlog is for logging in this package.
	qatdevicepluginlog = logf.Log.WithName("qatdeviceplugin-resource")

	qatMinVersion = controllers.ImageMinVersion
)

// SetupWebhookWithManager sets up a webhook for QatDevicePlugin custom resources.
func (r *QatDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-qatdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=qatdeviceplugins,verbs=create;update,versions=v1,name=mqatdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Defaulter = &QatDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *QatDevicePlugin) Default() {
	qatdevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-qat-plugin:" + qatMinVersion.String()
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-qatdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=qatdeviceplugins,versions=v1,name=vqatdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &QatDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *QatDevicePlugin) ValidateCreate() (admission.Warnings, error) {
	qatdevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(qatPluginKind) > 0 {
		return nil, errors.Errorf("an instance of %q already exists in the cluster", qatPluginKind)
	}

	return nil, r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *QatDevicePlugin) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	qatdevicepluginlog.Info("validate update", "name", r.Name)

	return nil, r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *QatDevicePlugin) ValidateDelete() (admission.Warnings, error) {
	qatdevicepluginlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func (r *QatDevicePlugin) validatePlugin() error {
	if r.Spec.InitImage != "" {
		if err := validatePluginImage(r.Spec.InitImage, "intel-qat-initcontainer", qatMinVersion); err != nil {
			return err
		}
	}

	if len(r.Spec.ProvisioningConfig) > 0 {
		if len(r.Spec.InitImage) == 0 {
			return errors.Errorf("ProvisioningConfig is set with no InitImage")
		}

		// check if 4xxxvf is enabled
		contains := false
		devicesWithCapabilities := map[KernelVfDriver]struct{}{
			"4xxxvf": {},
		}

		for _, kernelVfDriver := range r.Spec.KernelVfDrivers {
			if _, ok := devicesWithCapabilities[kernelVfDriver]; ok {
				contains = true
				break
			}
		}

		if !contains {
			return errors.Errorf("ProvisioningConfig is available only for 4xxx devices")
		}
	}

	return validatePluginImage(r.Spec.Image, "intel-qat-plugin", qatMinVersion)
}
