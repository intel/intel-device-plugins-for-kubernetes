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
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"

	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
)

const (
	resourceReplaceOp = `{
                "op": "remove",
                "path": "/spec/containers/%d/resources/%s/%s"
        }, {
                "op": "add",
                "path": "/spec/containers/%d/resources/%s/%s",
                "value": %s
        }`
	envAddOp = `{
                "op": "add",
                "path": "/spec/containers/%d/env",
                "value": [{
                        "name": "FPGA_REGION",
                        "value": "%s"
                }, {
                        "name": "FPGA_AFU",
                        "value": "%s"
                } %s]
        }`
	preprogrammed = "preprogrammed"
	orchestrated  = "orchestrated"
)

var (
	scheme         = runtime.NewScheme()
	codecs         = serializer.NewCodecFactory(scheme)
	rfc6901Escaper = strings.NewReplacer("~", "~0", "/", "~1")
	resourceRe     = regexp.MustCompile(`intel.com/fpga-(?P<Region>[[:alnum:]]+)(-(?P<Af>[[:alnum:]]+))?`)
)

func init() {
	addToScheme(scheme)
}

func addToScheme(scheme *runtime.Scheme) {
	corev1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)
}

// TODO: get rid of hardcoded translations of FPGA resource names to region interface IDs
func region2interfaceID(regionName string) (string, error) {
	switch strings.ToLower(regionName) {
	case "arria10":
		return "ce48969398f05f33946d560708be108a", nil
	}

	return "", fmt.Errorf("Unknown region name: %s", regionName)
}

// TODO: get rid of hardcoded translations of FPGA resource names to region interface IDs
func af2afuID(afName string) (string, error) {
	switch strings.ToLower(afName) {
	case "nlb0":
		return "d8424dc4a4a3c413f89e433683f9040b", nil
	case "nlb3":
		return "f7df405cbd7acf7222f144b0b93acd18", nil
	}

	return "", fmt.Errorf("Unknown AF name: %s", afName)
}

func parseResourceName(input string) (string, string, error) {
	var interfaceID string
	var afuID string
	var err error

	result := resourceRe.FindStringSubmatch(input)
	if result == nil {
		return "", "", nil
	}

	for num, group := range resourceRe.SubexpNames() {
		switch group {
		case "Region":
			interfaceID, err = region2interfaceID(result[num])
			if err != nil {
				return "", "", err
			}
		case "Af":
			afuID, err = af2afuID(result[num])
			if err != nil {
				return "", "", err
			}
		}
	}

	return interfaceID, afuID, nil
}

// TODO: get rid of hardcoded translations of FPGA resource names to region interface IDs
func translateFpgaResourceName(oldname corev1.ResourceName) string {
	switch strings.ToLower(string(oldname)) {
	case "intel.com/fpga-arria10":
		return rfc6901Escaper.Replace("intel.com/fpga-region-ce48969398f05f33946d560708be108a")
	case "intel.com/fpga-arria10-nlb0":
		return rfc6901Escaper.Replace("intel.com/fpga-af-d8424dc4a4a3c413f89e433683f9040b")
	case "intel.com/fpga-arria10-nlb3":
		return rfc6901Escaper.Replace("intel.com/fpga-af-f7df405cbd7acf7222f144b0b93acd18")
	}

	return ""
}

func getTLSConfig(certFile string, keyFile string) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		glog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

func getPatchOpsPreprogrammed(containerIdx int, container corev1.Container) ([]string, error) {
	var ops []string

	for resourceName, resourceQuantity := range container.Resources.Limits {
		newName := translateFpgaResourceName(resourceName)
		if len(newName) > 0 {
			op := fmt.Sprintf(resourceReplaceOp, containerIdx,
				"limits", rfc6901Escaper.Replace(string(resourceName)),
				containerIdx, "limits", newName, resourceQuantity.String())
			ops = append(ops, op)
		}
	}
	for resourceName, resourceQuantity := range container.Resources.Requests {
		newName := translateFpgaResourceName(resourceName)
		if len(newName) > 0 {
			op := fmt.Sprintf(resourceReplaceOp, containerIdx,
				"requests", rfc6901Escaper.Replace(string(resourceName)),
				containerIdx, "requests", newName, resourceQuantity.String())
			ops = append(ops, op)
		}
	}

	return ops, nil
}

func getEnvVars(container corev1.Container) (string, error) {
	var jsonstrings []string
	for _, envvar := range container.Env {
		if envvar.Name == "FPGA_REGION" || envvar.Name == "FPGA_AFU" {
			return "", fmt.Errorf("The env var '%s' is not allowed", envvar.Name)
		}
		jsonbytes, err := json.Marshal(envvar)
		if err != nil {
			return "", err
		}
		jsonstrings = append(jsonstrings, string(jsonbytes))
	}

	if len(jsonstrings) == 0 {
		return "", nil
	}

	return fmt.Sprintf(", %s", strings.Join(jsonstrings, ",")), nil
}

func getPatchOpsOrchestrated(containerIdx int, container corev1.Container) ([]string, error) {
	var ops []string

	mutated := false
	for resourceName, resourceQuantity := range container.Resources.Limits {
		interfaceID, afuID, err := parseResourceName(string(resourceName))
		if err != nil {
			return nil, err
		}

		if interfaceID == "" && afuID == "" {
			continue
		}

		if mutated {
			return nil, fmt.Errorf("Only one FPGA resource per container is supported in '%s' mode", orchestrated)
		}

		op := fmt.Sprintf(resourceReplaceOp, containerIdx, "limits", rfc6901Escaper.Replace(string(resourceName)),
			containerIdx, "limits", rfc6901Escaper.Replace("intel.com/fpga-region-"+interfaceID), resourceQuantity.String())
		ops = append(ops, op)

		oldVars, err := getEnvVars(container)
		if err != nil {
			return nil, err
		}
		op = fmt.Sprintf(envAddOp, containerIdx, interfaceID, afuID, oldVars)
		ops = append(ops, op)
		mutated = true
	}

	mutated = false
	for resourceName, resourceQuantity := range container.Resources.Requests {
		interfaceID, _, err := parseResourceName(string(resourceName))
		if err != nil {
			return nil, err
		}

		if interfaceID == "" {
			continue
		}

		if mutated {
			return nil, fmt.Errorf("Only one FPGA resource per container is supported in '%s' mode", orchestrated)
		}

		op := fmt.Sprintf(resourceReplaceOp, containerIdx, "requests", rfc6901Escaper.Replace(string(resourceName)),
			containerIdx, "requests", rfc6901Escaper.Replace("intel.com/fpga-region-"+interfaceID), resourceQuantity.String())
		ops = append(ops, op)
		mutated = true
	}

	return ops, nil
}

type getPatchOpsFunc func(int, corev1.Container) ([]string, error)

func mutatePods(ar v1beta1.AdmissionReview, getPatchOps getPatchOpsFunc) *v1beta1.AdmissionResponse {
	var ops []string

	glog.V(2).Info("mutating pods")

	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		glog.Errorf("expect resource to be %s", podResource)
		return nil
	}

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	for containerIdx, container := range pod.Spec.Containers {
		patchOps, err := getPatchOps(containerIdx, container)
		if err != nil {
			return toAdmissionResponse(err)
		}
		ops = append(ops, patchOps...)
	}

	if len(ops) > 0 {
		reviewResponse.Patch = []byte("[ " + strings.Join(ops, ",") + " ]")
		pt := v1beta1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
	}

	return &reviewResponse
}

type admitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var body []byte
	var reviewResponse *v1beta1.AdmissionResponse
	var reqUID types.UID

	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		glog.Error("No body in request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("contentType=%s, expect application/json", contentType)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	glog.V(2).Info(fmt.Sprintf("handling request: %v", body))
	ar := v1beta1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Error(err)
		reviewResponse = toAdmissionResponse(err)
	} else {
		if ar.Request == nil {
			err = errors.New("Request is empty")
			reviewResponse = toAdmissionResponse(err)
		} else {
			reqUID = ar.Request.UID
			reviewResponse = admit(ar)
		}
	}
	glog.V(2).Info(fmt.Sprintf("sending response: %v", reviewResponse))

	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = reqUID
	}

	// reset the Object and OldObject, they are not needed in a response.
	if ar.Request != nil {
		ar.Request.Object = runtime.RawExtension{}
		ar.Request.OldObject = runtime.RawExtension{}
	}

	resp, err := json.Marshal(response)
	if err != nil {
		glog.Error(err)
		return
	}
	if _, err := w.Write(resp); err != nil {
		glog.Error(err)
	}
}

func makePodsHandler(mode string) (func(w http.ResponseWriter, r *http.Request), error) {
	var getPatchOps getPatchOpsFunc

	switch mode {
	case preprogrammed:
		getPatchOps = getPatchOpsPreprogrammed
	case orchestrated:
		getPatchOps = getPatchOpsOrchestrated
	default:
		return nil, fmt.Errorf("Wrong mode: %s", mode)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, func(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
			return mutatePods(ar, getPatchOps)
		})
	}, nil
}

func main() {
	var certFile string
	var keyFile string
	var mode string

	flag.StringVar(&certFile, "tls-cert-file", certFile,
		"File containing the x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	flag.StringVar(&keyFile, "tls-private-key-file", keyFile, "File containing the x509 private key matching --tls-cert-file.")
	flag.StringVar(&mode, "mode", preprogrammed, fmt.Sprintf("webhook mode: '%s' (default) or '%s'", preprogrammed, orchestrated))

	flag.Parse()

	if certFile == "" {
		glog.Error("TLS certificate file is not set")
		os.Exit(1)
	}

	if keyFile == "" {
		glog.Error("TLS private key is not set")
		os.Exit(1)
	}

	if _, err := os.Stat(certFile); err != nil {
		glog.Error("TLS certificate not found")
		os.Exit(1)
	}

	if _, err := os.Stat(keyFile); err != nil {
		glog.Error("TLS private key not found")
		os.Exit(1)
	}

	servePods, err := makePodsHandler(mode)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
	http.HandleFunc("/pods", servePods)

	glog.V(2).Info("Webhook started")

	server := &http.Server{
		Addr:      ":443",
		TLSConfig: getTLSConfig(certFile, keyFile),
	}

	glog.Fatal(server.ListenAndServeTLS("", ""))
}
