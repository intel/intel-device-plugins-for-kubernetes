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

package qat

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ejob "k8s.io/kubernetes/test/e2e/framework/job"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	qatPluginKustomizationYaml = "deployments/qat_plugin/overlays/e2e/kustomization.yaml"
	cryptoTestYaml             = "deployments/qat_dpdk_app/crypto-perf/crypto-perf-dpdk-pod-requesting-qat-cy.yaml"
	compressTestYaml           = "deployments/qat_dpdk_app/compress-perf/compress-perf-dpdk-pod-requesting-qat-dc.yaml"
	cyResource                 = "qat.intel.com/cy"
	dcResource                 = "qat.intel.com/dc"
)

const (
	// The numbers for test below are from the document "Intel QuckAssist Technology Software for Linux*".
	// It is possible to add them for multiple test runs.
	symmetric = 1 << iota
	rsa
	dsa
	ecdsa
	dh
	compression
)

func init() {
	ginkgo.Describe("QAT plugin in DPDK mode", ginkgo.Label("qat"), ginkgo.Label("dpdk"), describeQatDpdkPlugin)
}

func describeQatDpdkPlugin() {
	f := framework.NewDefaultFramework("qatplugindpdk")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath := utils.MustLocateRepoFile(qatPluginKustomizationYaml)

	cryptoTestYamlPath := utils.MustLocateRepoFile(cryptoTestYaml)

	compressTestYamlPath := utils.MustLocateRepoFile(compressTestYaml)

	var dpPodName string

	var resourceName v1.ResourceName

	ginkgo.JustBeforeEach(func(ctx context.Context) {
		ginkgo.By("deploying QAT plugin in DPDK mode")
		pluginPods := utils.ApplyPluginAndWait(ctx, f, kustomizationPath, "intel-qat-plugin", 100*time.Second)
		dpPodName = pluginPods[0].Name

		ginkgo.By("checking QAT plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(pluginPods); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking if the resource is allocatable")
		gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, resourceName)).
			WithTimeout(30 * time.Second).Should(gomega.BeNumerically(">", 0))
	})

	ginkgo.AfterEach(func(ctx context.Context) {
		ginkgo.By("undeploying QAT plugin")
		utils.DeletePluginAndWait(ctx, f, kustomizationPath, dpPodName, 30*time.Second)
	})

	ginkgo.Context("When QAT resources are continuously available with crypto (cy) services enabled", ginkgo.Label("cy"), func() {
		// This BeforeEach runs even before the JustBeforeEach above.
		ginkgo.BeforeEach(func() {
			ginkgo.By("creating a configMap before plugin gets deployed")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=ServicesEnabled=sym;asym", "qat-config")

			ginkgo.By("setting resourceName for cy services")
			resourceName = cyResource
		})

		ginkgo.It("deploys a crypto pod (openssl) requesting QAT resources", ginkgo.Label("openssl"), func(ctx context.Context) {
			command := []string{
				"cpa_sample_code",
				"runTests=" + strconv.Itoa(symmetric),
				"signOfLife=1",
			}
			pod := createPod(ctx, f, "cpa-sample-code", resourceName, "intel/openssl-qat-engine:devel", command)

			ginkgo.By("waiting the cpa-sample-code pod for the resource " + resourceName.String() + " to finish successfully")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name, 300*time.Second)
		})

		ginkgo.It("deploys a crypto pod (dpdk crypto-perf) requesting QAT resources", ginkgo.Label("crypto-perf"), func(ctx context.Context) {
			ginkgo.By("submitting a crypto pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(cryptoTestYamlPath))

			ginkgo.By("waiting the crypto pod to finish successfully")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, "qat-dpdk-test-crypto-perf", "crypto-perf", 300*time.Second)
		})

		ginkgo.It("deploys a crypto pod (qat-engine testapp)", ginkgo.Label("qat-engine"), func(ctx context.Context) {
			command := []string{
				"testapp",
				"-provider", "qatprovider",
				"-async_jobs", "1",
				"-c", "1",
				"-n", "1",
				"-nc", "1",
				"-v",
				"-hw_algo", "0x0029",
			}
			pod := createPod(ctx, f, "qat-engine-testapp", resourceName, "intel/openssl-qat-engine:devel", command)

			ginkgo.By("waiting the qat-engine-testapp pod for the resource " + resourceName.String() + " to finish successfully")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name, 300*time.Second)
		})
	})

	ginkgo.Context("When QAT resources are continuously available with compress (dc) services enabled", ginkgo.Label("dc"), func() {
		ginkgo.BeforeEach(func() {
			ginkgo.By("creating a configMap before plugin gets deployed")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=ServicesEnabled=dc", "qat-config")

			ginkgo.By("setting resourceName for dc services")
			resourceName = dcResource
		})

		ginkgo.It("deploys a compress pod (openssl) requesting QAT resources", ginkgo.Label("openssl"), func(ctx context.Context) {
			command := []string{
				"cpa_sample_code",
				"runTests=" + strconv.Itoa(compression),
				"signOfLife=1",
			}
			pod := createPod(ctx, f, "cpa-sample-code", resourceName, "intel/openssl-qat-engine:devel", command)

			ginkgo.By("waiting the cpa-sample-code pod for the resource " + resourceName.String() + " to finish successfully")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name, 300*time.Second)
		})

		ginkgo.It("deploys a compress pod (dpdk compress-perf) requesting QAT resources", ginkgo.Label("compress-perf"), func(ctx context.Context) {
			ginkgo.By("submitting a compress pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(compressTestYamlPath))

			ginkgo.By("waiting the compress pod to finish successfully")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, "qat-dpdk-test-compress-perf", "compress-perf", 300*time.Second)
		})
	})

	ginkgo.Context("When a QAT device goes unresponsive", ginkgo.Label("nft"), func() {
		ginkgo.When("QAT's auto-reset is off", func() {
			ginkgo.BeforeEach(func() {
				ginkgo.By("creating a configMap before plugin gets deployed")
				e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=$'ServiceEnabled=dc\nAutoresetEnabled=off", "qat-config")

				ginkgo.By("setting resourceName for dc services")
				resourceName = dcResource
			})

			ginkgo.It("checks if unhealthy status is reported", ginkgo.Label("heartbeat"), func(ctx context.Context) {
				injectError(ctx, f, resourceName)

				ginkgo.By("waiting node resources become zero")
				gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, resourceName)).
					WithTimeout(30 * time.Second).Should(gomega.BeZero())
			})
		})

		ginkgo.When("QAT's autoreset is on", func() {
			ginkgo.BeforeEach(func() {
				ginkgo.By("creating a configMap before plugin gets deployed")
				e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=$'ServiceEnabled=dc\nAutoresetEnabled=on", "qat-config")

				ginkgo.By("setting resourceName for dc services")
				resourceName = dcResource
			})

			ginkgo.It("checks if an injected error gets solved", ginkgo.Label("autoreset"), func(ctx context.Context) {
				injectError(ctx, f, resourceName)

				ginkgo.By("seeing if there is zero resource")
				gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, resourceName)).
					WithTimeout(30 * time.Second).Should(gomega.BeZero())

				ginkgo.By("seeing if there is positive allocatable resource")
				gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, resourceName)).
					WithTimeout(300 * time.Second).Should(gomega.BeNumerically(">", 0))

				ginkgo.By("checking if openssl pod runs successfully")
				command := []string{
					"cpa_sample_code",
					"runTests=" + strconv.Itoa(compression),
					"signOfLife=1",
				}
				pod := createPod(ctx, f, "cpa-sample-code", resourceName, "intel/openssl-qat-engine:devel", command)

				ginkgo.By("waiting the cpa-sample-code pod for the resource " + resourceName.String() + " to finish successfully")
				utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name, 300*time.Second)
			})
		})
	})
}

func createPod(ctx context.Context, f *framework.Framework, name string, resourceName v1.ResourceName, image string, command []string) *v1.Pod {
	// qatlib >= 26.02 backs its DMA buffers with 2Mi hugepages whenever the
	// node has them and keeps a temporary file per allocation under
	// /dev/hugepages/qat. On a host this directory is created by qat.service;
	// here it is provided as a HugePages emptyDir mounted at that path.
	resources := v1.ResourceList{
		resourceName:                       resource.MustParse("1"),
		v1.ResourceMemory:                  resource.MustParse("128Mi"),
		v1.ResourceHugePagesPrefix + "2Mi": resource.MustParse("128Mi"),
	}

	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            name,
					Image:           image,
					ImagePullPolicy: "IfNotPresent",
					Command:         command,
					SecurityContext: &v1.SecurityContext{
						Capabilities: &v1.Capabilities{
							Add: []v1.Capability{"IPC_LOCK"}},
					},
					Resources: v1.ResourceRequirements{
						Requests: resources,
						Limits:   resources,
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "hugepage", MountPath: "/dev/hugepages/qat"},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "hugepage",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{Medium: v1.StorageMediumHugePages},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podSpec, metav1.CreateOptions{})
	framework.ExpectNoError(err, "pod Create API error")

	return pod
}

func injectError(ctx context.Context, f *framework.Framework, resourceName v1.ResourceName) {
	nodeName, _ := utils.FindNodeAndResourceCapacity(f, ctx, resourceName.String())
	if nodeName == "" {
		framework.Failf("failed to find a node that has the resource: %s", resourceName)
	}
	yes := true

	job := e2ejob.NewTestJobOnNode("success", "qat-inject-error", v1.RestartPolicyNever, 1, 1, nil, 0, nodeName)
	job.Spec.Template.Spec.Containers[0].Command = []string{
		"/bin/sh",
		"-c",
		"find /sys/kernel/debug/qat_*/heartbeat/ -name inject_error -exec sh -c 'echo 1 > {}' \\;",
	}
	job.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{
		Name:      "debugfs",
		MountPath: "/sys/kernel/debug/",
	}}
	job.Spec.Template.Spec.Volumes = []v1.Volume{{
		Name: "debugfs",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/sys/kernel/debug/",
			},
		},
	}}
	job.Spec.Template.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
		Privileged: &yes,
	}

	job, err := e2ejob.CreateJob(ctx, f.ClientSet, f.Namespace.Name, job)
	framework.ExpectNoError(err, "failed to create job in namespace: %s", f.Namespace.Name)

	err = e2ejob.WaitForJobComplete(ctx, f.ClientSet, f.Namespace.Name, job.Name, batchv1.JobReasonCompletionsReached, 1)
	framework.ExpectNoError(err, "failed to ensure job completion in namespace: %s", f.Namespace.Name)
}
