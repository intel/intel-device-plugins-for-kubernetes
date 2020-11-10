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
	"k8s.io/apimachinery/pkg/util/version"
)

// common functions for webhooks

func validatePluginImage(image, expectedName string, expectedMinVersion *version.Version) error {
	parts := strings.SplitN(image, ":", 2)
	if len(parts) != 2 {
		return errors.Errorf("incorrect image field %q", image)
	}
	namespacedName := parts[0]
	versionStr := parts[1]

	parts = strings.Split(namespacedName, "/")
	name := parts[len(parts)-1]
	if name != expectedName {
		return errors.Errorf("incorrect image name %q. Make sure you use '<vendor>/%s:<version>'", name, expectedName)
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
