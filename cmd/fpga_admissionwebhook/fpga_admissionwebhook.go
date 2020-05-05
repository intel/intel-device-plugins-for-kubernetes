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
	"io/ioutil"
	"net/http"
	"os"
	"strings"

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
	"k8s.io/klog"
)

const (
	controllerThreadNum = 1
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	klog.InitFlags(nil)
	addToScheme(scheme)
}

func addToScheme(scheme *runtime.Scheme) {
	corev1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)
}

func getTLSConfig(certFile string, keyFile string) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		klog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

func mutatePods(ar v1beta1.AdmissionReview, pm patcherManager) *v1beta1.AdmissionResponse {
	var ops []string

	klog.V(4).Info("mutating pods")

	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		klog.Warningf("Unexpected resource type %s", ar.Request.Resource)
		return nil
	}

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		klog.Warningf("%+v", err)
		return toAdmissionResponse(err)
	}

	namespace := pod.Namespace
	if namespace == "" && ar.Request.Namespace != "" {
		namespace = ar.Request.Namespace
	}
	name := pod.Name
	if name == "" && pod.ObjectMeta.GenerateName != "" {
		name = pod.ObjectMeta.GenerateName
	}
	klog.V(4).Infof("Received pod '%s' in name space '%s'", name, namespace)
	patcher := pm.getPatcher(namespace)

	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	for containerIdx, container := range pod.Spec.Containers {
		patchOps, err := patcher.getPatchOps(containerIdx, container)
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
		Allowed: false,
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
		klog.V(4).Info("No body in request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.V(4).Infof("contentType=%s, expect application/json", contentType)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	klog.V(4).Infof("handling request: %s", string(body))
	ar := v1beta1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		klog.Warningf("%+v", err)
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
	klog.V(4).Info("sending response", string(reviewResponse.Patch))

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
		klog.Error(err)
		return
	}
	if _, err := w.Write(resp); err != nil {
		klog.Warningf("%+v", err)
	}
}

func makePodsHandler(pm patcherManager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, func(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
			return mutatePods(ar, pm)
		})
	}
}

func main() {
	var kubeconfig string
	var master string
	var certFile string
	var keyFile string
	var config *rest.Config
	var err error

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.StringVar(&certFile, "tls-cert-file", certFile,
		"File containing the x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	flag.StringVar(&keyFile, "tls-private-key-file", keyFile, "File containing the x509 private key matching --tls-cert-file.")
	flag.Parse()

	if certFile == "" {
		klog.Fatal("TLS certificate file is not set")
	}

	if keyFile == "" {
		klog.Fatal("TLS private key is not set")
	}

	if _, err = os.Stat(certFile); err != nil {
		klog.Fatal("TLS certificate not found")
	}

	if _, err = os.Stat(keyFile); err != nil {
		klog.Fatal("TLS private key not found")
	}

	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	if err != nil {
		klog.Fatal("Failed to get cluster config ", err)
	}

	pm := newPatcherManager()

	controller, err := newController(pm, config)
	if err != nil {
		klog.Fatalf("%+v", err)
	}
	go controller.run(controllerThreadNum)

	http.HandleFunc("/pods", makePodsHandler(pm))

	klog.V(4).Info("Webhook started")

	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: getTLSConfig(certFile, keyFile),
	}

	klog.Fatal(server.ListenAndServeTLS("", ""))
}
