// Copyright 2022 Intel Corporation. All Rights Reserved.
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

package controllers

import (
	"context"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestUpgrade(test *testing.T) {
	image := "intel/intel-dsa-plugin"
	initimage := "intel/intel-idxd-config-initcontainer"
	version := ":" + ImageMinVersion.String()
	prevVersion := ":" + ImageMinVersion.WithMinor(ImageMinVersion.Minor()-1).String()
	tests := []struct {
		envVars           map[string]string
		image             string
		initimage         string
		expectedImage     string
		expectedInitimage string
		upgrade           bool
	}{
		{
			image:             image + prevVersion,
			expectedImage:     image + version,
			initimage:         initimage + prevVersion,
			expectedInitimage: initimage + version,
			upgrade:           true,
		},
		{
			image:             image + version,
			expectedImage:     image + version,
			initimage:         initimage + version,
			expectedInitimage: initimage + version,
			upgrade:           false,
		},
		{
			image:             image + ":devel",
			expectedImage:     image + ":devel",
			initimage:         initimage + ":devel",
			expectedInitimage: initimage + ":devel",
			upgrade:           false,
		},
		{
			image:             image,
			expectedImage:     image,
			initimage:         initimage,
			expectedInitimage: initimage,
			upgrade:           false,
		},
		{
			envVars: map[string]string{
				"INTEL_DSA_PLUGIN_SHA":                "intel/intel-dsa-plugin@sha256:000000000000000000000000000000000000000000000000000000000000000b",
				"INTEL_IDXD_CONFIG_INITCONTAINER_SHA": "intel/intel-idxd-config-initcontainer@sha256:000000000000000000000000000000000000000000000000000000000000000b",
			},
			image:             image + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedImage:     image + "@sha256:000000000000000000000000000000000000000000000000000000000000000b",
			initimage:         initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedInitimage: initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000b",
			upgrade:           true,
		},
		{
			envVars: map[string]string{
				"INTEL_DSA_PLUGIN_SHA":                "intel/intel-dsa-plugin@sha256:000000000000000000000000000000000000000000000000000000000000000a",
				"INTEL_IDXD_CONFIG_INITCONTAINER_SHA": "intel/intel-idxd-config-initcontainer@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			},
			image:             image + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedImage:     image + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			initimage:         initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedInitimage: initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			upgrade:           false,
		},
	}

	for i := range tests {
		t := tests[i]

		for key, value := range t.envVars {
			os.Setenv(key, value)
		}

		upgrade := UpgradeImages(context.Background(), &t.image, &t.initimage)

		if !(upgrade == t.upgrade && t.image == t.expectedImage && t.initimage == t.expectedInitimage) {
			test.Errorf("expectedUpgrade: %v, received: %v", t.upgrade, upgrade)
			test.Errorf("expectedImage: %s, received: %s", t.expectedImage, t.image)
			test.Errorf("expectedInitimage: %s, received: %s", t.expectedInitimage, t.initimage)
		}

		for key := range t.envVars {
			os.Unsetenv(key)
		}
	}
}

func TestSuffixedName(test *testing.T) {
	result := SuffixedName("name", "suffix")

	if result != "name-suffix" {
		test.Errorf("invalid suffixed name received: %v", result)
	}
}

func TestHasTolerationsChanged(test *testing.T) {
	tests := []struct {
		desc    string
		pre     []v1.Toleration
		post    []v1.Toleration
		changed bool
	}{
		{
			desc:    "no tolerations",
			pre:     nil,
			post:    nil,
			changed: false,
		},
		{
			desc: "from tolerations to nothing",
			pre: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			post:    nil,
			changed: true,
		},
		{
			desc: "from nothing to tolerations",
			pre:  nil,
			post: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			changed: true,
		},
		{
			desc: "no changes",
			pre: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			post: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			changed: false,
		},
		{
			desc: "tolerations changed",
			pre: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			post: []v1.Toleration{
				{Key: "foo2", Value: "bar2", Operator: "Equal", Effect: "NoSchedule"},
			},
			changed: true,
		},
	}

	for i := range tests {
		t := tests[i]

		changed := HasTolerationsChanged(t.pre, t.post)

		if changed != t.changed {
			test.Errorf("test: %s: expected: %v, received: %v", t.desc, t.changed, changed)
		}
	}
}
