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

// AcceleratorFunctionSpec contains actual specs for AcceleratorFunction.
type AcceleratorFunctionSpec struct {
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8,40}$`
	AfuID string `json:"afuId"`

	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8,32}$`
	InterfaceID string `json:"interfaceId"`

	// +kubebuilder:validation:Pattern=`^af|region$`
	Mode string `json:"mode"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=af

// AcceleratorFunction is a specification for an AcceleratorFunction resource.
type AcceleratorFunction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AcceleratorFunctionSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// AcceleratorFunctionList is a list of AcceleratorFunction resources.
type AcceleratorFunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AcceleratorFunction `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AcceleratorFunction{}, &AcceleratorFunctionList{})
}
