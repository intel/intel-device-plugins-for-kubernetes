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
	tests := []struct {
		image             string
		initimage         string
		expectedImage     string
		expectedInitimage string
		upgrade           bool
	}{
		{
			image:             "intel/intel-dsa-plugin:0.22.0",
			expectedImage:     "intel/intel-dsa-plugin:0.23.0",
			initimage:         "intel/intel-idxd-config-initcontainer:0.22.0",
			expectedInitimage: "intel/intel-idxd-config-initcontainer:0.23.0",
			upgrade:           true,
		},
		{
			image:             "intel/intel-dsa-plugin:0.23.0",
			expectedImage:     "intel/intel-dsa-plugin:0.23.0",
			initimage:         "intel/intel-idxd-config-initcontainer:0.23.0",
			expectedInitimage: "intel/intel-idxd-config-initcontainer:0.23.0",
			upgrade:           false,
		},
		{
			image:             "intel/intel-dsa-plugin:latest",
			expectedImage:     "intel/intel-dsa-plugin:latest",
			initimage:         "intel/intel-idxd-config-initcontainer:latest",
			expectedInitimage: "intel/intel-idxd-config-initcontainer:latest",
			upgrade:           false,
		},
		{
			image:             "intel/intel-dsa-plugin",
			expectedImage:     "intel/intel-dsa-plugin",
			initimage:         "intel/intel-idxd-config-initcontainer",
			expectedInitimage: "intel/intel-idxd-config-initcontainer",
			upgrade:           false,
		},
	}

	for _, t := range tests {
		upgrade := UpgradeImages(&t.image, &t.initimage)

		if !(upgrade == t.upgrade && t.image == t.expectedImage && t.initimage == t.expectedInitimage) {
			test.Errorf("expectedUpgrade: %v, received: %v", t.upgrade, upgrade)
			test.Errorf("expectedImage: %s, received: %s", t.expectedImage, t.image)
			test.Errorf("expectedInitimage: %s, received: %s", t.expectedInitimage, t.initimage)
		}
	}
}
