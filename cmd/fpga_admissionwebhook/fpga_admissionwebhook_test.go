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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestParseResourceName(t *testing.T) {
	tcases := []struct {
		input       string
		interfaceID string
		afuID       string
		expectedErr bool
	}{
		{
			input:       "fpga.intel.com/arria10",
			expectedErr: true,
		},
		{
			input:       "fpga.intel.com/unknown",
			expectedErr: true,
		},
		{
			input: "fpga.example.com/something",
		},
		{
			input:       "fpga.intel.com/arria10-nlb0",
			interfaceID: "ce48969398f05f33946d560708be108a",
			afuID:       "d8424dc4a4a3c413f89e433683f9040b",
		},
		{
			input:       "fpga.intel.com/arria10-nlb3",
			interfaceID: "ce48969398f05f33946d560708be108a",
			afuID:       "f7df405cbd7acf7222f144b0b93acd18",
		},
	}

	for num, tt := range tcases {
		interfaceID, afuID, err := parseResourceName(tt.input)
		if tt.expectedErr {
			if err != nil {
				continue
			} else {
				t.Errorf("In case %d we didn't get error", num)
			}
		}
		if tt.interfaceID != interfaceID || tt.afuID != afuID {
			t.Errorf("In case %d expected (%s, %s), but got (%s, %s)", num, tt.interfaceID, tt.afuID, interfaceID, afuID)
		}
	}
}

func TestGetPatchOpsOrchestrated(t *testing.T) {
	tcases := []struct {
		name        string
		container   corev1.Container
		expectedErr bool
		expectedOps int
	}{
		{
			name: "Successful handling",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"cpu": resource.MustParse("1"),
					},
				},
			},
			expectedOps: 3,
		},
		{
			name: "More than one FPGA in Limits",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "More than one FPGA in Requests",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
						"fpga.intel.com/arria10-nlb3": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown FPGA model in Requests",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/unknown-nlb0": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown AFU in Requests",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"fpga.intel.com/arria10-unknown": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown FPGA model in Limitss",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/unknown-nlb0": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unknown AFU in Limits",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-unknown": resource.MustParse("1"),
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Wrong ENV",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"fpga.intel.com/arria10-nlb0": resource.MustParse("1"),
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "FPGA_REGION",
						Value: "fake value",
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		ops, err := getPatchOpsOrchestrated(0, tt.container)
		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': no error returned", tt.name)
		}
		if !tt.expectedErr && err != nil {
			t.Errorf("Test case '%s': unexpected error %v", tt.name, err)
		}
		if len(ops) != tt.expectedOps {
			t.Errorf("test case '%s': expected %d ops, but got %d\n%v", tt.name, tt.expectedOps, len(ops), ops)
		}
	}
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
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu": resource.MustParse("1"),
							"fpga.intel.com/arria10": resource.MustParse("1"),
						},
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("1"),
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

	tcases := []struct {
		name             string
		ar               v1beta1.AdmissionReview
		getPatchOps      getPatchOpsFunc
		expectedResponse bool
		expectedPatchOps int
	}{
		{
			name: "empty admission request",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{},
			},
			getPatchOps: getPatchOpsPreprogrammed,
		},
		{
			name: "admission request without object",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				},
			},
			getPatchOps:      getPatchOpsPreprogrammed,
			expectedResponse: true,
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
			getPatchOps:      getPatchOpsPreprogrammed,
			expectedResponse: true,
		},
		{
			name: "non-empty admission request",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Object: runtime.RawExtension{
						Raw: podRaw,
					},
				},
			},
			getPatchOps:      getPatchOpsPreprogrammed,
			expectedResponse: true,
			expectedPatchOps: 4,
		},
		{
			name: "handle error after wrong getPatchOps()",
			ar: v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					Object: runtime.RawExtension{
						Raw: podRaw,
					},
				},
			},
			getPatchOps: func(int, corev1.Container) ([]string, error) {
				return nil, errors.New("Fake error returned from fake getPatchOps()")
			},
			expectedResponse: true,
		},
	}

	for _, tcase := range tcases {
		resp := mutatePods(tcase.ar, tcase.getPatchOps)

		if !tcase.expectedResponse && resp != nil {
			t.Errorf("Test case '%s': got unexpected response", tcase.name)
		} else if tcase.expectedResponse && resp == nil {
			t.Errorf("Test case '%s': got no response", tcase.name)
		} else if tcase.expectedResponse && tcase.expectedPatchOps > 0 {
			var ops interface{}

			err := json.Unmarshal(resp.Patch, &ops)
			if err != nil {
				t.Errorf("Test case '%s': got unparsable patch '%s'", tcase.name, resp.Patch)
			} else if len(ops.([]interface{})) != tcase.expectedPatchOps {
				t.Errorf("Test case '%s': got wrong number of operations in the patch. Expected %d, but got %d",
					tcase.name, tcase.expectedPatchOps, len(ops.([]interface{})))
			}
		}
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
	tcases := []struct {
		name        string
		mode        string
		expectedErr bool
	}{
		{
			name: "preprogrammed mode",
			mode: preprogrammed,
		},
		{
			name: "orchestrated mode",
			mode: orchestrated,
		},
		{
			name:        "wrong mode",
			mode:        "unparsable",
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		serveFunc, err := makePodsHandler(tt.mode)
		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': no error returned", tt.name)
		}
		if !tt.expectedErr {
			if err != nil {
				t.Errorf("Test case '%s': unexpected error %v", tt.name, err)
			} else {
				serveFunc(&fakeResponseWriter{}, &http.Request{})
			}
		}
	}
}

func TestGetEnvVars(t *testing.T) {
	container := corev1.Container{
		Name:  "test-container",
		Image: "test-image",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"cpu": resource.MustParse("1"),
				"fpga.intel.com/arria10": resource.MustParse("1"),
			},
			Requests: corev1.ResourceList{
				"cpu": resource.MustParse("1"),
				"fpga.intel.com/arria10": resource.MustParse("1"),
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "VARNAME1",
				Value: "2",
				ValueFrom: &corev1.EnvVarSource{
					ResourceFieldRef: &corev1.ResourceFieldSelector{
						Resource: "limits.cpu",
						Divisor:  resource.MustParse("1"),
					},
				},
			},
			{
				Name:  "VARNAME2",
				Value: "4",
				ValueFrom: &corev1.EnvVarSource{
					ResourceFieldRef: &corev1.ResourceFieldSelector{
						Resource: "limits.cpu",
						Divisor:  resource.MustParse("1"),
					},
				},
			},
		},
	}
	expected := `, {"name":"VARNAME1","value":"2","valueFrom":{"resourceFieldRef":{"resource":"limits.cpu","divisor":"1"}}},{"name":"VARNAME2","value":"4","valueFrom":{"resourceFieldRef":{"resource":"limits.cpu","divisor":"1"}}}`
	output, _ := getEnvVars(container)

	if output != expected {
		t.Error("Wrong result: ", output)
	}

	container = corev1.Container{
		Name:  "test-container",
		Image: "test-image",
		Env: []corev1.EnvVar{
			{
				Name:  "FPGA_REGION",
				Value: "fake value",
			},
			{
				Name:  "FPGA_AFU",
				Value: "fake value",
			},
		},
	}
	output2, err := getEnvVars(container)

	if len(output2) > 0 || err == nil {
		t.Error("Expected empty string, but got ", output2)
	}
}
