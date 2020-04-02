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
	"flag"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fpgav1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v1"
)

func init() {
	flag.Set("v", "4")
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

	p := newPatcher()

	p.addAf(af)
	if len(p.resourceModeMap) != 1 || len(p.afMap) != 1 || len(p.resourceMap) != 1 {
		t.Error("Failed to add AF to patcher")
	}

	p.removeAf(af.Name)
	if len(p.resourceModeMap) != 0 || len(p.afMap) != 0 || len(p.resourceMap) != 0 {
		t.Error("Failed to remove AF from patcher")
	}

	p.addRegion(region)
	if len(p.resourceModeMap) != 1 || len(p.resourceMap) != 1 {
		t.Error("Failed to add fpga region to patcher")
	}

	p.removeRegion(region.Name)
	if len(p.resourceModeMap) != 0 || len(p.resourceMap) != 0 {
		t.Error("Failed to remove fpga region from patcher")
	}
}

func TestGetPatchOps(t *testing.T) {
	tcases := []struct {
		name        string
		container   corev1.Container
		afs         []*fpgav1.AcceleratorFunction
		regions     []*fpgav1.FpgaRegion
		expectedErr bool
		expectedOps int
	}{
		{
			name: "Successful handling for region mode",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0":       resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb0-alias": resource.MustParse("2"),
						"cpu":                               resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0":       resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb0-alias": resource.MustParse("2"),
						"cpu":                               resource.MustParse("3"),
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "SOME_VAR",
						Value: "fake value",
					},
				},
			},
			afs: []*fpgav1.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav1.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        region,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0-alias",
					},
					Spec: fpgav1.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        region,
					},
				},
			},
			expectedOps: 7,
		},
		{
			name: "Successful handling for af mode",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
					},
				},
			},
			afs: []*fpgav1.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav1.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        af,
					},
				},
			},
			expectedOps: 4,
		},
		{
			name: "Successful handling for regiondevel mode",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10": resource.MustParse("1"),
					},
				},
			},
			regions: []*fpgav1.FpgaRegion{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10",
					},
					Spec: fpgav1.FpgaRegionSpec{
						InterfaceID: "ce48969398f05f33946d560708be108a",
					},
				},
			},
			expectedOps: 4,
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
			expectedErr: true,
		},
		{
			name: "Unequal FPGA resources in Limits and Requests 2",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("2"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown FPGA resources in container requirements",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/unknown-nlb0": resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						"fpga.intel.com/unknown-nlb0": resource.MustParse("1"),
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
						"cpu": resource.MustParse("1"),
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "FPGA_REGION",
						Value: "fake value",
					},
				},
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
			expectedErr: true,
		},
		{
			name: "Require resources operated in af and region modes",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("2"),
						"cpu":                         resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("2"),
						"cpu":                         resource.MustParse("3"),
					},
				},
			},
			afs: []*fpgav1.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav1.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        region,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb3",
					},
					Spec: fpgav1.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "f7df405cbd7acf7222f144b0b93acd18",
						Mode:        af,
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown mode",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
					},
				},
			},
			afs: []*fpgav1.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav1.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        "unknown",
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := newPatcher()
			for _, af := range tt.afs {
				p.addAf(af)
			}
			for _, region := range tt.regions {
				p.addRegion(region)
			}
			ops, err := p.getPatchOps(0, tt.container)
			if tt.expectedErr && err == nil {
				t.Errorf("Test case '%s': no error returned", tt.name)
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Test case '%s': unexpected error: %+v", tt.name, err)
			}
			if len(ops) != tt.expectedOps {
				t.Errorf("test case '%s': expected %d ops, but got %d\n%v", tt.name, tt.expectedOps, len(ops), ops)
			}
		})
	}
}

func TestGetPatcher(t *testing.T) {
	namespace := "test"
	tcases := []struct {
		name string
		pm   patcherManager
	}{
		{
			name: "Create new patcher",
			pm:   newPatcherManager(),
		},
		{
			name: "Return existing patcher",
			pm:   map[string]*patcher{namespace: newPatcher()},
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.pm.getPatcher(namespace)
			if p != tt.pm[namespace] {
				t.Error("stored and received patchers are not equal")
			}
		})
	}
}
