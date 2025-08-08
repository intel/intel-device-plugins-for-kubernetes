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

	// type switches can have only one type in a case so the same repeats for
	// all xDevicePlugin types.
	// TODO: implement receivers if more complex logic is needed.
	switch v := obj.(type) {
	case *DlbDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
	case *DsaDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
	case *FpgaDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
	case *GpuDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
	case *IaaDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
	case *QatDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
	case *SgxDevicePlugin:
		if len(v.Spec.Image) == 0 {
			v.Spec.Image = r.defaultImage
		}
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
