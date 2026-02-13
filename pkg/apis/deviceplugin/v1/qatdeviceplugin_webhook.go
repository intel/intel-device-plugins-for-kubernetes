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
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

var qatDefaultImage = "intel/intel-qat-plugin:" + controllers.ImageMinVersion.String()
var qatValidatorConfig = &validatorConfig{
	expectedImage:     "intel-qat-plugin",
	expectedInitImage: "intel-qat-initcontainer",
	expectedVersion:   *controllers.ImageMinVersion,
}

// SetupWebhookWithManager sets up a webhook for QatDevicePlugin custom resources.
func (r *QatDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-qatdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=qatdeviceplugins,verbs=create;update,versions=v1,name=mqatdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-qatdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=qatdeviceplugins,versions=v1,name=vqatdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *QatDevicePlugin) validatePlugin(ref *validatorConfig) error {
	if r.Spec.InitImage != "" {
		if err := validatePluginImage(r.Spec.InitImage, ref.expectedInitImage, &ref.expectedVersion); err != nil {
			return err
		}
	}

	if len(r.Spec.ProvisioningConfig) > 0 {
		if len(r.Spec.InitImage) == 0 {
			return fmt.Errorf("%w: ProvisioningConfig is set with no InitImage", errValidation)
		}

		// check if 4xxxvf is enabled
		contains := false
		devicesWithCapabilities := map[KernelVfDriver]struct{}{
			"4xxxvf":  {},
			"420xxvf": {},
		}

		for _, kernelVfDriver := range r.Spec.KernelVfDrivers {
			if _, ok := devicesWithCapabilities[kernelVfDriver]; ok {
				contains = true
				break
			}
		}

		if !contains {
			return fmt.Errorf("%w: ProvisioningConfig is available only for 4xxx and 420xx devices", errValidation)
		}
	}

	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
