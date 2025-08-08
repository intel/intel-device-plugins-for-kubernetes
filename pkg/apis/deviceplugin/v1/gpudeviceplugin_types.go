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

// GpuDevicePluginSpec defines the desired state of GpuDevicePlugin.
type GpuDevicePluginSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// NodeSelector provides a simple way to constrain device plugin pods to nodes with particular labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Image is a container image with GPU device plugin executable.
	Image string `json:"image,omitempty"`

	// InitImage is a container image with tools (e.g., GPU NFD source hook) installed on each node.
	InitImage string `json:"initImage,omitempty"`

	// AllowIDs is a comma-separated list of PCI IDs of GPU devices that should only be advertised by the plugin.
	// If not set, all devices are advertised.
	// The list can contain IDs in the form of '0x1234,0x49a4,0x50b4'.
	// Cannot be used together with DenyIDs.
	AllowIDs string `json:"allowIDs,omitempty"`

	// DenyIDs is a comma-separated list of PCI IDs of GPU devices that should only be denied by the plugin.
	// If not set, all devices are advertised.
	// The list can contain IDs in the form of '0x1234,0x49a4,0x50b4'.
	// Cannot be used together with AllowIDs.
	DenyIDs string `json:"denyIDs,omitempty"`

	// PreferredAllocationPolicy sets the mode of allocating GPU devices on a node.
	// See documentation for detailed description of the policies. Only valid when SharedDevNum > 1 is set.
	// +kubebuilder:validation:Enum=balanced;packed;none
	PreferredAllocationPolicy string `json:"preferredAllocationPolicy,omitempty"`

	// Specialized nodes (e.g., with accelerators) can be Tainted to make sure unwanted pods are not scheduled on them. Tolerations can be set for the plugin pod to neutralize the Taint.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// SharedDevNum is a number of containers that can share the same GPU device.
	// +kubebuilder:validation:Minimum=1
	SharedDevNum int `json:"sharedDevNum,omitempty"`

	// LogLevel sets the plugin's log level.
	// +kubebuilder:validation:Minimum=0
	LogLevel int `json:"logLevel,omitempty"`

	// EnableMonitoring enables the monitoring resource ('i915_monitoring')
	// which gives access to all GPU devices on given node. Typically used with Intel XPU-Manager.
	EnableMonitoring bool `json:"enableMonitoring,omitempty"`
}

// GpuDevicePluginStatus defines the observed state of GpuDevicePlugin.
// TODO(rojkov): consider code deduplication with QatDevicePluginStatus.
type GpuDevicePluginStatus struct {
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
// +kubebuilder:resource:path=gpudeviceplugins,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Node Selector",type=string,JSONPath=`.spec.nodeSelector`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="Intel GPU Device Plugin"

// GpuDevicePlugin is the Schema for the gpudeviceplugins API. It represents
// the GPU device plugin responsible for advertising Intel GPU hardware resources to
// the kubelet.
type GpuDevicePlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status GpuDevicePluginStatus `json:"status,omitempty"`
	Spec   GpuDevicePluginSpec   `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// GpuDevicePluginList contains a list of GpuDevicePlugin.
type GpuDevicePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GpuDevicePlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GpuDevicePlugin{}, &GpuDevicePluginList{})
}
