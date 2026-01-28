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

	"k8s.io/apimachinery/pkg/runtime"
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

var _ admission.CustomDefaulter = &commonDevicePluginDefaulter{}

type commonDevicePluginValidator struct {
	expectedImage     string
	expectedInitImage string
	expectedVersion   version.Version
}

var _ admission.CustomValidator = &commonDevicePluginValidator{}

// Default implements admission.CustomDefaulter so a webhook will be registered for the type.
func (r *commonDevicePluginDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	logf.FromContext(ctx).Info("default")

	setDefaultImageIfNeeded := func(image *string) {
		if len(*image) == 0 {
			*image = r.defaultImage
		}
	}

	// type switches can have only one type in a case so the same repeats for
	// all xDevicePlugin types.
	// TODO: implement receivers if more complex logic is needed.
	switch v := obj.(type) {
	case *DlbDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *DsaDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *FpgaDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *GpuDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *IaaDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *QatDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *SgxDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	case *NpuDevicePlugin:
		setDefaultImageIfNeeded(&v.Spec.Image)
	default:
		return fmt.Errorf("%w: expected an xDevicePlugin object but got %T", errObjType, obj)
	}

	return nil
}

// ValidateCreate implements admission.CustomValidator so a webhook will be registered for the type.
func (r *commonDevicePluginValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate create")

	switch v := obj.(type) {
	case *DlbDevicePlugin:
		return nil, v.validatePlugin(r)
	case *DsaDevicePlugin:
		return nil, v.validatePlugin(r)
	case *GpuDevicePlugin:
		return nil, v.validatePlugin(r)
	case *FpgaDevicePlugin:
		return nil, v.validatePlugin(r)
	case *IaaDevicePlugin:
		return nil, v.validatePlugin(r)
	case *QatDevicePlugin:
		return nil, v.validatePlugin(r)
	case *SgxDevicePlugin:
		return nil, v.validatePlugin(r)
	case *NpuDevicePlugin:
		return nil, v.validatePlugin(r)
	default:
		return nil, fmt.Errorf("%w: expected an xDevicePlugin object but got %T", errObjType, obj)
	}
}

// ValidateUpdate implements admission.CustomValidator so a webhook will be registered for the type.
func (r *commonDevicePluginValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	logf.FromContext(ctx).Info("validate update")

	switch v := newObj.(type) {
	case *DlbDevicePlugin:
		return nil, v.validatePlugin(r)
	case *DsaDevicePlugin:
		return nil, v.validatePlugin(r)
	case *GpuDevicePlugin:
		return nil, v.validatePlugin(r)
	case *FpgaDevicePlugin:
		return nil, v.validatePlugin(r)
	case *IaaDevicePlugin:
		return nil, v.validatePlugin(r)
	case *QatDevicePlugin:
		return nil, v.validatePlugin(r)
	case *SgxDevicePlugin:
		return nil, v.validatePlugin(r)
	case *NpuDevicePlugin:
		return nil, v.validatePlugin(r)
	default:
		return nil, fmt.Errorf("%w: expected an xDevicePlugin object but got %T", errObjType, newObj)
	}
}

// ValidateDelete implements admission.CustomValidator so a webhook will be registered for the type.
func (r *commonDevicePluginValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
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

// Type-specific defaulter wrappers for the new generic Defaulter[T] interface

type dlbDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *dlbDevicePluginDefaulter) Default(ctx context.Context, obj *DlbDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type dsaDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *dsaDevicePluginDefaulter) Default(ctx context.Context, obj *DsaDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type fpgaDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *fpgaDevicePluginDefaulter) Default(ctx context.Context, obj *FpgaDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type gpuDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *gpuDevicePluginDefaulter) Default(ctx context.Context, obj *GpuDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type iaaDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *iaaDevicePluginDefaulter) Default(ctx context.Context, obj *IaaDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type npuDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *npuDevicePluginDefaulter) Default(ctx context.Context, obj *NpuDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type qatDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *qatDevicePluginDefaulter) Default(ctx context.Context, obj *QatDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

type sgxDevicePluginDefaulter struct{ *commonDevicePluginDefaulter }

func (d *sgxDevicePluginDefaulter) Default(ctx context.Context, obj *SgxDevicePlugin) error {
	return d.commonDevicePluginDefaulter.Default(ctx, obj)
}

// Type-specific validator wrappers for the new generic Validator[T] interface

type dlbDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *dlbDevicePluginValidator) ValidateCreate(ctx context.Context, obj *DlbDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *dlbDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *DlbDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *dlbDevicePluginValidator) ValidateDelete(ctx context.Context, obj *DlbDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type dsaDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *dsaDevicePluginValidator) ValidateCreate(ctx context.Context, obj *DsaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *dsaDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *DsaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *dsaDevicePluginValidator) ValidateDelete(ctx context.Context, obj *DsaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type fpgaDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *fpgaDevicePluginValidator) ValidateCreate(ctx context.Context, obj *FpgaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *fpgaDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *FpgaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *fpgaDevicePluginValidator) ValidateDelete(ctx context.Context, obj *FpgaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type gpuDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *gpuDevicePluginValidator) ValidateCreate(ctx context.Context, obj *GpuDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *gpuDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *GpuDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *gpuDevicePluginValidator) ValidateDelete(ctx context.Context, obj *GpuDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type iaaDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *iaaDevicePluginValidator) ValidateCreate(ctx context.Context, obj *IaaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *iaaDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *IaaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *iaaDevicePluginValidator) ValidateDelete(ctx context.Context, obj *IaaDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type npuDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *npuDevicePluginValidator) ValidateCreate(ctx context.Context, obj *NpuDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *npuDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *NpuDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *npuDevicePluginValidator) ValidateDelete(ctx context.Context, obj *NpuDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type qatDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *qatDevicePluginValidator) ValidateCreate(ctx context.Context, obj *QatDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *qatDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *QatDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *qatDevicePluginValidator) ValidateDelete(ctx context.Context, obj *QatDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}

type sgxDevicePluginValidator struct{ *commonDevicePluginValidator }

func (v *sgxDevicePluginValidator) ValidateCreate(ctx context.Context, obj *SgxDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateCreate(ctx, obj)
}

func (v *sgxDevicePluginValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *SgxDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateUpdate(ctx, oldObj, newObj)
}

func (v *sgxDevicePluginValidator) ValidateDelete(ctx context.Context, obj *SgxDevicePlugin) (admission.Warnings, error) {
	return v.commonDevicePluginValidator.ValidateDelete(ctx, obj)
}
