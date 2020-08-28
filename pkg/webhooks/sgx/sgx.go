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

package sgx

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/internal/containers"
)

// +kubebuilder:webhook:path=/pods-sgx,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create;update,versions=v1,name=sgx.mutator.webhooks.intel.com,sideEffects=None

// SgxMutator annotates Pods.
type SgxMutator struct {
	Client  client.Client
	decoder *admission.Decoder
}

const (
	namespace           = "sgx.intel.com"
	encl                = namespace + "/enclave"
	epc                 = namespace + "/epc"
	provision           = namespace + "/provision"
	provisionAnnotation = namespace + "/needs-provision"
)

func (s *SgxMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	if err := s.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	totalEpc := int64(0)

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	for idx, container := range pod.Spec.Containers {
		requestedResources, err := containers.GetRequestedResources(container, namespace)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if epcSize, ok := requestedResources[epc]; ok {
			totalEpc += epcSize

			attestation, found := pod.Annotations[provisionAnnotation]
			if found && attestation == "yes" {
				pod.Spec.Containers[idx].Resources.Limits[corev1.ResourceName(provision)] = resource.MustParse("1")
				pod.Spec.Containers[idx].Resources.Requests[corev1.ResourceName(provision)] = resource.MustParse("1")
			}
			pod.Spec.Containers[idx].Resources.Limits[corev1.ResourceName(encl)] = resource.MustParse("1")
			pod.Spec.Containers[idx].Resources.Requests[corev1.ResourceName(encl)] = resource.MustParse("1")
		}
	}

	if totalEpc != 0 {
		quantity := resource.NewQuantity(totalEpc, resource.BinarySI)
		pod.Annotations["sgx.intel.com/epc"] = quantity.String()
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// SgxMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.
func (s *SgxMutator) InjectDecoder(d *admission.Decoder) error {
	s.decoder = d
	return nil
}
