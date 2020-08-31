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
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	_ = corev1.AddToScheme(scheme)
}

// PatcherManager keeps track of patchers registered for different Kubernetes namespaces.
type PatcherManager struct {
	log      logr.Logger
	patchers map[string]*patcher
}

// NewPatcherManager creates a new PatcherManager.
func NewPatcherManager(log logr.Logger) *PatcherManager {
	return &PatcherManager{
		log:      log,
		patchers: make(map[string]*patcher),
	}
}

// GetPatcher returns a patcher specific to given namespace.
func (pm *PatcherManager) GetPatcher(namespace string) *patcher {
	if p, ok := pm.patchers[namespace]; ok {
		return p
	}

	p := newPatcher(pm.log.WithValues("namespace", namespace))
	pm.patchers[namespace] = p
	pm.log.V(1).Info("created new patcher", "namespace", namespace)

	return p
}

// GetPodMutator returns a handler function replacing FPGA resource names with
// real FPGA resources in pods.
func (pm *PatcherManager) GetPodMutator() func(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
	return pm.mutate
}

// +kubebuilder:webhook:verbs=create;update,path=/pods,mutating=true,failurePolicy=Ignore,groups="",resources=pods,versions=v1,name=fpga.mutator.webhooks.intel.com

func (pm *PatcherManager) mutate(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if req.Resource != podResource {
		err := errors.Errorf("unexpected resource type %q", req.Resource)
		pm.log.Error(err, "unable to mutate")
		return toAdmissionResponse(err)
	}

	raw := req.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		pm.log.Error(err, "unable to decode")
		return toAdmissionResponse(err)
	}

	namespace := pod.Namespace
	if namespace == "" && req.Namespace != "" {
		namespace = req.Namespace
	}
	name := pod.Name
	if name == "" && pod.ObjectMeta.GenerateName != "" {
		name = pod.ObjectMeta.GenerateName
	}
	pm.log.V(1).Info("Received pod", "Pod", name, "Namespace", namespace)
	patcher := pm.GetPatcher(namespace)

	reviewResponse := admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}

	ops := []string{}
	for containerIdx, container := range pod.Spec.Containers {
		patchOps, err := patcher.getPatchOps(containerIdx, container)
		if err != nil {
			return toAdmissionResponse(err)
		}
		ops = append(ops, patchOps...)
	}

	if len(ops) > 0 {
		reviewResponse.Patch = []byte("[ " + strings.Join(ops, ",") + " ]")
		pt := admissionv1beta1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
	}
	return webhook.AdmissionResponse{
		AdmissionResponse: reviewResponse,
	}
}

func toAdmissionResponse(err error) webhook.AdmissionResponse {
	return webhook.AdmissionResponse{
		AdmissionResponse: admissionv1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
			Allowed: false,
		},
	}
}
