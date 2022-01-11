// Copyright 2020-2021 Intel Corporation. All Rights Reserved.
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

// DsaDevicePluginSpec defines the desired state of DsaDevicePlugin.
type DsaDevicePluginSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// NodeSelector provides a simple way to constrain device plugin pods to nodes with particular labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Image is a container image with DSA device plugin executable.
	Image string `json:"image,omitempty"`

	// InitImage is an initcontainer image to configure and enable DSA devices and workqueues with idxd-config (accel-config) utility
	InitImage string `json:"initImage,omitempty"`

	// ProvisioningConfig is a ConfigMap used to pass the DSA devices and workqueues configuration into idxd-config initcontainer.
	ProvisioningConfig string `json:"provisioningConfig,omitempty"`

	// SharedDevNum is a number of containers that can share the same DSA device.
	// +kubebuilder:validation:Minimum=1
	SharedDevNum int `json:"sharedDevNum,omitempty"`

	// LogLevel sets the plugin's log level.
	// +kubebuilder:validation:Minimum=0
	LogLevel int `json:"logLevel,omitempty"`
}

// DsaDevicePluginStatus defines the observed state of DsaDevicePlugin.
type DsaDevicePluginStatus struct {
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
// +kubebuilder:resource:path=dsadeviceplugins,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Node Selector",type=string,JSONPath=`.spec.nodeSelector`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="Intel DSA Device Plugin"

// DsaDevicePlugin is the Schema for the dsadeviceplugins API. It represents
// the DSA device plugin responsible for advertising Intel DSA hardware resources to
// the kubelet.
type DsaDevicePlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DsaDevicePluginSpec   `json:"spec,omitempty"`
	Status DsaDevicePluginStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DsaDevicePluginList contains a list of DsaDevicePlugin.
type DsaDevicePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DsaDevicePlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DsaDevicePlugin{}, &DsaDevicePluginList{})
}
