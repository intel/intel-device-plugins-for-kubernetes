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

package gpu

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml   = "deployments/gpu_plugin/kustomization.yaml"
	monitoringYaml      = "deployments/gpu_plugin/overlays/monitoring_shared-dev_nfd/kustomization.yaml"
	healthMgmtYaml      = "deployments/gpu_plugin/overlays/health/kustomization.yaml"
	nfdRulesYaml        = "deployments/nfd/overlays/node-feature-rules/kustomization.yaml"
	containerName       = "testcontainer"
	ptKustomizationYaml = "deployments/gpu_pytorch_demo/kustomization.yaml"
	ptPodName           = "training-pod"
)

func init() {
	// This needs to be Ordered because only one GPU plugin can function on the node at once.
	ginkgo.Describe("GPU plugin [Device:gpu]", describe, ginkgo.Ordered)
}

func createPluginAndVerifyExistence(f *framework.Framework, ctx context.Context, kustomizationPath, baseResource string) {
	ginkgo.By("deploying GPU plugin")
	e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

	ginkgo.By("waiting for GPU plugin's availability")
	podList, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
		labels.Set{"app": "intel-gpu-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
	if err != nil {
		e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
		e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
		framework.Failf("unable to wait for all pods to be running and ready: %v", err)
	}

	ginkgo.By("checking GPU plugin's securityContext")
	if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
		framework.Failf("container filesystem info checks failed: %v", err)
	}

	ginkgo.By("checking if the resource is allocatable")
	if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, v1.ResourceName(baseResource), 30*time.Second, utils.WaitForPositiveResource); err != nil {
		framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
	}
}

func describe() {
	f := framework.NewDefaultFramework("gpuplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	vanillaPath, errFailedToLocateRepoFile := utils.LocateRepoFile(kustomizationYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", kustomizationYaml, errFailedToLocateRepoFile)
	}

	monitoringPath, errFailedToLocateRepoFile := utils.LocateRepoFile(monitoringYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", monitoringYaml, errFailedToLocateRepoFile)
	}

	healthMgmtPath, errFailedToLocateRepoFile := utils.LocateRepoFile(healthMgmtYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", healthMgmtYaml, errFailedToLocateRepoFile)
	}

	ginkgo.Context("When GPU plugin is deployed [Resource:i915]", func() {
		ginkgo.AfterEach(func(ctx context.Context) {
			framework.Logf("Removing gpu-plugin manually")

			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(vanillaPath))

			framework.Logf("Waiting for i915 resources to go to zero")

			// Wait for resources to go to zero
			if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, "gpu.intel.com/i915", 30*time.Second, utils.WaitForZeroResource); err != nil {
				framework.Failf("unable to wait for nodes to have no resources: %v", err)
			}
		})

		ginkgo.It("checks availability of GPU resources [App:busybox]", func(ctx context.Context) {
			createPluginAndVerifyExistence(f, ctx, vanillaPath, "gpu.intel.com/i915")

			podListFunc := framework.ListObjects(f.ClientSet.CoreV1().Pods(f.Namespace.Name).List, metav1.ListOptions{})

			pods, err := podListFunc(ctx)
			if err != nil {
				framework.Failf("Couldn't list pods: %+v", err)
			}

			if len(pods.Items) != 1 {
				framework.Failf("Invalid amount of Pods listed %d", len(pods.Items))
			}

			pluginPod := pods.Items[0]

			ginkgo.By("checking if CDI path is included in volumes")
			found := false
			for _, v := range pluginPod.Spec.Volumes {
				if v.HostPath != nil && v.HostPath.Path == "/var/run/cdi" {
					framework.Logf("CDI volume found")
					found = true

					break
				}
			}

			if !found {
				framework.Fail("Couldn't find CDI volume in GPU plugin deployment")
			}

			ginkgo.By("submitting a pod requesting GPU resources")
			podSpec := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "gpuplugin-tester"},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Args:    []string{"-c", "ls /dev/dri"},
							Name:    containerName,
							Image:   imageutils.GetE2EImage(imageutils.BusyBox),
							Command: []string{"/bin/sh"},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{"gpu.intel.com/i915": resource.MustParse("1")},
								Limits:   v1.ResourceList{"gpu.intel.com/i915": resource.MustParse("1")},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			}
			pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podSpec, metav1.CreateOptions{})
			framework.ExpectNoError(err, "pod Create API error")

			ginkgo.By("waiting the pod to finish successfully")
			e2epod.NewPodClient(f).WaitForSuccess(ctx, pod.ObjectMeta.Name, 60*time.Second)

			ginkgo.By("checking log output")
			log, err := e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, pod.Name, containerName)

			if err != nil {
				framework.Failf("unable to get log from pod: %v", err)
			}

			if !strings.Contains(log, "card") || !strings.Contains(log, "renderD") {
				framework.Logf("log output: %s", log)
				framework.Failf("device mounts not found from log")
			}

			framework.Logf("found card and renderD from the log")
		})

		ginkgo.Context("When [Deployment:monitoring] deployment is applied [Resource:i915]", func() {
			ginkgo.It("check if monitoring resource is available", func(ctx context.Context) {
				createPluginAndVerifyExistence(f, ctx, monitoringPath, "gpu.intel.com/i915")

				ginkgo.By("checking if the monitoring resource is allocatable")
				if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, "gpu.intel.com/i915_monitoring", 30*time.Second, utils.WaitForPositiveResource); err != nil {
					framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
				}
			})
		})

		ginkgo.Context("When [Deployment:healthManagement] deployment is applied [Resource:i915]", func() {
			ginkgo.It("check if i915 resources is available", func(ctx context.Context) {
				createPluginAndVerifyExistence(f, ctx, healthMgmtPath, "gpu.intel.com/i915")
			})
		})

		ginkgo.It("run a small workload on the GPU [App:pytorch]", func(ctx context.Context) {
			createPluginAndVerifyExistence(f, ctx, vanillaPath, "gpu.intel.com/i915")

			kustomYaml, err := utils.LocateRepoFile(ptKustomizationYaml)
			if err != nil {
				framework.Failf("unable to locate %q: %v", ptKustomizationYaml, err)
			}

			ginkgo.By("submitting demo deployment")

			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomYaml))

			ginkgo.By("waiting the pod to finish")

			err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, ptPodName, f.Namespace.Name, 300*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, ptPodName, containerName))

			framework.Logf("tensorflow execution succeeded!")
		})

		ginkgo.When("there is no app to run [App:noapp]", func() {
			ginkgo.It("does nothing", func() {})
		})
	})

	ginkgo.Context("When GPU resources are available [Resource:xe]", func() {
		ginkgo.It("checks availability of GPU resources [App:busybox]", func(ctx context.Context) {
			createPluginAndVerifyExistence(f, ctx, vanillaPath, "gpu.intel.com/xe")

			ginkgo.By("submitting a pod requesting GPU resources")
			podSpec := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "gpuplugin-tester"},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Args:    []string{"-c", "ls /dev/dri"},
							Name:    containerName,
							Image:   imageutils.GetE2EImage(imageutils.BusyBox),
							Command: []string{"/bin/sh"},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{"gpu.intel.com/xe": resource.MustParse("1")},
								Limits:   v1.ResourceList{"gpu.intel.com/xe": resource.MustParse("1")},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			}
			pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podSpec, metav1.CreateOptions{})
			framework.ExpectNoError(err, "pod Create API error")

			ginkgo.By("waiting the pod to finish successfully")
			e2epod.NewPodClient(f).WaitForSuccess(ctx, pod.ObjectMeta.Name, 60*time.Second)

			ginkgo.By("checking log output")
			log, err := e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, pod.Name, containerName)

			if err != nil {
				framework.Failf("unable to get log from pod: %v", err)
			}

			if !strings.Contains(log, "card") || !strings.Contains(log, "renderD") {
				framework.Logf("log output: %s", log)
				framework.Failf("device mounts not found from log")
			}

			framework.Logf("found card and renderD from the log")
		})

		ginkgo.When("there is no app to run [App:noapp]", func() {
			ginkgo.It("does nothing", func() {})
		})
	})
}
