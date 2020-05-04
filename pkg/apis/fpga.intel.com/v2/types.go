package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AcceleratorFunction is a specification for an AcceleratorFunction resource
type AcceleratorFunction struct {
	metav1.TypeMeta   `json:"inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AcceleratorFunctionSpec `json:"spec"`
}

// AcceleratorFunctionSpec contains actual specs for AcceleratorFunction
type AcceleratorFunctionSpec struct {
	AfuID       string `json:"afuId"`
	InterfaceID string `json:"interfaceId"`
	Mode        string `json:"mode"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AcceleratorFunctionList is a list of AcceleratorFunction resources
type AcceleratorFunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AcceleratorFunction `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FpgaRegion is a specification for a FpgaRegion resource
type FpgaRegion struct {
	metav1.TypeMeta   `json:"inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FpgaRegionSpec `json:"spec"`
}

// FpgaRegionSpec contains actual specs for FpgaRegion
type FpgaRegionSpec struct {
	InterfaceID string `json:"interfaceId"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FpgaRegionList is a list of FpgaRegion resources
type FpgaRegionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FpgaRegion `json:"items"`
}
