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

// DlbDevicePluginSpec defines the desired state of DlbDevicePlugin.
type DlbDevicePluginSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// NodeSelector provides a simple way to constrain device plugin pods to nodes with particular labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Image is a container image with DLB device plugin executable.
	Image string `json:"image,omitempty"`

	// InitImage is a container image with a script that initializes devices.
	InitImage string `json:"initImage,omitempty"`

	// Specialized nodes (e.g., with accelerators) can be Tainted to make sure unwanted pods are not scheduled on them. Tolerations can be set for the plugin pod to neutralize the Taint.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// LogLevel sets the plugin's log level.
	// +kubebuilder:validation:Minimum=0
	LogLevel int `json:"logLevel,omitempty"`
}

// DlbDevicePluginStatus defines the observed state of DlbDevicePlugin.
type DlbDevicePluginStatus struct {
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
// +kubebuilder:resource:path=dlbdeviceplugins,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Node Selector",type=string,JSONPath=`.spec.nodeSelector`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="Intel DLB Device Plugin"

// DEPRECATED: DlbDevicePlugin is the Schema for the dlbdeviceplugins API. It represents
// the DLB device plugin responsible for advertising Intel DLB hardware resources to
// the kubelet.
type DlbDevicePlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DlbDevicePluginSpec   `json:"spec,omitempty"`
	Status DlbDevicePluginStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DlbDevicePluginList contains a list of DlbDevicePlugin.
type DlbDevicePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DlbDevicePlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DlbDevicePlugin{}, &DlbDevicePluginList{})
}
