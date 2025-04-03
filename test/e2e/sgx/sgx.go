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
	"fmt"
	"path/filepath"
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
	e2ejob "k8s.io/kubernetes/test/e2e/framework/job"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	ns                   = "inteldeviceplugins-system"
	timeout              = time.Second * 120
	kustomizationWebhook = "deployments/sgx_admissionwebhook/overlays/default-with-certmanager/kustomization.yaml"
	kustomizationPlugin  = "deployments/sgx_plugin/base/kustomization.yaml"
	// TODO: move to epc-cgroups overlay once available.
	// kustomizationPlugin  = "deployments/sgx_plugin/overlays/epc-cgroups/kustomization.yaml".
	stressNGImage       = "intel/stress-ng-gramine:devel"
	stressNGEnclaveSize = 402653184
)

func init() {
	ginkgo.Describe("SGX plugin [Device:sgx]", describe)
}

func describe() {
	f := framework.NewDefaultFramework("sgxplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	deploymentWebhookPath, errFailedToLocateRepoFile := utils.LocateRepoFile(kustomizationWebhook)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", kustomizationWebhook, errFailedToLocateRepoFile)
	}

	deploymentPluginPath, errFailedToLocateRepoFile := utils.LocateRepoFile(kustomizationPlugin)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", kustomizationPlugin, errFailedToLocateRepoFile)
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		_ = utils.DeployWebhook(ctx, f, deploymentWebhookPath)

		ginkgo.By("deploying SGX plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(deploymentPluginPath))

		ginkgo.By("waiting for SGX plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-sgx-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking SGX plugin's securityContext")
		if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.Context("When SGX resources are available", func() {
		var nodeWithEPC string
		var epcCapacity int64

		ginkgo.BeforeEach(func(ctx context.Context) {
			ginkgo.By("checking if the resource is allocatable")
			if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, "sgx.intel.com/epc", 150*time.Second, utils.WaitForPositiveResource); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable epc resource: %v", err)
			}
			if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, "sgx.intel.com/enclave", 30*time.Second, utils.WaitForPositiveResource); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable enclave resource: %v", err)
			}
			if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, "sgx.intel.com/provision", 30*time.Second, utils.WaitForPositiveResource); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable provision resource: %v", err)
			}

			nodelist, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				framework.Failf("failed to list Nodes: %v", err)
			}

			// we have at least one node with sgx.intel.com/epc capacity
			for _, item := range nodelist.Items {
				if q, ok := item.Status.Allocatable["sgx.intel.com/epc"]; ok && q.Value() > 0 {
					epcCapacity = q.Value()
					nodeWithEPC = item.Name
					break
				}
			}
		})

		ginkgo.It("deploys a sgx-sdk-demo pod requesting SGX enclave resources [App:sgx-sdk-demo]", func(ctx context.Context) {
			podSpec := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "sgxplugin-tester"},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:       "testcontainer",
							Image:      "intel/sgx-sdk-demo:devel",
							WorkingDir: "/opt/intel/sgx-sample-app/",
							Command:    []string{"/opt/intel/sgx-sample-app/sgx-sample-app"},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("42")},
								Limits:   v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("42")},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			}
			pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podSpec, metav1.CreateOptions{})
			framework.ExpectNoError(err, "pod Create API error")

			ginkgo.By("waiting the pod to finish successfully")
			err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.ObjectMeta.Name, f.Namespace.Name, 60*time.Second)
			gomega.Expect(err).To(gomega.BeNil(), utils.GetPodLogs(ctx, f, pod.ObjectMeta.Name, "testcontainer"))
		})

		ginkgo.It("deploys simultaneous SGX EPC stressor jobs with equal EPC limits but no memory limits [App:sgx-epc-cgroup]", func(ctx context.Context) {
			parallelism := int32(epcCapacity/stressNGEnclaveSize) + 1
			completions := int32(epcCapacity/stressNGEnclaveSize) + 1
			quantity := resource.NewQuantity(stressNGEnclaveSize/2, resource.BinarySI)

			testArgs := []string{
				"stress-ng",
				"--verbose",
				"--vm",
				"1",
				"--vm-bytes",
				"20%",
				"--page-in",
				"-t",
				"30",
			}
			job := e2ejob.NewTestJobOnNode("success", fmt.Sprintf("sgx-epc-stressjob-npods-%d", parallelism), v1.RestartPolicyNever, parallelism, completions, nil, 0, nodeWithEPC)

			job.Spec.Template.Spec.Containers[0].Image = stressNGImage
			job.Spec.Template.Spec.Containers[0].Args = testArgs
			job.Spec.Template.Spec.Containers[0].Resources = v1.ResourceRequirements{
				Requests: v1.ResourceList{"sgx.intel.com/epc": *quantity},
				Limits:   v1.ResourceList{"sgx.intel.com/epc": *quantity},
			}

			job, err := e2ejob.CreateJob(ctx, f.ClientSet, f.Namespace.Name, job)
			framework.ExpectNoError(err, "failed to create job in namespace: %s", f.Namespace.Name)

			err = e2ejob.WaitForJobComplete(ctx, f.ClientSet, f.Namespace.Name, job.Name, nil, completions)
			framework.ExpectNoError(err, "failed to ensure job completion in namespace: %s", f.Namespace.Name)
		})

		ginkgo.It("deploys one SGX EPC stressor job with the EPC limit set to enclave size and the memory limit set very low [App:sgx-epc-cgroup]", func(ctx context.Context) {
			quantity := resource.NewQuantity(stressNGEnclaveSize, resource.BinarySI)

			testArgs := []string{
				"stress-ng",
				"--verbose",
				"--vm",
				"1",
				"--vm-bytes",
				"20%",
				"--page-in",
				"-t",
				"30",
			}
			job := e2ejob.NewTestJobOnNode("success", "sgx-epc-stressjob-lowmemlimit", v1.RestartPolicyNever, 1, 1, nil, 0, nodeWithEPC)

			job.Spec.Template.Spec.Containers[0].Image = stressNGImage
			job.Spec.Template.Spec.Containers[0].Args = testArgs
			job.Spec.Template.Spec.Containers[0].Resources = v1.ResourceRequirements{
				Requests: v1.ResourceList{"sgx.intel.com/epc": *quantity},
				Limits: v1.ResourceList{"sgx.intel.com/epc": *quantity,
					v1.ResourceMemory: resource.MustParse("42Mi")},
			}

			job, err := e2ejob.CreateJob(ctx, f.ClientSet, f.Namespace.Name, job)
			framework.ExpectNoError(err, "failed to create job in namespace: %s", f.Namespace.Name)

			err = e2ejob.WaitForJobComplete(ctx, f.ClientSet, f.Namespace.Name, job.Name, nil, 1)
			framework.ExpectNoError(err, "failed to ensure job completion in namespace: %s", f.Namespace.Name)
		})

		ginkgo.It("deploys one SGX EPC stressor job with EDMM that ramps EPC allocations and memory limit set to kill once enough EPC pages are reclaimed [App:sgx-epc-cgroup]", func(ctx context.Context) {
			quantity := resource.NewQuantity(epcCapacity/10, resource.BinarySI)

			testArgs := []string{
				"stress-ng-edmm",
				"--verbose",
				"--bigheap",
				"1",
				"--bigheap-growth",
				"10m",
				"--page-in",
				"-t",
				"300",
			}
			job := e2ejob.NewTestJobOnNode("success", "sgx-epc-stressjob-oom", v1.RestartPolicyNever, 1, 1, nil, 0, nodeWithEPC)

			job.Spec.Template.Spec.Containers[0].Image = stressNGImage
			job.Spec.Template.Spec.Containers[0].Args = testArgs
			job.Spec.Template.Spec.Containers[0].Resources = v1.ResourceRequirements{
				Requests: v1.ResourceList{"sgx.intel.com/epc": *quantity},
				Limits: v1.ResourceList{"sgx.intel.com/epc": *quantity,
					v1.ResourceMemory: *quantity},
			}

			job, err := e2ejob.CreateJob(ctx, f.ClientSet, f.Namespace.Name, job)
			framework.ExpectNoError(err, "failed to create job in namespace: %s", f.Namespace.Name)
			err = e2ejob.WaitForJobFailed(f.ClientSet, f.Namespace.Name, job.Name)
			framework.ExpectNoError(err, "failed to ensure job completion in namespace: %s", f.Namespace.Name)
		})

		ginkgo.When("there is no app to run [App:noapp]", func() {
			ginkgo.It("does nothing", func() {})
		})
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("undeploying SGX plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(deploymentPluginPath))
	})
}
