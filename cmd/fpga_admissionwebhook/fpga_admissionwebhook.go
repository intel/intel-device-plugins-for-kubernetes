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
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	preprogrammed       = "preprogrammed"
	orchestrated        = "orchestrated"
	controllerThreadNum = 1
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	addToScheme(scheme)
}

func addToScheme(scheme *runtime.Scheme) {
	corev1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)
}

func getTLSConfig(certFile string, keyFile string) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

func mutatePods(ar v1beta1.AdmissionReview, p *patcher) *v1beta1.AdmissionResponse {
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
		fmt.Printf("ERROR: %+v\n", err)
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	for containerIdx, container := range pod.Spec.Containers {
		patchOps, err := p.getPatchOps(containerIdx, container)
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

	glog.V(2).Info(fmt.Sprintf("handling request: %s", string(body)))
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

func makePodsHandler(p *patcher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, func(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
			return mutatePods(ar, p)
		})
	}
}

func fatal(err error) {
	fmt.Printf("ERROR: %+v\n", err)
	os.Exit(1)
}

func main() {
	var kubeconfig string
	var master string
	var certFile string
	var keyFile string
	var mode string
	var config *rest.Config
	var err error

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
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

	if _, err = os.Stat(certFile); err != nil {
		glog.Error("TLS certificate not found")
		os.Exit(1)
	}

	if _, err = os.Stat(keyFile); err != nil {
		glog.Error("TLS private key not found")
		os.Exit(1)
	}

	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	if err != nil {
		glog.Error("Failed to get cluster config ", err)
		os.Exit(1)
	}

	patcher, err := newPatcher(mode)
	if err != nil {
		fatal(err)
	}

	controller, err := newController(patcher, config)
	if err != nil {
		fatal(err)
	}
	go controller.run(controllerThreadNum)

	http.HandleFunc("/pods", makePodsHandler(patcher))

	glog.V(2).Info("Webhook started")

	server := &http.Server{
		Addr:      ":443",
		TLSConfig: getTLSConfig(certFile, keyFile),
	}

	fatal(server.ListenAndServeTLS("", ""))
}
