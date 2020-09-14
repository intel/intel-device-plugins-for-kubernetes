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
	"flag"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func init() {
	_ = flag.Set("v", "4")
}

func TestGetRequestedResources(t *testing.T) {
	tcases := []struct {
		name           string
		namespace      string
		container      corev1.Container
		expectedErr    bool
		expectedResult map[string]int64
	}{
		{
			name:      "Normal case",
			namespace: "device.intel.com",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"device.intel.com/type":  resource.MustParse("1"),
						"device.intel.com/type2": resource.MustParse("2"),
						"cpu":                    resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"device.intel.com/type":  resource.MustParse("1"),
						"device.intel.com/type2": resource.MustParse("2"),
						"cpu":                    resource.MustParse("3"),
					},
				},
			},
			expectedResult: map[string]int64{
				"device.intel.com/type":  1,
				"device.intel.com/type2": 2,
			},
		},
		{
			name:      "Unmatched device",
			namespace: "device2.intel.com",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"device.intel.com/type":  resource.MustParse("1"),
						"device.intel.com/type2": resource.MustParse("2"),
						"cpu":                    resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"device.intel.com/type": resource.MustParse("1"),
						"device.intel.com/typ2": resource.MustParse("2"),
						"cpu":                   resource.MustParse("3"),
					},
				},
			},
			expectedResult: map[string]int64{},
		},
		{
			name:      "Unequal device resources in Limits and Requests 1",
			namespace: "device.intel.com",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"device.intel.com/type": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name:      "Unequal device resources in Limits and Requests 2",
			namespace: "device.intel.com",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"device.intel.com/type": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name:      "Wrong type of quantity",
			namespace: "device.intel.com",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"device.intel.com/type": resource.MustParse("1.1"),
					},
					Requests: corev1.ResourceList{
						"device.intel.com/type": resource.MustParse("1.1"),
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetRequestedResources(tt.container, tt.namespace)
			if tt.expectedErr && err == nil {
				t.Errorf("Test case '%s': no error returned", tt.name)
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Test case '%s': unexpected error: %+v", tt.name, err)
			}
			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("test case '%s': result %+v does not match expected %+v\n", tt.name, result, tt.expectedResult)
			}
		})
	}
}
