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
	dlbPluginKind = "DlbDevicePlugin"
)

var (
	// dlbdevicepluginlog is for logging in this package.
	dlbdevicepluginlog = logf.Log.WithName("dlbdeviceplugin-resource")

	dlbMinVersion = version.MustParseSemantic(imageMinVersion)
)

// SetupWebhookWithManager sets up a webhook for DlbDevicePlugin custom resources.
func (r *DlbDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-dlbdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dlbdeviceplugins,verbs=create;update,versions=v1,name=mdlbdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Defaulter = &DlbDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *DlbDevicePlugin) Default() {
	dlbdevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-dlb-plugin:" + dlbMinVersion.String()
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-dlbdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=dlbdeviceplugins,versions=v1,name=vdlbdeviceplugin.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &DlbDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *DlbDevicePlugin) ValidateCreate() error {
	dlbdevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(dlbPluginKind) > 0 {
		return errors.Errorf("an instance of %q already exists in the cluster", dlbPluginKind)
	}

	return r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *DlbDevicePlugin) ValidateUpdate(old runtime.Object) error {
	dlbdevicepluginlog.Info("validate update", "name", r.Name)

	return r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *DlbDevicePlugin) ValidateDelete() error {
	dlbdevicepluginlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *DlbDevicePlugin) validatePlugin() error {
	return validatePluginImage(r.Spec.Image, "intel-dlb-plugin", dlbMinVersion)
}
