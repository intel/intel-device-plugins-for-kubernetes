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
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

// SetupWebhookWithManager sets up a webhook for SgxDevicePlugin custom resources.
func (r *SgxDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(&sgxDevicePluginDefaulter{
			commonDevicePluginDefaulter: commonDevicePluginDefaulter{
				defaultImage: "intel/intel-sgx-plugin:" + controllers.ImageMinVersion.String(),
			},
		}).
		WithValidator(&sgxDevicePluginValidator{
			commonDevicePluginValidator: commonDevicePluginValidator{
				expectedImage:     "intel-sgx-plugin",
				expectedInitImage: "intel-sgx-initcontainer",
				expectedVersion:   *controllers.ImageMinVersion,
			},
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-sgxdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=sgxdeviceplugins,verbs=create;update,versions=v1,name=msgxdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1,reinvocationPolicy=IfNeeded
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-sgxdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=sgxdeviceplugins,versions=v1,name=vsgxdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *SgxDevicePlugin) validatePlugin(ref *commonDevicePluginValidator) error {
	if err := validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion); err != nil {
		return err
	}

	if r.Spec.InitImage == "" {
		return nil
	}

	return validatePluginImage(r.Spec.InitImage, ref.expectedInitImage, &ref.expectedVersion)
}
