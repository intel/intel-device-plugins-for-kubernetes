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
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/version"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const sha256RE = "@sha256:[0-9a-f]{64}$"

var errValidation = errors.New("invalid resource")

type validatorConfig struct {
	expectedImage     string
	expectedInitImage string
	expectedVersion   version.Version
}

func validatePluginImage(image, expectedImageName string, expectedMinVersion *version.Version) error {
	imageRe := regexp.MustCompile(expectedImageName + sha256RE)
	if imageRe.MatchString(image) {
		return nil
	}

	// Ignore registry, vendor and extract the image name with the tag
	parts := strings.SplitN(filepath.Base(image), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w: incorrect image field %q", errValidation, image)
	}

	imageName := parts[0]
	versionStr := parts[1]

	// If user provided faulty SHA digest, the image name may include @sha256 suffix so strip it.
	if strings.TrimSuffix(imageName, "@sha256") != expectedImageName {
		return fmt.Errorf("%w: incorrect image name %q. Make sure you use '<vendor>/%s'", errValidation, imageName, expectedImageName)
	}

	ver, err := version.ParseSemantic(versionStr)
	if err != nil {
		return fmt.Errorf("%w: %w: Make sure it's either valid SHA digest or semver tag", errValidation, err)
	}

	if !ver.AtLeast(expectedMinVersion) {
		return fmt.Errorf("%w: version %q is too low. Should be at least %q", errValidation, ver, expectedMinVersion)
	}

	return nil
}

// DlbDevicePlugin webhook methods

func (r *DlbDevicePlugin) Default(ctx context.Context, obj *DlbDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = dlbDefaultImage
	}
	return nil
}

func (r *DlbDevicePlugin) ValidateCreate(ctx context.Context, obj *DlbDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(dlbValidatorConfig)
}

func (r *DlbDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *DlbDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(dlbValidatorConfig)
}

func (r *DlbDevicePlugin) ValidateDelete(ctx context.Context, obj *DlbDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// DsaDevicePlugin webhook methods

func (r *DsaDevicePlugin) Default(ctx context.Context, obj *DsaDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = dsaDefaultImage
	}
	return nil
}

func (r *DsaDevicePlugin) ValidateCreate(ctx context.Context, obj *DsaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(dsaValidatorConfig)
}

func (r *DsaDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *DsaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(dsaValidatorConfig)
}

func (r *DsaDevicePlugin) ValidateDelete(ctx context.Context, obj *DsaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// FpgaDevicePlugin webhook methods

func (r *FpgaDevicePlugin) Default(ctx context.Context, obj *FpgaDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = fpgaDefaultImage
	}
	return nil
}

func (r *FpgaDevicePlugin) ValidateCreate(ctx context.Context, obj *FpgaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(fpgaValidatorConfig)
}

func (r *FpgaDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *FpgaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(fpgaValidatorConfig)
}

func (r *FpgaDevicePlugin) ValidateDelete(ctx context.Context, obj *FpgaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// GpuDevicePlugin webhook methods

func (r *GpuDevicePlugin) Default(ctx context.Context, obj *GpuDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = gpuDefaultImage
	}
	return nil
}

func (r *GpuDevicePlugin) ValidateCreate(ctx context.Context, obj *GpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(gpuValidatorConfig)
}

func (r *GpuDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *GpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(gpuValidatorConfig)
}

func (r *GpuDevicePlugin) ValidateDelete(ctx context.Context, obj *GpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// IaaDevicePlugin webhook methods

func (r *IaaDevicePlugin) Default(ctx context.Context, obj *IaaDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = iaaDefaultImage
	}
	return nil
}

func (r *IaaDevicePlugin) ValidateCreate(ctx context.Context, obj *IaaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(iaaValidatorConfig)
}

func (r *IaaDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *IaaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(iaaValidatorConfig)
}

func (r *IaaDevicePlugin) ValidateDelete(ctx context.Context, obj *IaaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// NpuDevicePlugin webhook methods

func (r *NpuDevicePlugin) Default(ctx context.Context, obj *NpuDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = npuDefaultImage
	}
	return nil
}

func (r *NpuDevicePlugin) ValidateCreate(ctx context.Context, obj *NpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(npuValidatorConfig)
}

func (r *NpuDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *NpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(npuValidatorConfig)
}

func (r *NpuDevicePlugin) ValidateDelete(ctx context.Context, obj *NpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// QatDevicePlugin webhook methods

func (r *QatDevicePlugin) Default(ctx context.Context, obj *QatDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = qatDefaultImage
	}
	return nil
}

func (r *QatDevicePlugin) ValidateCreate(ctx context.Context, obj *QatDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(qatValidatorConfig)
}

func (r *QatDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *QatDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(qatValidatorConfig)
}

func (r *QatDevicePlugin) ValidateDelete(ctx context.Context, obj *QatDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}

// SgxDevicePlugin webhook methods

func (r *SgxDevicePlugin) Default(ctx context.Context, obj *SgxDevicePlugin) error {
	logf.FromContext(ctx).Info("default", "name", obj.Name)
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = sgxDefaultImage
	}
	return nil
}

func (r *SgxDevicePlugin) ValidateCreate(ctx context.Context, obj *SgxDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create", "name", obj.Name)
	return nil, obj.validatePlugin(sgxValidatorConfig)
}

func (r *SgxDevicePlugin) ValidateUpdate(ctx context.Context, oldObj, newObj *SgxDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update", "name", newObj.Name)
	return nil, newObj.validatePlugin(sgxValidatorConfig)
}

func (r *SgxDevicePlugin) ValidateDelete(ctx context.Context, obj *SgxDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete", "name", obj.Name)
	return nil, nil
}
