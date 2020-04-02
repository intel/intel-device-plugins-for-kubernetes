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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fpgav1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v1"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	flag.Set("v", "4")
}

func fakeMutatePods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	reviewResponse := v1beta1.AdmissionResponse{}
	return &reviewResponse
}

func TestServe(t *testing.T) {
	ar1, err := json.Marshal(&v1beta1.AdmissionReview{})
	if err != nil {
		t.Fatal(err)
	}
	ar2, err := json.Marshal(&v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{},
	})
	if err != nil {
		t.Fatal(err)
	}

	tcases := []struct {
		header         http.Header
		body           io.Reader
		expectedStatus int
	}{
		{
			expectedStatus: http.StatusBadRequest,
		},
		{
			body:           strings.NewReader("hello world"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			body: strings.NewReader("hello world"),
			header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			body: bytes.NewReader(ar1),
			header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			body: bytes.NewReader(ar2),
			header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tcase := range tcases {
		req, err := http.NewRequest("POST", "/pods", tcase.body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header = tcase.header

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serve(w, r, fakeMutatePods) })

		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != tcase.expectedStatus {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, tcase.expectedStatus)
		}

		if tcase.expectedStatus == http.StatusOK {
			var ar v1beta1.AdmissionReview
			err = json.Unmarshal(rr.Body.Bytes(), &ar)
			if err != nil {
				t.Error(err)
			}
		}
	}
}

func TestMutatePods(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
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
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
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
		name             string
		ar               v1beta1.AdmissionReview
		expectedResponse bool
		expectedAllowed  bool
		expectedPatchOps int
	}{
		{
			name: "empty admission request",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{},
			},
		},
		{
			name: "admission request without object",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				},
			},
			expectedResponse: true,
			expectedAllowed:  true,
		},
		{
			name: "admission request with corrupted object",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Object: runtime.RawExtension{
						Raw: []byte(`{"corrupted json":}`),
					},
				},
			},
			expectedResponse: true,
		},
		{
			name: "successful non-empty admission request",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Object: runtime.RawExtension{
						Raw: podRaw,
					},
				},
			},
			expectedResponse: true,
			expectedPatchOps: 4,
			expectedAllowed:  true,
		},
		{
			name: "handle error after wrong getPatchOps()",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Object: runtime.RawExtension{
						Raw: brokenPodRaw,
					},
				},
			},
			expectedResponse: true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			p := newPatcher()
			p.addRegion(&fpgav1.FpgaRegion{
				ObjectMeta: metav1.ObjectMeta{
					Name: "arria10",
				},
				Spec: fpgav1.FpgaRegionSpec{
					InterfaceID: "ce48969398f05f33946d560708be108a",
				},
			})
			pm := newPatcherManager()
			pm["default"] = p
			resp := mutatePods(tcase.ar, pm)

			actualPatchOps := 0
			if !tcase.expectedResponse && resp != nil {
				t.Errorf("Test case '%s': got unexpected response", tcase.name)
			} else if tcase.expectedResponse && resp == nil {
				t.Errorf("Test case '%s': got no response", tcase.name)
			} else if tcase.expectedResponse {
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
			}

			if actualPatchOps != tcase.expectedPatchOps {
				t.Errorf("Test case '%s': got wrong number of operations in the patch. Expected %d, but got %d\n%s",
					tcase.name, tcase.expectedPatchOps, actualPatchOps, string(resp.Patch))
			}
		})
	}
}

type fakeResponseWriter struct {
}

func (*fakeResponseWriter) Header() http.Header {
	return http.Header{}
}

func (*fakeResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (*fakeResponseWriter) WriteHeader(int) {
}

func TestMakePodsHandler(t *testing.T) {
	serveFunc := makePodsHandler(newPatcherManager())
	serveFunc(&fakeResponseWriter{}, &http.Request{})
}
