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
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/version"
)

// common constants for webhooks.
const imageMinVersion string = "0.21.0"

// common functions for webhooks

func validatePluginImage(image, expectedImageName string, expectedMinVersion *version.Version) error {
	// Ignore registry, vendor and extract the image name with the tag

	parts := strings.SplitN(filepath.Base(image), ":", 2)
	if len(parts) != 2 {
		return errors.Errorf("incorrect image field %q", image)
	}

	imageName := parts[0]
	versionStr := parts[1]

	if imageName != expectedImageName {
		return errors.Errorf("incorrect image name %q. Make sure you use '<vendor>/%s:<version>'", imageName, expectedImageName)
	}

	ver, err := version.ParseSemantic(versionStr)
	if err != nil {
		return errors.Wrapf(err, "unable to parse version %q", versionStr)
	}

	if !ver.AtLeast(expectedMinVersion) {
		return errors.Errorf("version %q is too low. Should be at least %q", ver, expectedMinVersion)
	}

	return nil
}
