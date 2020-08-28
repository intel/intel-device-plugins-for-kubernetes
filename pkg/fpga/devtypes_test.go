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

package fpga

import (
	"flag"
	"testing"
)

func init() {
	_ = flag.Set("v", "4")
}

func TestGetAfuDevType(t *testing.T) {
	tcases := []struct {
		name            string
		interfaceID     string
		afuID           string
		expectedDevType string
		expectedErr     bool
	}{
		{
			name:            "Everything is correct 1",
			interfaceID:     "ce48969398f05f33946d560708be108a",
			afuID:           "d8424dc4a4a3c413f89e433683f9040b",
			expectedDevType: "af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs",
		},
		{
			name:            "Everything is correct 2",
			interfaceID:     "bfac4d851ee856fe8c95865ce1bbaa2d",
			afuID:           "f7df405cbd7acf7222f144b0b93acd18",
			expectedDevType: "af-bfa.f7d.v6xNhR7oVv6MlYZc4buqLfffQFy9es9yIvFEsLk6zRg",
		},
		{
			name:        "unparsable interfaceID",
			interfaceID: "unparsable",
			expectedErr: true,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			devtype, err := GetAfuDevType(tt.interfaceID, tt.afuID)
			if tt.expectedErr && err == nil {
				t.Errorf("no error returned")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if tt.expectedDevType != devtype {
				t.Errorf("expected %q, but got %q", tt.expectedDevType, devtype)
			}
		})
	}
}
