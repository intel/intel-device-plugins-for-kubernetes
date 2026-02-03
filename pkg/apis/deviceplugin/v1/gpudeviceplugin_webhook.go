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

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(&commonDevicePluginDefaulter{
			defaultImage: "intel/intel-gpu-plugin:" + controllers.ImageMinVersion.String(),
		}).
		WithValidator(&commonDevicePluginValidator{
			expectedImage:   "intel-gpu-plugin",
			expectedVersion: *controllers.ImageMinVersion,
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-gpudeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=gpudeviceplugins,verbs=create;update,versions=v1,name=mgpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-gpudeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=gpudeviceplugins,versions=v1,name=vgpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

func validateDeviceIds(idList string) error {
	if idList != "" {
		for id := range strings.SplitSeq(idList, ",") {
			if id == "" {
				return fmt.Errorf("%w: Empty PCI Device ID", errValidation)
			}

			if !pciIDRegex.MatchString(id) {
				return fmt.Errorf("%w: Invalid PCI Device ID: %s", errValidation, id)
			}
		}
	}

	return nil
}

func (r *GpuDevicePlugin) validatePlugin(ref *commonDevicePluginValidator) error {
	if r.Spec.SharedDevNum == 1 && r.Spec.PreferredAllocationPolicy != "none" {
		return fmt.Errorf("%w: PreferredAllocationPolicy is valid only when setting sharedDevNum > 1", errValidation)
	}

	if err := validateDeviceIds(r.Spec.AllowIDs); err != nil {
		return fmt.Errorf("%w: Allow IDs", err)
	}

	if err := validateDeviceIds(r.Spec.DenyIDs); err != nil {
		return fmt.Errorf("%w: Deny IDs", err)
	}

	if len(r.Spec.AllowIDs) > 0 && len(r.Spec.DenyIDs) > 0 {
		return fmt.Errorf("%w: AllowIDs and DenyIDs cannot be used together", errValidation)
	}

	if r.Spec.VFIOMode {
		if r.Spec.EnableMonitoring {
			return fmt.Errorf("%w: enableMonitoring cannot be used together with vfioMode", errValidation)
		}
		if r.Spec.SharedDevNum > 1 {
			return fmt.Errorf("%w: sharedDevNum cannot be greater than 1 when vfioMode is enabled", errValidation)
		}
	}

	if !r.Spec.VFIOMode && r.Spec.InitImage != "" {
		return fmt.Errorf("%w: initImage should be used only when vfioMode is enabled", errValidation)
	}

	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
