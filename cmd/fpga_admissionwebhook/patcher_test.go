// Copyright 2018 Intel Corporation. All Rights Reserved.
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

package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fpgav1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

func init() {
	debug.Activate()
}

func TestPatcherStorageFunctions(t *testing.T) {
	af := &fpgav1.AcceleratorFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name: "arria10-nlb0",
		},
		Spec: fpgav1.AcceleratorFunctionSpec{
			AfuID: "d8424dc4a4a3c413f89e433683f9040b",
		},
	}
	region := &fpgav1.FpgaRegion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "arria10",
		},
		Spec: fpgav1.FpgaRegionSpec{
			InterfaceID: "ce48969398f05f33946d560708be108a",
		},
	}

	p, err := newPatcher(preprogrammed)
	if err != nil {
		t.Fatal(err)
	}

	p.addAf(af)
	if len(p.afMap) != 1 || len(p.resourceMap) != 1 {
		t.Error("Failed to add AF to patcher")
	}

	p.removeAf(af.Name)
	if len(p.afMap) != 0 || len(p.resourceMap) != 0 {
		t.Error("Failed to remove AF from patcher")
	}

	p.addRegion(region)
	if len(p.regionMap) != 1 || len(p.resourceMap) != 1 {
		t.Error("Failed to add fpga region to patcher")
	}

	p.removeRegion(region.Name)
	if len(p.regionMap) != 0 || len(p.resourceMap) != 0 {
		t.Error("Failed to remove fpga region from patcher")
	}
}

func TestGetPatchOpsPreprogrammed(t *testing.T) {
	tcases := []struct {
		name        string
		resourceMap map[string]string
		container   corev1.Container
		expectedErr bool
		expectedOps int
	}{
		{
			name: "Empty container",
		},
		{
			name: "Unknown resource in limits",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-unknown": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown resource in requests",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-unknown": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Successful case",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
				},
			},
			resourceMap: map[string]string{
				"fpga.intel.com/arria10-nlb0": rfc6901Escaper.Replace("fpga.intel.com/af-d8424dc4a4a3c413f89e433683f9040b"),
			},
			expectedOps: 2,
		},
	}
	for _, tt := range tcases {
		p := &patcher{
			resourceMap: tt.resourceMap,
		}
		ops, err := p.getPatchOpsPreprogrammed(0, tt.container)
		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': no error returned", tt.name)
		}
		if !tt.expectedErr && err != nil {
			t.Errorf("Test case '%s': unexpected error %v", tt.name, err)
		}
		if len(ops) != tt.expectedOps {
			t.Errorf("test case '%s': expected %d ops, but got %d\n%v", tt.name, tt.expectedOps, len(ops), ops)
		}
	}
}

func TestParseResourceName(t *testing.T) {
	tcases := []struct {
		input       string
		interfaceID string
		afuID       string
		afMap       map[string]string
		regionMap   map[string]string
		expectedErr bool
	}{
		{
			input: "fpga.intel.com/arria10",
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			interfaceID: "ce48969398f05f33946d560708be108a",
		},
		{
			input: "fpga.intel.com/arria10-unknown",
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			expectedErr: true,
		},
		{
			input:       "fpga.intel.com/unknown",
			expectedErr: true,
		},
		{
			input: "fpga.example.com/something",
		},
		{
			input: "fpga.intel.com/arria10-nlb0",
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			afMap: map[string]string{
				"arria10-nlb0": "d8424dc4a4a3c413f89e433683f9040b",
			},
			interfaceID: "ce48969398f05f33946d560708be108a",
			afuID:       "d8424dc4a4a3c413f89e433683f9040b",
		},
	}

	for num, tt := range tcases {
		p := &patcher{
			afMap:     tt.afMap,
			regionMap: tt.regionMap,
		}
		interfaceID, afuID, err := p.parseResourceName(tt.input)
		if tt.expectedErr {
			if err != nil {
				continue
			} else {
				t.Errorf("In case %d we didn't get error", num)
			}
		}
		if tt.interfaceID != interfaceID || tt.afuID != afuID {
			t.Errorf("In case %d expected (%s, %s), but got (%s, %s)", num, tt.interfaceID, tt.afuID, interfaceID, afuID)
		}
	}
}

func TestGetPatchOpsOrchestrated(t *testing.T) {
	tcases := []struct {
		name        string
		container   corev1.Container
		afMap       map[string]string
		regionMap   map[string]string
		expectedErr bool
		expectedOps int
	}{
		{
			name: "Successful handling",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "SOME_VAR",
						Value: "fake value",
					},
				},
			},
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			afMap: map[string]string{
				"arria10-nlb0": "d8424dc4a4a3c413f89e433683f9040b",
			},
			expectedOps: 5,
		},
		{
			name: "Unequal FPGA resources in Limits and Requests 1",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("2"),
					},
				},
			},
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			afMap: map[string]string{
				"arria10-nlb0": "d8424dc4a4a3c413f89e433683f9040b",
				"arria10-nlb3": "f7df405cbd7acf7222f144b0b93acd18",
			},
			expectedErr: true,
		},
		{
			name: "Unequal FPGA resources in Limits and Requests 2",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("1"),
					},
				},
			},
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			afMap: map[string]string{
				"arria10-nlb0": "d8424dc4a4a3c413f89e433683f9040b",
				"arria10-nlb3": "f7df405cbd7acf7222f144b0b93acd18",
			},
			expectedErr: true,
		},
		{
			name: "Unknown FPGA model in Requests",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/unknown-nlb0": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown AFU in Requests",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-unknown": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown FPGA model in Limits",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/unknown-nlb0": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown AFU in Limits",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-unknown": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Wrong ENV",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "FPGA_REGION",
						Value: "fake value",
					},
				},
			},
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			afMap: map[string]string{
				"arria10-nlb0": "d8424dc4a4a3c413f89e433683f9040b",
			},
			expectedErr: true,
		},
		{
			name: "Wrong type of quantity",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1.1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1.1"),
					},
				},
			},
			regionMap: map[string]string{
				"arria10": "ce48969398f05f33946d560708be108a",
			},
			afMap: map[string]string{
				"arria10-nlb0": "d8424dc4a4a3c413f89e433683f9040b",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		p := &patcher{
			afMap:     tt.afMap,
			regionMap: tt.regionMap,
		}
		debug.Print(tt.name)
		ops, err := p.getPatchOpsOrchestrated(0, tt.container)
		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': no error returned", tt.name)
		}
		if !tt.expectedErr && err != nil {
			t.Errorf("Test case '%s': unexpected error %+v", tt.name, err)
		}
		if len(ops) != tt.expectedOps {
			t.Errorf("test case '%s': expected %d ops, but got %d\n%v", tt.name, tt.expectedOps, len(ops), ops)
		}
	}
}
