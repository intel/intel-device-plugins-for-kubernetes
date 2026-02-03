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

// SetupWebhookWithManager sets up a webhook for GpuDevicePlugin custom resources.
func (r *GpuDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
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

// ValidatePCIDeviceIDs validates that the provided comma-separated list of PCI
// device IDs is in the correct format (0x followed by 4 hexadecimal digits).
func validatePCIDeviceIDs(pciIDList string) error {
	if pciIDList == "" {
		return nil
	}

	r := regexp.MustCompile(`^0x[0-9a-fA-F]{4}$`)

	for id := range strings.SplitSeq(pciIDList, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("empty PCI device ID: %w", errValidation)
		}

		if !r.MatchString(id) {
			return fmt.Errorf("invalid PCI device ID: %s: %w", id, errValidation)
		}
	}

	return nil
}

func (r *GpuDevicePlugin) validatePlugin(ref *commonDevicePluginValidator) error {
	if r.Spec.SharedDevNum == 1 {
		switch r.Spec.PreferredAllocationPolicy {
		case "packed", "balanced":
			return fmt.Errorf("preferredAllocationPolicy is valid only when setting sharedDevNum > 1: %w", errValidation)
		}
	}

	if err := validatePCIDeviceIDs(r.Spec.AllowIDs); err != nil {
		return fmt.Errorf("invalid allow IDs: %w", err)
	}

	if err := validatePCIDeviceIDs(r.Spec.DenyIDs); err != nil {
		return fmt.Errorf("invalid deny IDs: %w", err)
	}

	if len(r.Spec.AllowIDs) > 0 && len(r.Spec.DenyIDs) > 0 {
		return fmt.Errorf("allowIDs and denyIDs cannot be used together: %w", errValidation)
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
		return fmt.Errorf("initImage should be used only when vfioMode is enabled: %w", errValidation)
	}

	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
