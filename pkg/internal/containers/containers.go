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

package containers

import (
	"strings"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
)

// GetRequestedResources validates the container's requirements first, then returns them as a map.
func GetRequestedResources(container corev1.Container, ns string) (map[string]int64, error) {
	// Container may happen to have Requests, but not Limits. Check Requests first,
	// then in the next loop iterate over Limits.
	for resourceName, resourceQuantity := range container.Resources.Requests {
		rname := strings.ToLower(string(resourceName))
		if !strings.HasPrefix(rname, ns) {
			continue
		}

		if container.Resources.Limits[resourceName] != resourceQuantity {
			return nil, errors.Errorf(
				"'limits' and 'requests' for %q must be equal as extended resources cannot be overcommitted",
				rname)
		}
	}

	resources := make(map[string]int64)
	for resourceName, resourceQuantity := range container.Resources.Limits {
		rname := strings.ToLower(string(resourceName))
		if !strings.HasPrefix(rname, ns) {
			continue
		}

		if container.Resources.Requests[resourceName] != resourceQuantity {
			return nil, errors.Errorf(
				"'limits' and 'requests' for %q must be equal as extended resources cannot be overcommitted",
				rname)
		}

		quantity, ok := resourceQuantity.AsInt64()
		if !ok {
			return nil, errors.Errorf("resource quantity isn't of integral type for %q", rname)
		}

		resources[rname] = quantity
	}

	return resources, nil
}
