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

package patcher

import (
	"flag"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga/v2"
)

func init() {
	_ = flag.Set("v", "4")
}

func checkExpectedError(t *testing.T, expectedErr bool, err error, testName string) {
	t.Helper()

	if expectedErr && err == nil {
		t.Errorf("Test case '%s': no error returned", testName)
	}

	if !expectedErr && err != nil {
		t.Errorf("Test case '%s': unexpected error: %+v", testName, err)
	}
}

func TestPatcherStorageFunctions(t *testing.T) {
	goodAf := &fpgav2.AcceleratorFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name: "arria10-nlb0",
		},
		Spec: fpgav2.AcceleratorFunctionSpec{
			AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
			InterfaceID: "ce48969398f05f33946d560708be108a",
			Mode:        af,
		},
	}
	brokenAf := &fpgav2.AcceleratorFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name: "arria10-nlb0",
		},
		Spec: fpgav2.AcceleratorFunctionSpec{
			AfuID:       "wrong string",
			InterfaceID: "wrong string",
			Mode:        af,
		},
	}
	region := &fpgav2.FpgaRegion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "arria10",
		},
		Spec: fpgav2.FpgaRegionSpec{
			InterfaceID: "ce48969398f05f33946d560708be108a",
		},
	}
	regionAlias := &fpgav2.FpgaRegion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "arria10-alias",
		},
		Spec: fpgav2.FpgaRegionSpec{
			InterfaceID: "ce48969398f05f33946d560708be108a",
		},
	}

	tcases := []struct {
		name                        string
		afsToAdd                    []*fpgav2.AcceleratorFunction
		afsToRemove                 []*fpgav2.AcceleratorFunction
		regionsToAdd                []*fpgav2.FpgaRegion
		regionsToRemove             []*fpgav2.FpgaRegion
		expectedResourceModeMapSize int
		expectedResourceMapSize     int
		expectedIdentitySetSize     int
		expectedErr                 bool
	}{
		{
			name:                        "Add one good af",
			afsToAdd:                    []*fpgav2.AcceleratorFunction{goodAf},
			expectedResourceModeMapSize: 1,
			expectedResourceMapSize:     1,
			expectedIdentitySetSize:     1,
		},
		{
			name:        "Add one broken af",
			afsToAdd:    []*fpgav2.AcceleratorFunction{brokenAf},
			expectedErr: true,
		},
		{
			name:        "Add one and remove one good af",
			afsToAdd:    []*fpgav2.AcceleratorFunction{goodAf},
			afsToRemove: []*fpgav2.AcceleratorFunction{goodAf},
		},
		{
			name:                        "Add one good region",
			regionsToAdd:                []*fpgav2.FpgaRegion{region},
			expectedResourceModeMapSize: 1,
			expectedResourceMapSize:     1,
			expectedIdentitySetSize:     1,
		},
		{
			name:            "Add one and remove one good region",
			regionsToAdd:    []*fpgav2.FpgaRegion{region},
			regionsToRemove: []*fpgav2.FpgaRegion{region},
		},
		{
			name:                        "Add one region and one alias then remove one alias",
			regionsToAdd:                []*fpgav2.FpgaRegion{region, regionAlias},
			regionsToRemove:             []*fpgav2.FpgaRegion{region},
			expectedResourceModeMapSize: 1,
			expectedResourceMapSize:     1,
			expectedIdentitySetSize:     1,
		},
		{
			name:                        "Add one region and one alias",
			regionsToAdd:                []*fpgav2.FpgaRegion{region, regionAlias},
			expectedResourceModeMapSize: 2,
			expectedResourceMapSize:     2,
			expectedIdentitySetSize:     1,
		},
		{
			name:            "Add one region and one alias then remove all",
			regionsToAdd:    []*fpgav2.FpgaRegion{region, regionAlias},
			regionsToRemove: []*fpgav2.FpgaRegion{region, regionAlias},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := newPatcher(ctrl.Log.WithName("test"))
			for _, af := range tt.afsToAdd {
				err := p.AddAf(af)
				checkExpectedError(t, tt.expectedErr, err, tt.name)
			}
			for _, reg := range tt.regionsToAdd {
				p.AddRegion(reg)
			}
			for _, af := range tt.afsToRemove {
				p.RemoveAf(af.Name)
			}
			for _, reg := range tt.regionsToRemove {
				p.RemoveRegion(reg.Name)
			}
			if tt.expectedResourceModeMapSize != len(p.resourceModeMap) {
				t.Errorf("wrong size of resourceModeMap. Expected %d, but got %d", tt.expectedResourceModeMapSize, len(p.resourceModeMap))
			}
			if tt.expectedResourceMapSize != len(p.resourceMap) {
				t.Errorf("wrong size of resourceMap. Expected %d, but got %d", tt.expectedResourceMapSize, len(p.resourceMap))
			}
			if tt.expectedIdentitySetSize != len(p.identitySet) {
				t.Errorf("wrong size of identitySet. Expected %d, but got %d", tt.expectedIdentitySetSize, len(p.identitySet))
			}
		})
	}
}

func TestSanitizeContainerEnv(t *testing.T) {
	tcases := []struct {
		name              string
		container         corev1.Container
		expectedContainer corev1.Container
	}{
		{
			name:              "Container OK",
			container:         corev1.Container{},
			expectedContainer: corev1.Container{},
		},
		{
			name: "Wrong ENV FPGA_AFU",
			container: corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "FPGA_AFU",
						Value: "fake value",
					},
				},
			},
			expectedContainer: corev1.Container{},
		},
		{
			name: "Wrong ENV FPGA_REGION",
			container: corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "FPGA_REGION",
						Value: "fake value",
					},
				},
			},
			expectedContainer: corev1.Container{},
		},
		{
			name: "Wrong ENV FPGA_REGION and unrelated env var",
			container: corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "FPGA_REGION",
						Value: "fake value",
					},
					{
						Name:  "FAKE_VAR",
						Value: "fake value",
					},
				},
			},
			expectedContainer: corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "FAKE_VAR",
						Value: "fake value",
					},
				},
			},
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			c := sanitizeContainer(tt.container)
			if tt.expectedContainer.Env != nil && !reflect.DeepEqual(c.Env, tt.expectedContainer.Env) {
				t.Errorf("Got wrong container Env %+v instead of expected %+v", c.Env, tt.expectedContainer.Env)
			}
			if tt.expectedContainer.Env == nil && c.Env != nil {
				t.Errorf("Got wrong container Env %+v instead of expected 'nil'", c.Env)
			}
		})
	}
}

func TestGetPatchOps(t *testing.T) {
	tcases := []struct {
		name        string
		container   corev1.Container
		afs         []*fpgav2.AcceleratorFunction
		regions     []*fpgav2.FpgaRegion
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
			afs: []*fpgav2.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        region,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0-alias",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
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
			afs: []*fpgav2.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        af,
					},
				},
			},
			expectedOps: 4,
		},
		{
			name: "Skip handling for an identity mapping",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs": resource.MustParse("1"),
					},
				},
			},
			afs: []*fpgav2.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        af,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0-alias",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        af,
					},
				},
			},
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
			regions: []*fpgav2.FpgaRegion{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10",
					},
					Spec: fpgav2.FpgaRegionSpec{
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
			afs: []*fpgav2.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
						AfuID:       "d8424dc4a4a3c413f89e433683f9040b",
						InterfaceID: "ce48969398f05f33946d560708be108a",
						Mode:        region,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb3",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
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
			afs: []*fpgav2.AcceleratorFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: fpgav2.AcceleratorFunctionSpec{
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
			p := newPatcher(ctrl.Log.WithName("test"))
			for _, af := range tt.afs {
				_ = p.AddAf(af)
			}
			for _, region := range tt.regions {
				p.AddRegion(region)
			}
			ops, err := p.getPatchOps(0, tt.container)
			checkExpectedError(t, tt.expectedErr, err, tt.name)
			if len(ops) != tt.expectedOps {
				t.Errorf("test case '%s': expected %d ops, but got %d\n%v", tt.name, tt.expectedOps, len(ops), ops)
			}
		})
	}
}
