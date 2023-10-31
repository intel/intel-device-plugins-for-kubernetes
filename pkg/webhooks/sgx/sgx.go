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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/internal/containers"
)

var ErrObjectType = errors.New("invalid runtime object type")

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create,versions=v1,name=sgx.mutator.webhooks.intel.com,sideEffects=None,admissionReviewVersions=v1,reinvocationPolicy=IfNeeded

// Mutator annotates Pods.
type Mutator struct{}

func (s *Mutator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithDefaulter(s).
		Complete()
}

const (
	epcLimitKey              = "epc-limit.nri.io/container"
	namespace                = "sgx.intel.com"
	encl                     = namespace + "/enclave"
	epc                      = namespace + "/epc"
	provision                = namespace + "/provision"
	quoteProvAnnotation      = namespace + "/quote-provider"
	aesmdQuoteProvKey        = "aesmd"
	aesmdSocketDirectoryPath = "/var/run/aesmd"
	aesmdSocketName          = "aesmd-socket"
)

func createAesmdVolumeIfNotExists(needsAesmd bool, epcUserCount int32, aesmdPresent bool, pod *corev1.Pod) *corev1.Volume {
	var vol *corev1.Volume

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
		vol = &corev1.Volume{
			Name: aesmdSocketName,
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
		vol = &corev1.Volume{
			Name: aesmdSocketName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: aesmdSocketDirectoryPath,
					Type: &dirOrCreate,
				},
			},
		}
	}

	// Do not return a new Volume if it already exists in the Pod spec
	if pod.Spec.Volumes != nil {
		for _, existingVolume := range pod.Spec.Volumes {
			if existingVolume.Name == vol.Name {
				return nil
			}
		}
	}

	return vol
}

func volumeMountExists(path string, container *corev1.Container) bool {
	if container.VolumeMounts != nil {
		for _, vm := range container.VolumeMounts {
			if vm.MountPath == path {
				return true
			}
		}
	}

	return false
}

func createNewVolumeMounts(container *corev1.Container, volumeMount *corev1.VolumeMount) []corev1.VolumeMount {
	if container.VolumeMounts == nil {
		return []corev1.VolumeMount{*volumeMount}
	}

	return append(container.VolumeMounts, *volumeMount)
}

func (s *Mutator) Default(ctx context.Context, obj runtime.Object) error {
	var pod *corev1.Pod

	log := logf.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)

	if !ok {
		return fmt.Errorf("%w: expected a Pod but got a %T", ErrObjectType, obj)
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
			return err
		}

		// the container has no sgx.intel.com/epc
		epcSize, ok := requestedResources[epc]
		if !ok {
			continue
		}

		pod.Annotations[fmt.Sprintf("%s.%s", epcLimitKey, container.Name)] = fmt.Sprintf("%d", epcSize)

		totalEpc += epcSize

		// Quote Generation Modes:
		//
		// in-process: A container has its own quote provider library library: In this mode,
		// the container needs a handle to /dev/sgx_provision (sgx.intel.com/provision resource).
		// out-of-process: A container uses Intel aesmd. In this mode, the container must talk to
		// aesmd over /var/run/aesmd/aesm.sock. aesmd can run either as a side-car or a DaemonSet
		//
		// Mode selection: The mode selection is done by setting sgx.intel.com/quote-provider annotation
		// to a value that specifies the container name. If the annotation matches the container requesting
		// SGX EPC resources, the webhook adds both /dev/sgx_provision and /dev/sgx_enclave resource requests.
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
		epcUserCount++

		switch quoteProvider {
		// container mutate logic for Intel aesmd users
		case aesmdQuoteProvKey:
			// Check if we already have a VolumeMount for this path -- let's not add it if it's there.
			// This needs to be an external function because of the linting complexity check. We lose
			// one "if" this way.
			if !volumeMountExists(aesmdSocketDirectoryPath, &pod.Spec.Containers[idx]) {
				vms := createNewVolumeMounts(&pod.Spec.Containers[idx],
					&corev1.VolumeMount{
						Name:      aesmdSocketName,
						MountPath: aesmdSocketDirectoryPath,
					})
				container.VolumeMounts = vms
			}

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
		}

		pod.Spec.Containers[idx] = container
	}

	if vol := createAesmdVolumeIfNotExists(quoteProvider == aesmdQuoteProvKey, epcUserCount, aesmdPresent, pod); vol != nil {
		if pod.Spec.Volumes == nil {
			pod.Spec.Volumes = make([]corev1.Volume, 0)
		}

		pod.Spec.Volumes = append(pod.Spec.Volumes, *vol)
	}

	if totalEpc != 0 {
		quantity := resource.NewQuantity(totalEpc, resource.BinarySI)
		pod.Annotations["sgx.intel.com/epc"] = quantity.String()
	}

	log.Info("Mutated SGX Pod")

	return nil
}
