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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FpgaRegionSpec contains actual specs for FpgaRegion.
type FpgaRegionSpec struct {
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8,32}$`
	InterfaceID string `json:"interfaceId"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fpga

// FpgaRegion is a specification for a FpgaRegion resource.
type FpgaRegion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FpgaRegionSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// FpgaRegionList is a list of FpgaRegion resources.
type FpgaRegionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FpgaRegion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FpgaRegion{}, &FpgaRegionList{})
}
