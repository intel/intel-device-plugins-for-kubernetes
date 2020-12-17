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
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const (
	fpgaPluginKind = "FpgaDevicePlugin"
)

var (
	// fpgadevicepluginlog is for logging in this package.
	fpgadevicepluginlog = logf.Log.WithName("fpgadeviceplugin-resource")

	fpgaMinVersion = version.MustParseSemantic("0.19.0")
)

// SetupWebhookWithManager sets up a webhook for FpgaDevicePlugin custom resources.
func (r *FpgaDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-fpgadeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=fpgadeviceplugins,verbs=create;update,versions=v1,name=mfpgadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Defaulter = &FpgaDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *FpgaDevicePlugin) Default() {
	fpgadevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-fpga-plugin:0.19.0"
	}

	if len(r.Spec.InitImage) == 0 {
		r.Spec.InitImage = "intel/intel-fpga-initcontainer:0.19.0"
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-fpgadeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=fpgadeviceplugins,versions=v1,name=vfpgadeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &FpgaDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *FpgaDevicePlugin) ValidateCreate() error {
	fpgadevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(fpgaPluginKind) > 0 {
		return errors.Errorf("an instance of %q already exists in the cluster", fpgaPluginKind)
	}

	return r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *FpgaDevicePlugin) ValidateUpdate(old runtime.Object) error {
	fpgadevicepluginlog.Info("validate update", "name", r.Name)

	return r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *FpgaDevicePlugin) ValidateDelete() error {
	fpgadevicepluginlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *FpgaDevicePlugin) validatePlugin() error {
	if err := validatePluginImage(r.Spec.Image, "intel-fpga-plugin", fpgaMinVersion); err != nil {
		return err
	}

	return validatePluginImage(r.Spec.InitImage, "intel-fpga-initcontainer", fpgaMinVersion)
}
