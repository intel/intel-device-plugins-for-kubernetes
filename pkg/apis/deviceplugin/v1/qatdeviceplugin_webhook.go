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
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const (
	qatPluginKind = "QatDevicePlugin"
)

var (
	// qatdevicepluginlog is for logging in this package.
	qatdevicepluginlog = logf.Log.WithName("qatdeviceplugin-resource")

	qatMinVersion = version.MustParseSemantic("0.18.0")
)

// SetupWebhookWithManager sets up a webhook for QatDevicePlugin custom resources.
func (r *QatDevicePlugin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deviceplugin-intel-com-v1-qatdeviceplugin,mutating=true,failurePolicy=fail,groups=deviceplugin.intel.com,resources=qatdeviceplugins,verbs=create;update,versions=v1,name=mqatdeviceplugin.kb.io

var _ webhook.Defaulter = &QatDevicePlugin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *QatDevicePlugin) Default() {
	qatdevicepluginlog.Info("default", "name", r.Name)

	if len(r.Spec.Image) == 0 {
		r.Spec.Image = "intel/intel-qat-plugin:0.18.0"
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-deviceplugin-intel-com-v1-qatdeviceplugin,mutating=false,failurePolicy=fail,groups=deviceplugin.intel.com,resources=qatdeviceplugins,versions=v1,name=vqatdeviceplugin.kb.io

var _ webhook.Validator = &QatDevicePlugin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *QatDevicePlugin) ValidateCreate() error {
	qatdevicepluginlog.Info("validate create", "name", r.Name)

	if controllers.GetDevicePluginCount(qatPluginKind) > 0 {
		return errors.Errorf("an instance of %q already exists in the cluster", qatPluginKind)
	}

	return r.validatePlugin()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *QatDevicePlugin) ValidateUpdate(old runtime.Object) error {
	qatdevicepluginlog.Info("validate update", "name", r.Name)

	return r.validatePlugin()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *QatDevicePlugin) ValidateDelete() error {
	qatdevicepluginlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *QatDevicePlugin) validatePlugin() error {
	parts := strings.SplitN(r.Spec.Image, ":", 2)
	if len(parts) != 2 {
		return errors.Errorf("incorrect image field %q", r.Spec.Image)
	}
	namespacedName := parts[0]
	versionStr := parts[1]

	parts = strings.Split(namespacedName, "/")
	name := parts[len(parts)-1]
	if name != "intel-qat-plugin" {
		return errors.Errorf("incorrect image name %q. Make sure you use '<vendor>/intel-qat-plugin:<version>'", name)
	}

	ver, err := version.ParseSemantic(versionStr)
	if err != nil {
		return errors.Wrapf(err, "unable to parse version %q", versionStr)
	}

	if !ver.AtLeast(qatMinVersion) {
		return errors.Errorf("version %q is too low. Should be at least %q", ver, qatMinVersion)
	}

	return nil
}
