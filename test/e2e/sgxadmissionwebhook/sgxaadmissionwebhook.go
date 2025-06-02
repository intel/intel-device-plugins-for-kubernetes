// Copyright 2021-2022 Intel Corporation. All Rights Reserved.
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

// Package sgxadmissionwebhook implements E2E tests for SGX admission webhook.
package sgxadmissionwebhook

import (
	"context"
	"reflect"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/sgx_admissionwebhook/overlays/default-with-certmanager/kustomization.yaml"
)

func init() {
	ginkgo.Describe("SGX Admission Webhook", describe)
}

func describe() {
	f := framework.NewDefaultFramework("sgxwebhook")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var webhook v1.Pod

	ginkgo.BeforeEach(func(ctx context.Context) {
		kustomizationPath, err := utils.LocateRepoFile(kustomizationYaml)
		if err != nil {
			framework.Failf("unable to locate %q: %v", kustomizationYaml, err)
		}
		webhook = utils.DeployWebhook(ctx, f, kustomizationPath)
	})

	ginkgo.It("checks the webhook pod is safely configured", func(ctx context.Context) {
		err := utils.TestContainersRunAsNonRoot([]v1.Pod{webhook})
		gomega.Expect(err).To(gomega.BeNil())
		err = utils.TestWebhookServerTLS(ctx, f, "https://intelsgxwebhook-webhook-service")
		gomega.Expect(err).To(gomega.BeNil())
	})
	ginkgo.It("mutates created pods when no quote generation is needed", func(ctx context.Context) {
		ginkgo.By("submitting the pod")
		pod := submitPod(ctx, f, []string{"test"}, "")

		ginkgo.By("checking the container resources have been mutated")
		checkMutatedResources(ctx, f, pod.Spec.Containers[0].Resources, []v1.ResourceName{"sgx.intel.com/enclave"}, []v1.ResourceName{"sgx.intel.com/provision"})

		ginkgo.By("checking the pod total EPC size annotation is correctly set")
		gomega.Expect(pod.Annotations["sgx.intel.com/epc"]).To(gomega.Equal("1Mi"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test"]).To(gomega.Equal("1048576"))
	})
	ginkgo.It("mutates created pods when the container contains the quote generation libraries", func(ctx context.Context) {
		ginkgo.By("submitting the pod")
		pod := submitPod(ctx, f, []string{"test"}, "test")

		ginkgo.By("checking the container resources have been mutated")
		checkMutatedResources(ctx, f, pod.Spec.Containers[0].Resources, []v1.ResourceName{"sgx.intel.com/enclave", "sgx.intel.com/provision"}, []v1.ResourceName{})

		ginkgo.By("checking the pod total EPC size annotation is correctly set")
		gomega.Expect(pod.Annotations["sgx.intel.com/epc"]).To(gomega.Equal("1Mi"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test"]).To(gomega.Equal("1048576"))
	})
	ginkgo.It("mutates created pods when the container uses aesmd from a side-car container to generate quotes", func(ctx context.Context) {
		ginkgo.By("submitting the pod")
		pod := submitPod(ctx, f, []string{"test", "aesmd"}, "aesmd")
		ginkgo.By("checking the container resources have been mutated")
		checkMutatedResources(ctx, f, pod.Spec.Containers[0].Resources, []v1.ResourceName{"sgx.intel.com/enclave"}, []v1.ResourceName{"sgx.intel.com/provision"})
		checkMutatedResources(ctx, f, pod.Spec.Containers[1].Resources, []v1.ResourceName{"sgx.intel.com/enclave", "sgx.intel.com/provision"}, []v1.ResourceName{})
		ginkgo.By("checking the container volumes have been mutated")
		checkMutatedVolumes(f, pod, "aesmd-socket", v1.EmptyDirVolumeSource{})
		ginkgo.By("checking the container envvars have been mutated")
		gomega.Expect(pod.Spec.Containers[0].Env[0].Name).To(gomega.Equal("SGX_AESM_ADDR"))
		gomega.Expect(pod.Spec.Containers[0].Env[0].Value).To(gomega.Equal("1"))
		ginkgo.By("checking the pod total EPC size annotation is correctly set")
		gomega.Expect(pod.Annotations["sgx.intel.com/epc"]).To(gomega.Equal("2Mi"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test"]).To(gomega.Equal("1048576"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.aesmd"]).To(gomega.Equal("1048576"))
	})
	ginkgo.It("mutates created pods where one container uses host/daemonset aesmd to generate quotes", func(ctx context.Context) {
		ginkgo.By("submitting the pod")
		pod := submitPod(ctx, f, []string{"test"}, "aesmd")
		ginkgo.By("checking the container resources have been mutated")
		checkMutatedResources(ctx, f, pod.Spec.Containers[0].Resources, []v1.ResourceName{"sgx.intel.com/enclave"}, []v1.ResourceName{"sgx.intel.com/provision"})
		ginkgo.By("checking the container volumes have been mutated")
		checkMutatedVolumes(f, pod, "aesmd-socket", v1.HostPathVolumeSource{})
		ginkgo.By("checking the container envvars have been mutated")
		gomega.Expect(pod.Spec.Containers[0].Env[0].Name).To(gomega.Equal("SGX_AESM_ADDR"))
		gomega.Expect(pod.Spec.Containers[0].Env[0].Value).To(gomega.Equal("1"))
		ginkgo.By("checking the pod total EPC size annotation is correctly set")
		gomega.Expect(pod.Annotations["sgx.intel.com/epc"]).To(gomega.Equal("1Mi"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test"]).To(gomega.Equal("1048576"))
	})
	ginkgo.It("mutates created pods where three containers use host/daemonset aesmd to generate quotes", func(ctx context.Context) {
		ginkgo.By("submitting the pod")
		pod := submitPod(ctx, f, []string{"test1", "test2", "test3"}, "aesmd")
		ginkgo.By("checking the container resources have been mutated")
		checkMutatedResources(ctx, f, pod.Spec.Containers[0].Resources, []v1.ResourceName{"sgx.intel.com/enclave"}, []v1.ResourceName{"sgx.intel.com/provision"})
		checkMutatedResources(ctx, f, pod.Spec.Containers[1].Resources, []v1.ResourceName{"sgx.intel.com/enclave"}, []v1.ResourceName{"sgx.intel.com/provision"})
		checkMutatedResources(ctx, f, pod.Spec.Containers[2].Resources, []v1.ResourceName{"sgx.intel.com/enclave"}, []v1.ResourceName{"sgx.intel.com/provision"})
		ginkgo.By("checking the container volumes have been mutated")
		checkMutatedVolumes(f, pod, "aesmd-socket", v1.HostPathVolumeSource{})
		ginkgo.By("checking the container envvars have been mutated")
		gomega.Expect(pod.Spec.Containers[0].Env[0].Name).To(gomega.Equal("SGX_AESM_ADDR"))
		gomega.Expect(pod.Spec.Containers[0].Env[0].Value).To(gomega.Equal("1"))
		gomega.Expect(pod.Spec.Containers[1].Env[0].Name).To(gomega.Equal("SGX_AESM_ADDR"))
		gomega.Expect(pod.Spec.Containers[1].Env[0].Value).To(gomega.Equal("1"))
		gomega.Expect(pod.Spec.Containers[2].Env[0].Name).To(gomega.Equal("SGX_AESM_ADDR"))
		gomega.Expect(pod.Spec.Containers[2].Env[0].Value).To(gomega.Equal("1"))
		ginkgo.By("checking the pod total EPC size annotation is correctly set")
		gomega.Expect(pod.Annotations["sgx.intel.com/epc"]).To(gomega.Equal("3Mi"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test1"]).To(gomega.Equal("1048576"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test2"]).To(gomega.Equal("1048576"))
		gomega.Expect(pod.Annotations["epc-limit.nri.io/container.test3"]).To(gomega.Equal("1048576"))
	})
	ginkgo.It("checks that Volumes and VolumeMounts are created only once", func(ctx context.Context) {
		ginkgo.By("submitting the pod")
		podSpec := createPodSpec([]string{"test"}, "aesmd")
		podSpec.Spec.Volumes = make([]v1.Volume, 0)
		podSpec.Spec.Volumes = append(podSpec.Spec.Volumes, v1.Volume{
			Name: "aesmd-socket",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		})
		podSpec.Spec.Containers[0].VolumeMounts = make([]v1.VolumeMount, 0)
		podSpec.Spec.Containers[0].VolumeMounts = append(podSpec.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      "aesmd-socket",
			MountPath: "/var/run/aesmd",
		})
		pod := submitCustomPod(ctx, f, podSpec)
		ginkgo.By("checking the container volumes have been not mutated")
		checkMutatedVolumes(f, pod, "aesmd-socket", v1.EmptyDirVolumeSource{})
	})
}

func checkMutatedVolumes(f *framework.Framework, pod *v1.Pod, volumeName string, volumeType interface{}) {
	gomega.Expect(len(pod.Spec.Volumes)).To(gomega.Equal(1))

	switch reflect.TypeOf(volumeType).String() {
	case "v1.HostPathVolumeSource":
		gomega.Expect(pod.Spec.Volumes[0].HostPath).NotTo(gomega.BeNil())
		gomega.Expect(pod.Spec.Volumes[0].Name).To(gomega.Equal(volumeName))
	case "v1.EmptyDirVolumeSource":
		gomega.Expect(pod.Spec.Volumes[0].EmptyDir).NotTo(gomega.BeNil())
		gomega.Expect(pod.Spec.Volumes[0].Name).To(gomega.Equal(volumeName))
	}

	for _, c := range pod.Spec.Containers {
		gomega.Expect(len(c.VolumeMounts)).To(gomega.Equal(1))
		gomega.Expect(c.VolumeMounts[0].Name).To(gomega.Equal(volumeName))
	}
}

func checkMutatedResources(ctx context.Context, f *framework.Framework, r v1.ResourceRequirements, expectedResources, forbiddenResources []v1.ResourceName) {
	for _, res := range expectedResources {
		q, ok := r.Limits[res]
		if !ok {
			e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Fail("the pod has missing resources")
		}

		gomega.Expect(q.String()).To(gomega.Equal("1"))
	}

	for _, res := range forbiddenResources {
		_, ok := r.Limits[res]
		if ok {
			e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Fail("the pod has extra resources")
		}
	}
}

func submitCustomPod(ctx context.Context, f *framework.Framework, podSpec *v1.Pod) *v1.Pod {
	pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx,
		podSpec, metav1.CreateOptions{})

	framework.ExpectNoError(err, "pod Create API error")

	return pod
}

func createPodSpec(containerNames []string, quoteProvider string) *v1.Pod {
	containers := make([]v1.Container, 0)

	for _, c := range containerNames {
		containers = append(containers, v1.Container{
			Name:  c,
			Image: imageutils.GetPauseImageName(),
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("1Mi")},
				Limits:   v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("1Mi")},
			},
		})
	}

	disabled := false

	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook-tester-pod",
			Annotations: map[string]string{
				"sgx.intel.com/quote-provider": quoteProvider,
			},
		},
		Spec: v1.PodSpec{
			AutomountServiceAccountToken: &disabled,
			Containers:                   containers,
		},
	}

	return podSpec
}

func submitPod(ctx context.Context, f *framework.Framework, containerNames []string, quoteProvider string) *v1.Pod {
	return submitCustomPod(ctx, f, createPodSpec(containerNames, quoteProvider))
}
