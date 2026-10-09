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
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationWebhook = "deployments/sgx_admissionwebhook/overlays/default-with-certmanager/kustomization.yaml"
	kustomizationPlugin  = "deployments/sgx_plugin/base/kustomization.yaml"
)

func init() {
	ginkgo.Describe("SGX plugin", ginkgo.Label("sgx"), describe)
}

func describe() {
	f := framework.NewDefaultFramework("sgxplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	deploymentWebhookPath := utils.MustLocateRepoFile(kustomizationWebhook)

	deploymentPluginPath := utils.MustLocateRepoFile(kustomizationPlugin)

	var pluginPodName string

	ginkgo.BeforeEach(func(ctx context.Context) {
		_ = utils.DeployWebhook(ctx, f, deploymentWebhookPath)

		ginkgo.By("deploying SGX plugin")
		pluginPods := utils.ApplyPluginAndWait(ctx, f, deploymentPluginPath, "intel-sgx-plugin", 100*time.Second)
		pluginPodName = pluginPods[0].Name

		ginkgo.By("checking SGX plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(pluginPods); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.Context("When SGX resources are available", func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			ginkgo.By("checking if the resource is allocatable")
			gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, "sgx.intel.com/epc")).
				WithTimeout(150 * time.Second).Should(gomega.BeNumerically(">", 0))
			gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, "sgx.intel.com/enclave")).
				WithTimeout(30 * time.Second).Should(gomega.BeNumerically(">", 0))
			gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, "sgx.intel.com/provision")).
				WithTimeout(30 * time.Second).Should(gomega.BeNumerically(">", 0))
		})

		ginkgo.It("deploys a sgx-sdk-demo pod requesting SGX enclave resources", ginkgo.Label("sgx-sdk-demo"), func(ctx context.Context) {
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
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, pod.ObjectMeta.Name, "testcontainer", 60*time.Second)
		})
	})

	ginkgo.AfterEach(func(ctx context.Context) {
		ginkgo.By("undeploying SGX plugin")
		utils.DeletePluginAndWait(ctx, f, deploymentPluginPath, pluginPodName, 30*time.Second)
	})
}
