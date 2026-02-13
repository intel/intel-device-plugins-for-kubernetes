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

var dlbDefaultImage = "intel/intel-dlb-plugin:" + controllers.ImageMinVersion.String()
var dlbValidatorConfig = &validatorConfig{
	expectedImage:     "intel-dlb-plugin",
	expectedInitImage: "intel-dlb-initimage",
	expectedVersion:   *controllers.ImageMinVersion,
}

// SetupWebhookWithManager sets up a webhook for DlbDevicePlugin custom resources.
func (r *DlbDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-dlbdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dlbdeviceplugins,verbs=create;update,versions=v1,name=mdlbdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-dlbdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dlbdeviceplugins,versions=v1,name=vdlbdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *DlbDevicePlugin) validatePlugin(ref *validatorConfig) error {
	if r.Spec.InitImage != "" {
		if err := validatePluginImage(r.Spec.InitImage, ref.expectedInitImage, &ref.expectedVersion); err != nil {
			return err
		}
	}

	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
