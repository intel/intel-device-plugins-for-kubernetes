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
	"testing"
)

func TestUpgrade(test *testing.T) {
	image := "intel/intel-dsa-plugin"
	initimage := "intel/intel-idxd-config-initcontainer"
	version := ":" + ImageMinVersion.String()
	prevVersion := ":" + ImageMinVersion.WithMinor(ImageMinVersion.Minor()-1).String()
	tests := []struct {
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
	}

	for i := range tests {
		t := tests[i]

		upgrade := UpgradeImages(&t.image, &t.initimage)

		if !(upgrade == t.upgrade && t.image == t.expectedImage && t.initimage == t.expectedInitimage) {
			test.Errorf("expectedUpgrade: %v, received: %v", t.upgrade, upgrade)
			test.Errorf("expectedImage: %s, received: %s", t.expectedImage, t.image)
			test.Errorf("expectedInitimage: %s, received: %s", t.expectedInitimage, t.initimage)
		}
	}
}
