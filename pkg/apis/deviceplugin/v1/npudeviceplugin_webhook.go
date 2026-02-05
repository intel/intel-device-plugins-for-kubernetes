// Copyright 2025 Intel Corporation. All Rights Reserved.
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

// SetupWebhookWithManager sets up a webhook for NpuDevicePlugin custom resources.
func (r *NpuDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithCustomDefaulter(&commonDevicePluginDefaulter{
			defaultImage: "intel/intel-npu-plugin:" + controllers.ImageMinVersion.String(),
		}).
		WithCustomValidator(&commonDevicePluginValidator{
			expectedImage:   "intel-npu-plugin",
			expectedVersion: *controllers.ImageMinVersion,
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-npudeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=npudeviceplugins,verbs=create;update,versions=v1,name=mnpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-npudeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=npudeviceplugins,versions=v1,name=vnpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *NpuDevicePlugin) validatePlugin(ref *commonDevicePluginValidator) error {
	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
