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

package patcher

import (
	"context"
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga/v2"
)

func init() {
	ctrl.SetLogger(klogr.New())
}

func TestGetPatcher(t *testing.T) {
	log := ctrl.Log.WithName("test")
	namespace := "test"
	tcases := []struct {
		pm   *Manager
		name string
	}{
		{
			name: "Create new patcher",
			pm:   NewPatcherManager(log),
		},
		{
			name: "Return existing patcher",
			pm:   &Manager{patchers: map[string]*Patcher{namespace: newPatcher(log)}},
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.pm.GetPatcher(namespace)
			if p != tt.pm.patchers[namespace] {
				t.Error("stored and received patchers are not equal")
			}
		})
	}
}

func TestMutate(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    "default",
			GenerateName: "goodpod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu":                    resource.MustParse("1"),
							"fpga.intel.com/arria10": resource.MustParse("1"),
						},
						Requests: corev1.ResourceList{
							"cpu":                    resource.MustParse("1"),
							"fpga.intel.com/arria10": resource.MustParse("1"),
						},
					},
				},
			},
		},
	}
	brokenPod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu":                    resource.MustParse("1"),
							"fpga.intel.com/arria10": resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	podRaw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}

	brokenPodRaw, err := json.Marshal(brokenPod)
	if err != nil {
		t.Fatal(err)
	}

	tcases := []struct {
		ar               admissionv1.AdmissionRequest
		name             string
		expectedPatchOps int
		expectedAllowed  bool
	}{
		{
			name: "empty admission request",
			ar:   admissionv1.AdmissionRequest{},
		},
		{
			name: "admission request without object",
			ar: admissionv1.AdmissionRequest{
				Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			},
			expectedAllowed: true,
		},
		{
			name: "admission request with corrupted object",
			ar: admissionv1.AdmissionRequest{
				Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				Object: runtime.RawExtension{
					Raw: []byte(`{"corrupted json":}`),
				},
			},
		},
		{
			name: "successful non-empty admission request",
			ar: admissionv1.AdmissionRequest{
				Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				Object: runtime.RawExtension{
					Raw: podRaw,
				},
			},
			expectedPatchOps: 4,
			expectedAllowed:  true,
		},
		{
			name: "handle error after wrong getPatchOps()",
			ar: admissionv1.AdmissionRequest{
				Namespace: "test",
				Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				Object: runtime.RawExtension{
					Raw: brokenPodRaw,
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			log := ctrl.Log.WithName("test")
			p := newPatcher(log)
			p.AddRegion(&fpgav2.FpgaRegion{
				ObjectMeta: metav1.ObjectMeta{
					Name: "arria10",
				},
				Spec: fpgav2.FpgaRegionSpec{
					InterfaceID: "ce48969398f05f33946d560708be108a",
				},
			})
			pm := NewPatcherManager(log)
			pm.patchers["default"] = p
			resp := pm.GetPodMutator()(context.TODO(), webhook.AdmissionRequest{AdmissionRequest: tcase.ar})

			actualPatchOps := 0
			if tcase.expectedAllowed != resp.Allowed {
				t.Errorf("Allowed expected to be %t but got %t", tcase.expectedAllowed, resp.Allowed)
			} else if resp.Allowed && resp.Patch != nil {
				var ops interface{}

				err := json.Unmarshal(resp.Patch, &ops)
				if err != nil {
					t.Errorf("Test case '%s': got unparsable patch '%s'", tcase.name, resp.Patch)
				} else {
					actualPatchOps = len(ops.([]interface{}))
				}
			}

			if actualPatchOps != tcase.expectedPatchOps {
				t.Errorf("Test case '%s': got wrong number of operations in the patch. Expected %d, but got %d\n%s",
					tcase.name, tcase.expectedPatchOps, actualPatchOps, string(resp.Patch))
			}
		})
	}
}
