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
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

var cli client.Client

// SetupWebhookWithManager sets up a webhook for GpuDevicePlugin custom resources.
func (r *GpuDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	cli = mgr.GetClient()

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

func (r *GpuDevicePlugin) crossCheckResourceManagement(ctx context.Context) bool {
	log := logf.FromContext(ctx)
	gpuCrs := GpuDevicePluginList{}

	if err := cli.List(ctx, &gpuCrs); err != nil {
		log.Info("unable to list GPU CRs")

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

func (r *GpuDevicePlugin) validatePlugin(ctx context.Context, ref *commonDevicePluginValidator) error {
	if r.Spec.SharedDevNum == 1 && r.Spec.PreferredAllocationPolicy != "none" {
		return fmt.Errorf("%w: PreferredAllocationPolicy is valid only when setting sharedDevNum > 1", errValidation)
	}

	if r.Spec.SharedDevNum == 1 && r.Spec.ResourceManager {
		return fmt.Errorf("%w: resourceManager is valid only when setting sharedDevNum > 1", errValidation)
	}

	if !r.crossCheckResourceManagement(ctx) {
		return fmt.Errorf("%w: All GPU CRs must be with or without resource management", errValidation)
	}

	return validatePluginImage(r.Spec.Image, ref.expectedImage, &ref.expectedVersion)
}
