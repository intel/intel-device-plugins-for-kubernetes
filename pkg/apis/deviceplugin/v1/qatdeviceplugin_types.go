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

// +kubebuilder:validation:Enum={"dh895xccvf","c6xxvf","c3xxxvf","d15xxvf","4xxxvf","c4xxxvf"}

// KernelVfDriver is a VF device driver for QuickAssist devices.
type KernelVfDriver string

// QatDevicePluginSpec defines the desired state of QatDevicePlugin.
type QatDevicePluginSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file.

	// Image is a container image with QAT device plugin executable.
	Image string `json:"image,omitempty"`

	// DpdkDriver is a DPDK device driver for configuring the QAT device.
	// +kubebuilder:validation:Enum=igb_uio;vfio-pci
	DpdkDriver string `json:"dpdkDriver,omitempty"`

	// NodeSelector provides a simple way to constrain device plugin pods to nodes with particular labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// KernelVfDrivers is a list of VF device drivers for the QuickAssist devices in the system.
	KernelVfDrivers []KernelVfDriver `json:"kernelVfDrivers,omitempty"`

	// MaxNumDevices is a maximum number of QAT devices to be provided to the QuickAssist device plugin
	// +kubebuilder:validation:Minimum=1
	MaxNumDevices int `json:"maxNumDevices,omitempty"`

	// LogLevel sets the plugin's log level.
	// +kubebuilder:validation:Minimum=0
	LogLevel int `json:"logLevel,omitempty"`
}

// QatDevicePluginStatus defines the observed state of QatDevicePlugin.
// TODO(rojkov): consider code deduplication with GpuDevicePluginStatus.
type QatDevicePluginStatus struct {
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
// +kubebuilder:resource:path=qatdeviceplugins,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Node Selector",type=string,JSONPath=`.spec.nodeSelector`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="Intel QuickAssist Technology Device Plugin"

// QatDevicePlugin is the Schema for the qatdeviceplugins API. It represents the QAT device
// plugin responsible for advertising Intel QuickAssist Technology hardware resources
// to the kubelet.
type QatDevicePlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status QatDevicePluginStatus `json:"status,omitempty"`
	Spec   QatDevicePluginSpec   `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// QatDevicePluginList contains a list of QatDevicePlugin.
type QatDevicePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QatDevicePlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QatDevicePlugin{}, &QatDevicePluginList{})
}
