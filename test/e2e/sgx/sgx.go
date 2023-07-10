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
	"path/filepath"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	ns                   = "inteldeviceplugins-system"
	timeout              = time.Second * 120
	kustomizationWebhook = "deployments/sgx_admissionwebhook/overlays/default-with-certmanager/kustomization.yaml"
	kustomizationPlugin  = "deployments/sgx_plugin/base/kustomization.yaml"
)

func init() {
	ginkgo.Describe("SGX plugin", describe)
}

func describe() {
	f := framework.NewDefaultFramework("sgxplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	deploymentWebhookPath, err := utils.LocateRepoFile(kustomizationWebhook)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationWebhook, err)
	}

	deploymentPluginPath, err := utils.LocateRepoFile(kustomizationPlugin)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationPlugin, err)
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		_ = utils.DeployWebhook(ctx, f, deploymentWebhookPath)

		ginkgo.By("deploying SGX plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(deploymentPluginPath))

		ginkgo.By("waiting for SGX plugin's availability")
		podList, errPodNotRunning := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-sgx-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if errPodNotRunning != nil {
			e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", errPodNotRunning)
		}

		ginkgo.By("checking SGX plugin's securityContext")
		if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.Context("When SGX resources are available", func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			ginkgo.By("checking if the resource is allocatable")
			if err = utils.WaitForNodesWithResource(ctx, f.ClientSet, "sgx.intel.com/epc", 150*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable epc resource: %v", err)
			}
			if err = utils.WaitForNodesWithResource(ctx, f.ClientSet, "sgx.intel.com/enclave", 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable enclave resource: %v", err)
			}
			if err = utils.WaitForNodesWithResource(ctx, f.ClientSet, "sgx.intel.com/provision", 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable provision resource: %v", err)
			}
		})

		ginkgo.It("deploys a sgx-sdk-demo pod requesting SGX enclave resources", func(ctx context.Context) {
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

			e2epod.NewPodClient(f).WaitForSuccess(ctx, pod.ObjectMeta.Name, 60*time.Second)
		})
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("undeploying SGX plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(deploymentPluginPath))
	})
}
