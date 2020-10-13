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
	quoteProvAnnotation = namespace + "/quote-provider"
	aesmdQuoteProvKey   = "aesmd"
)

func getAesmdVolume(needsAesmd bool, epcUserCount int32, aesmdPresent bool) *corev1.Volume {
	switch {
	case epcUserCount == 0:
		// none of the containers in this pod request SGX resourced.
		return nil
	case !needsAesmd:
		// the pod does not specify sgx.intel.com/quote-provider: aesmd
		return nil
	case aesmdPresent && epcUserCount >= 2:
		// aesmd sidecar: the pod has a container named aesmd and >=1 _other_ containers requesting
		// SGX resources. aesmd socket path is provided as an emptydir volume within the pod and
		// mounted by all (SGX) containers.
		return &corev1.Volume{
			Name: "aesmd-socket",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}
	default:
		// aesmd DaemonSet: 'sgx.intel.com/quote-provider: aesmd' is set and no sidecar
		// deployment detected. aesmd socket path is provided as a hostpath volume and mounted
		// by all (SGX) containers.
		dirOrCreate := corev1.HostPathDirectoryOrCreate
		return &corev1.Volume{
			Name: "aesmd-socket",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/aesmd",
					Type: &dirOrCreate,
				},
			},
		}
	}
}

func (s *SgxMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	if err := s.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	totalEpc := int64(0)
	epcUserCount := int32(0)
	aesmdPresent := bool(false)

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	quoteProvider := pod.Annotations[quoteProvAnnotation]

	for idx, container := range pod.Spec.Containers {
		requestedResources, err := containers.GetRequestedResources(container, namespace)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// the container has no sgx.intel.com/epc
		epcSize, ok := requestedResources[epc]
		if !ok {
			continue
		}

		totalEpc += epcSize

		// Quote Generation Modes:
		//
		// in-process: A container has its own quote provider library library: In this mode,
		// the container needs a handle to /dev/sgx/provision (sgx.intel.com/provision resource).
		// out-of-process: A container uses Intel aesmd. In this mode, the container must talk to
		// aesmd over /var/run/aesmd/aesm.sock. aesmd can run either as a side-car or a DaemonSet
		//
		// Mode selection: The mode selection is done by setting sgx.intel.com/quote-provider annotation
		// to a value that specifies the container name. If the annotation matches the container requesting
		// SGX EPC resources, the webhook adds both /dev/sgx/provision and /dev/sgx/enclave resource requests.
		// Without sgx.intel.com/quote-provider annotation set, the container is not able to generate quotes
		// for its enclaves. When pods set sgx.intel.com/quote-provider: "aesmd", Intel aesmd specific volume
		// mounts are added. In both DaemonSet and sidecar deployment scenarios for aesmd, its container name
		// must be set to "aesmd" (TODO: make it configurable?).

		if quoteProvider == container.Name {
			container.Resources.Limits[corev1.ResourceName(provision)] = resource.MustParse("1")
			container.Resources.Requests[corev1.ResourceName(provision)] = resource.MustParse("1")
		}

		container.Resources.Limits[corev1.ResourceName(encl)] = resource.MustParse("1")
		container.Resources.Requests[corev1.ResourceName(encl)] = resource.MustParse("1")

		// we count how many containers within the pod request SGX resources. If the container
		// count is >= 1 and one of them is named aesmdQuoteProvKey, 'aesmd sidecar' deployment
		// assumed.
		epcUserCount += 1

		switch quoteProvider {
		// container mutate logic for Intel aesmd users
		case aesmdQuoteProvKey:
			if container.VolumeMounts == nil {
				container.VolumeMounts = make([]corev1.VolumeMount, 0)
			}

			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{
					Name:      "aesmd-socket",
					MountPath: "/var/run/aesmd",
				})

			if container.Name == aesmdQuoteProvKey {
				aesmdPresent = true
			}

			if container.Env == nil {
				container.Env = make([]corev1.EnvVar, 0)
			}

			// this sets SGX_AESM_ADDR for aesmd itself too but it's harmless
			container.Env = append(container.Env,
				corev1.EnvVar{
					Name:  "SGX_AESM_ADDR",
					Value: "1",
				})
			// container mutate logic for non-aesmd quote providers
			// case:
		}
		pod.Spec.Containers[idx] = container
	}

	if vol := getAesmdVolume(quoteProvider == aesmdQuoteProvKey, epcUserCount, aesmdPresent); vol != nil {
		if pod.Spec.Volumes == nil {
			pod.Spec.Volumes = make([]corev1.Volume, 0)
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, *vol)
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
