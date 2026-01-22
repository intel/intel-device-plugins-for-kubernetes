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
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

// SetupWebhookWithManager sets up a webhook for DsaDevicePlugin custom resources.
func (r *DsaDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(&dsaDevicePluginDefaulter{
			commonDevicePluginDefaulter: commonDevicePluginDefaulter{
				defaultImage: "intel/intel-dsa-plugin:" + controllers.ImageMinVersion.String(),
			},
		}).
		WithValidator(&dsaDevicePluginValidator{
			commonDevicePluginValidator: commonDevicePluginValidator{
				expectedImage:     "intel-dsa-plugin",
				expectedInitImage: "intel-idxd-config-initcontainer",
				expectedVersion:   *controllers.ImageMinVersion,
			},
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-dsadeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dsadeviceplugins,verbs=create;update,versions=v1,name=mdsadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-dsadeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dsadeviceplugins,versions=v1,name=vdsadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *DsaDevicePlugin) validatePlugin(ref *commonDevicePluginValidator) error {
	if err := validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion); err != nil {
		return err
	}

	if len(r.Spec.ProvisioningConfig) > 0 && len(r.Spec.InitImage) == 0 {
		return fmt.Errorf("%w: ProvisioningConfig is set with no InitImage", errValidation)
	}

	if len(r.Spec.InitImage) > 0 {
		return validatePluginImage(r.Spec.InitImage, ref.expectedInitImage, &ref.expectedVersion)
	}

	return nil
}
