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

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FpgaDevicePluginSpec defines the desired state of FpgaDevicePlugin.
type FpgaDevicePluginSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// NodeSelector provides a simple way to constrain device plugin pods to nodes with particular labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Image is a container image with FPGA device plugin executable.
	Image string `json:"image,omitempty"`

	// InitImage is a container image with tools used to initialize the host before starting FPGA workloads on it.
	InitImage string `json:"initImage,omitempty"`

	// Mode is a mode of the plugin's operation.
	// +kubebuilder:validation:Enum=af;region;regiondevel
	Mode string `json:"mode,omitempty"`

	// LogLevel sets the plugin's log level.
	// +kubebuilder:validation:Minimum=0
	LogLevel int `json:"logLevel,omitempty"`
}

// FpgaDevicePluginStatus defines the observed state of FpgaDevicePlugin.
type FpgaDevicePluginStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file

	// ControlledDaemoSet references the DaemonSet controlled by the operator.
	// +optional
	ControlledDaemonSet v1.ObjectReference `json:"controlledDaemonSet,omitempty"`

	// The list of Node names where the device plugin pods are running.
	// +optional
	NodeNames []string `json:"nodeNames,omitempty"`

	// The total number of nodes that should be running the device plugin
	// pod (including nodes correctly running the device plugin pod).
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`

	// The number of nodes that should be running the device plugin pod and have one
	// or more of the device plugin pod running and ready.
	NumberReady int32 `json:"numberReady"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=fpgadeviceplugins,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Node Selector",type=string,JSONPath=`.spec.nodeSelector`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="Intel FPGA Device Plugin"

// FpgaDevicePlugin is the Schema for the fpgadeviceplugins API. It represents
// the FPGA device plugin responsible for advertising Intel FPGA hardware resources to
// the kubelet.
type FpgaDevicePlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FpgaDevicePluginSpec   `json:"spec,omitempty"`
	Status FpgaDevicePluginStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FpgaDevicePluginList contains a list of FpgaDevicePlugin.
type FpgaDevicePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FpgaDevicePlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FpgaDevicePlugin{}, &FpgaDevicePluginList{})
}
