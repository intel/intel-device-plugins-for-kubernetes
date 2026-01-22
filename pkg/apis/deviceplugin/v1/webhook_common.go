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

var errObjType = errors.New("invalid object")
var errValidation = errors.New("invalid resource")

// common functions for webhooks

type commonDevicePluginDefaulter struct {
	defaultImage string
}

type commonDevicePluginValidator struct {
	expectedImage     string
	expectedInitImage string
	expectedVersion   version.Version
}

// Type-specific defaulters
type dlbDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type dsaDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type fpgaDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type gpuDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type iaaDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type qatDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type sgxDevicePluginDefaulter struct{ commonDevicePluginDefaulter }
type npuDevicePluginDefaulter struct{ commonDevicePluginDefaulter }

// Type-specific validators
type dlbDevicePluginValidator struct{ commonDevicePluginValidator }
type dsaDevicePluginValidator struct{ commonDevicePluginValidator }
type fpgaDevicePluginValidator struct{ commonDevicePluginValidator }
type gpuDevicePluginValidator struct{ commonDevicePluginValidator }
type iaaDevicePluginValidator struct{ commonDevicePluginValidator }
type qatDevicePluginValidator struct{ commonDevicePluginValidator }
type sgxDevicePluginValidator struct{ commonDevicePluginValidator }
type npuDevicePluginValidator struct{ commonDevicePluginValidator }

// Default implementations for each type
func (r *dlbDevicePluginDefaulter) Default(ctx context.Context, obj *DlbDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *dsaDevicePluginDefaulter) Default(ctx context.Context, obj *DsaDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *fpgaDevicePluginDefaulter) Default(ctx context.Context, obj *FpgaDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *gpuDevicePluginDefaulter) Default(ctx context.Context, obj *GpuDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *iaaDevicePluginDefaulter) Default(ctx context.Context, obj *IaaDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *qatDevicePluginDefaulter) Default(ctx context.Context, obj *QatDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *sgxDevicePluginDefaulter) Default(ctx context.Context, obj *SgxDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

func (r *npuDevicePluginDefaulter) Default(ctx context.Context, obj *NpuDevicePlugin) error {
	logf.FromContext(ctx).Info("default")
	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = r.defaultImage
	}
	return nil
}

// ValidateCreate implementations for each type
func (r *dlbDevicePluginValidator) ValidateCreate(ctx context.Context, obj *DlbDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *dsaDevicePluginValidator) ValidateCreate(ctx context.Context, obj *DsaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *fpgaDevicePluginValidator) ValidateCreate(ctx context.Context, obj *FpgaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *gpuDevicePluginValidator) ValidateCreate(ctx context.Context, obj *GpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *iaaDevicePluginValidator) ValidateCreate(ctx context.Context, obj *IaaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *qatDevicePluginValidator) ValidateCreate(ctx context.Context, obj *QatDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *sgxDevicePluginValidator) ValidateCreate(ctx context.Context, obj *SgxDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *npuDevicePluginValidator) ValidateCreate(ctx context.Context, obj *NpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")
	return nil, obj.validatePlugin(&r.commonDevicePluginValidator)
}

// ValidateUpdate implementations for each type
func (r *dlbDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *DlbDevicePlugin, newObj *DlbDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *dsaDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *DsaDevicePlugin, newObj *DsaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *fpgaDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *FpgaDevicePlugin, newObj *FpgaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *gpuDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *GpuDevicePlugin, newObj *GpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *iaaDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *IaaDevicePlugin, newObj *IaaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *qatDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *QatDevicePlugin, newObj *QatDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *sgxDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *SgxDevicePlugin, newObj *SgxDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

func (r *npuDevicePluginValidator) ValidateUpdate(ctx context.Context, _ *NpuDevicePlugin, newObj *NpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")
	return nil, newObj.validatePlugin(&r.commonDevicePluginValidator)
}

// ValidateDelete implementations for each type
func (r *dlbDevicePluginValidator) ValidateDelete(ctx context.Context, obj *DlbDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *dsaDevicePluginValidator) ValidateDelete(ctx context.Context, obj *DsaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *fpgaDevicePluginValidator) ValidateDelete(ctx context.Context, obj *FpgaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *gpuDevicePluginValidator) ValidateDelete(ctx context.Context, obj *GpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *iaaDevicePluginValidator) ValidateDelete(ctx context.Context, obj *IaaDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *qatDevicePluginValidator) ValidateDelete(ctx context.Context, obj *QatDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *sgxDevicePluginValidator) ValidateDelete(ctx context.Context, obj *SgxDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
}

func (r *npuDevicePluginValidator) ValidateDelete(ctx context.Context, obj *NpuDevicePlugin) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate delete")
	return nil, nil
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
