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
	"regexp"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

var pciIDRegex regexp.Regexp

// SetupWebhookWithManager sets up a webhook for GpuDevicePlugin custom resources.
func (r *GpuDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	pciIDRegex = *regexp.MustCompile(`^0x[0-9a-f]{4}$`)

	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(&gpuDevicePluginDefaulter{&commonDevicePluginDefaulter{
			defaultImage: "intel/intel-gpu-plugin:" + controllers.ImageMinVersion.String(),
		}}).
		WithValidator(&gpuDevicePluginValidator{&commonDevicePluginValidator{
			expectedImage:   "intel-gpu-plugin",
			expectedVersion: *controllers.ImageMinVersion,
		}}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-gpudeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=gpudeviceplugins,verbs=create;update,versions=v1,name=mgpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-gpudeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=gpudeviceplugins,versions=v1,name=vgpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *GpuDevicePlugin) validatePlugin(ref *commonDevicePluginValidator) error {
	if r.Spec.SharedDevNum == 1 && r.Spec.PreferredAllocationPolicy != "none" {
		return fmt.Errorf("%w: PreferredAllocationPolicy is valid only when setting sharedDevNum > 1", errValidation)
	}

	if r.Spec.AllowIDs != "" {
		for id := range strings.SplitSeq(r.Spec.AllowIDs, ",") {
			if id == "" {
				return fmt.Errorf("%w: Empty PCI Device ID in AllowIDs", errValidation)
			}

			if !pciIDRegex.MatchString(id) {
				return fmt.Errorf("%w: Invalid PCI Device ID: %s", errValidation, id)
			}
		}
	}

	if r.Spec.DenyIDs != "" {
		for id := range strings.SplitSeq(r.Spec.DenyIDs, ",") {
			if id == "" {
				return fmt.Errorf("%w: Empty PCI Device ID in DenyIDs", errValidation)
			}

			if !pciIDRegex.MatchString(id) {
				return fmt.Errorf("%w: Invalid PCI Device ID: %s", errValidation, id)
			}
		}
	}

	if len(r.Spec.AllowIDs) > 0 && len(r.Spec.DenyIDs) > 0 {
		return fmt.Errorf("%w: AllowIDs and DenyIDs cannot be used together", errValidation)
	}

	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
