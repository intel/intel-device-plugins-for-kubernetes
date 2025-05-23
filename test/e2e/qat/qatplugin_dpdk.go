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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ejob "k8s.io/kubernetes/test/e2e/framework/job"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
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
	ginkgo.Describe("QAT plugin in DPDK mode [Device:qat] [Mode:dpdk]", describeQatDpdkPlugin)
}

func describeQatDpdkPlugin() {
	f := framework.NewDefaultFramework("qatplugindpdk")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath, errFailedToLocateRepoFile := utils.LocateRepoFile(qatPluginKustomizationYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", qatPluginKustomizationYaml, errFailedToLocateRepoFile)
	}

	cryptoTestYamlPath, errFailedToLocateRepoFile := utils.LocateRepoFile(cryptoTestYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", cryptoTestYaml, errFailedToLocateRepoFile)
	}

	compressTestYamlPath, errFailedToLocateRepoFile := utils.LocateRepoFile(compressTestYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", compressTestYaml, errFailedToLocateRepoFile)
	}

	var dpPodName string

	var resourceName v1.ResourceName

	ginkgo.JustBeforeEach(func(ctx context.Context) {
		ginkgo.By("deploying QAT plugin in DPDK mode")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for QAT plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-qat-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}
		dpPodName = podList.Items[0].Name

		ginkgo.By("checking QAT plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking if the resource is allocatable")
		if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, resourceName, 30*time.Second, utils.WaitForPositiveResource); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}
	})

	ginkgo.AfterEach(func(ctx context.Context) {
		ginkgo.By("undeploying QAT plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(kustomizationPath))
		if err := e2epod.WaitForPodNotFoundInNamespace(ctx, f.ClientSet, dpPodName, f.Namespace.Name, 30*time.Second); err != nil {
			framework.Failf("failed to terminate pod: %v", err)
		}
	})

	ginkgo.Context("When QAT resources are continuously available with crypto (cy) services enabled [Resource:cy]", func() {
		// This BeforeEach runs even before the JustBeforeEach above.
		ginkgo.BeforeEach(func() {
			ginkgo.By("creating a configMap before plugin gets deployed")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=ServicesEnabled=sym;asym", "qat-config")

			ginkgo.By("setting resourceName for cy services")
			resourceName = cyResource
		})

		ginkgo.It("deploys a crypto pod (openssl) requesting QAT resources [App:openssl]", func(ctx context.Context) {
			command := []string{
				"cpa_sample_code",
				"runTests=" + strconv.Itoa(symmetric),
				"signOfLife=1",
			}
			pod := createPod(ctx, f, "cpa-sample-code", resourceName, "intel/openssl-qat-engine:devel", command)

			ginkgo.By("waiting the cpa-sample-code pod for the resource " + resourceName.String() + " to finish successfully")
			err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.ObjectMeta.Name, f.Namespace.Name, 300*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name))
		})

		ginkgo.It("deploys a crypto pod (dpdk crypto-perf) requesting QAT resources [App:crypto-perf]", func(ctx context.Context) {
			ginkgo.By("submitting a crypto pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(cryptoTestYamlPath))

			ginkgo.By("waiting the crypto pod to finish successfully")
			err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, "qat-dpdk-test-crypto-perf", f.Namespace.Name, 300*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, "qat-dpdk-test-crypto-perf", "crypto-perf"))
		})

		ginkgo.It("deploys a crypto pod (qat-engine testapp) [App:qat-engine]", func(ctx context.Context) {
			command := []string{
				"testapp",
				"-engine", "qathwtest",
				"-async_jobs", "1",
				"-c", "1",
				"-n", "1",
				"-nc", "1",
				"-v",
				"-hw_algo", "0x0029",
			}
			pod := createPod(ctx, f, "qat-engine-testapp", resourceName, "intel/openssl-qat-engine:devel", command)

			ginkgo.By("waiting the qat-engine-testapp pod for the resource " + resourceName.String() + " to finish successfully")
			err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.ObjectMeta.Name, f.Namespace.Name, 300*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name))
		})

		ginkgo.When("there is no app to run [App:noapp]", func() {
			ginkgo.It("does nothing", func() {})
		})
	})

	ginkgo.Context("When QAT resources are continuously available with compress (dc) services enabled [Resource:dc]", func() {
		ginkgo.BeforeEach(func() {
			ginkgo.By("creating a configMap before plugin gets deployed")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=ServicesEnabled=dc", "qat-config")

			ginkgo.By("setting resourceName for dc services")
			resourceName = dcResource
		})

		ginkgo.It("deploys a compress pod (openssl) requesting QAT resources [App:openssl]", func(ctx context.Context) {
			command := []string{
				"cpa_sample_code",
				"runTests=" + strconv.Itoa(compression),
				"signOfLife=1",
			}
			pod := createPod(ctx, f, "cpa-sample-code", resourceName, "intel/openssl-qat-engine:devel", command)

			ginkgo.By("waiting the cpa-sample-code pod for the resource " + resourceName.String() + " to finish successfully")
			err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.ObjectMeta.Name, f.Namespace.Name, 300*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name))
		})

		ginkgo.It("deploys a compress pod (dpdk compress-perf) requesting QAT resources [App:compress-perf]", func(ctx context.Context) {
			ginkgo.By("submitting a compress pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(compressTestYamlPath))

			ginkgo.By("waiting the compress pod to finish successfully")
			err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, "qat-dpdk-test-compress-perf", f.Namespace.Name, 300*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, "qat-dpdk-test-compress-perf", "compress-perf"))
		})

		ginkgo.When("there is no app to run [App:noapp]", func() {
			ginkgo.It("does nothing", func() {})
		})
	})

	ginkgo.Context("When a QAT device goes unresponsive", func() {
		ginkgo.When("QAT's auto-reset is off", func() {
			ginkgo.BeforeEach(func() {
				ginkgo.By("creating a configMap before plugin gets deployed")
				e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=$'ServiceEnabled=dc\nAutoresetEnabled=off", "qat-config")

				ginkgo.By("setting resourceName for dc services")
				resourceName = dcResource
			})

			ginkgo.It("checks if unhealthy status is reported [Functionality:heartbeat]", func(ctx context.Context) {
				injectError(ctx, f, resourceName)

				ginkgo.By("waiting node resources become zero")
				if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, resourceName, 30*time.Second, utils.WaitForZeroResource); err != nil {
					framework.Failf("unable to wait for nodes to have no resource: %v", err)
				}
			})
		})

		ginkgo.When("QAT's autoreset is on", func() {
			ginkgo.BeforeEach(func() {
				ginkgo.By("creating a configMap before plugin gets deployed")
				e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "--from-literal", "qat.conf=$'ServiceEnabled=dc\nAutoresetEnabled=on", "qat-config")

				ginkgo.By("setting resourceName for dc services")
				resourceName = dcResource
			})

			ginkgo.It("checks if an injected error gets solved [Functionality:auto-reset]", func(ctx context.Context) {
				injectError(ctx, f, resourceName)

				ginkgo.By("seeing if there is zero resource")
				if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, resourceName, 30*time.Second, utils.WaitForZeroResource); err != nil {
					framework.Failf("unable to wait for nodes to have no resource: %v", err)
				}

				ginkgo.By("seeing if there is positive allocatable resource")
				if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, resourceName, 300*time.Second, utils.WaitForPositiveResource); err != nil {
					framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
				}

				ginkgo.By("checking if openssl pod runs successfully")
				command := []string{
					"cpa_sample_code",
					"runTests=" + strconv.Itoa(compression),
					"signOfLife=1",
				}
				pod := createPod(ctx, f, "cpa-sample-code", resourceName, "intel/openssl-qat-engine:devel", command)

				ginkgo.By("waiting the cpa-sample-code pod for the resource " + resourceName.String() + " to finish successfully")
				err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.ObjectMeta.Name, f.Namespace.Name, 300*time.Second)
				gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name))
			})
		})
	})
}

func createPod(ctx context.Context, f *framework.Framework, name string, resourceName v1.ResourceName, image string, command []string) *v1.Pod {
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
						Requests: v1.ResourceList{resourceName: resource.MustParse("1")},
						Limits:   v1.ResourceList{resourceName: resource.MustParse("1")},
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
