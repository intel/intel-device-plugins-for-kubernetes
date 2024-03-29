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
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

var (
	// gpudevicepluginlog is for logging in this package.
	gpudevicepluginlog = logf.Log.WithName("gpudeviceplugin-resource")

	gpuMinVersion = controllers.ImageMinVersion
)

var cli client.Client

// SetupWebhookWithManager sets up a webhook for GpuDevicePlugin custom resources.
func (r *GpuDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	cli = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-gpudeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=gpudeviceplugins,verbs=create;update,versions=v1,name=mgpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Defaulter = &GpuDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *GpuDevicePlugin) Default() {
	gpudevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-gpu-plugin:" + gpuMinVersion.String()
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-gpudeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=gpudeviceplugins,versions=v1,name=vgpudeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &GpuDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GpuDevicePlugin) ValidateCreate() (admission.Warnings, error) {
	gpudevicepluginlog.Info("validate create", "name", r.Name)

	return nil, r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GpuDevicePlugin) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	gpudevicepluginlog.Info("validate update", "name", r.Name)

	return nil, r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GpuDevicePlugin) ValidateDelete() (admission.Warnings, error) {
	gpudevicepluginlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func (r *GpuDevicePlugin) crossCheckResourceManagement() bool {
	ctx := context.Background()
	gpuCrs := GpuDevicePluginList{}

	if err := cli.List(ctx, &gpuCrs); err != nil {
		gpudevicepluginlog.Info("unable to list GPU CRs")

		return false
	}

	for _, cr := range gpuCrs.Items {
		// Ignore itself.
		if cr.Name == r.Name {
			continue
		}

		if cr.Spec.ResourceManager != r.Spec.ResourceManager {
			return false
		}
	}

	return true
}

func (r *GpuDevicePlugin) validatePlugin() error {
	if r.Spec.SharedDevNum == 1 && r.Spec.PreferredAllocationPolicy != "none" {
		return errors.Errorf("PreferredAllocationPolicy is valid only when setting sharedDevNum > 1")
	}

	if r.Spec.SharedDevNum == 1 && r.Spec.ResourceManager {
		return errors.Errorf("resourceManager is valid only when setting sharedDevNum > 1")
	}

	if !r.crossCheckResourceManagement() {
		return errors.Errorf("All GPU CRs must be with or without resource management")
	}

	return validatePluginImage(r.Spec.Image, "intel-gpu-plugin", gpuMinVersion)
}
