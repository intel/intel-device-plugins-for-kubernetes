// Copyright 2021-2026 Intel Corporation. All Rights Reserved.
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

package dsa

import (
	"context"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/dsa_plugin/overlays/dsa_initcontainer/dsa_initcontainer.yaml"
	kustomVfioYaml    = "deployments/dsa_plugin/overlays/dsa_vfio_initcontainer/dsa_initcontainer.yaml"
	configmapYaml     = "demo/dsa.conf"
	demoYaml          = "demo/dsa-accel-config-demo-pod.yaml"
	dpdkDemoYaml      = "demo/dsa-dpdk-dmadevtest.yaml"
	dpdkVfioYaml      = "demo/dsa-dpdk-dmadevtest-vfio.yaml"
	podName           = "dsa-accel-config-demo"
	dpdkPodName       = "dpdk"
	dpdkVfioPodName   = "dpdk-vfio"
)

func init() {
	ginkgo.Describe("DSA plugin [Device:dsa]", describe)
}

func describe() {
	f := framework.NewDefaultFramework("dsaplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var dpPodName string
	var kustomizationPath string
	var configMapPath string
	var expectedResource corev1.ResourceName

	demoPath := utils.MustLocateRepoFile(demoYaml)

	demoDpdkPath := utils.MustLocateRepoFile(dpdkDemoYaml)

	demoDpdkVfioPath := utils.MustLocateRepoFile(dpdkVfioYaml)

	ginkgo.Context("When DSA resources are available", ginkgo.Label("dsa"), ginkgo.Label("idxd"), func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			kustomizationPath = utils.MustLocateRepoFile(kustomizationYaml)

			configMapPath = utils.MustLocateRepoFile(configmapYaml)

			expectedResource = "dsa.intel.com/wq-user-dedicated"

			ginkgo.By("deploying DSA plugin")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "intel-dsa-config", "--from-file="+configMapPath)

			pluginPods := utils.ApplyPluginAndWait(ctx, f, kustomizationPath, "intel-dsa-plugin", 300*time.Second)
			dpPodName = pluginPods[0].Name

			ginkgo.By("checking DSA plugin's securityContext")
			if err := utils.TestPodsFileSystemInfo(pluginPods); err != nil {
				framework.Failf("container filesystem info checks failed: %v", err)
			}

			ginkgo.By("checking if the resource is allocatable")
			gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, expectedResource)).
				WithTimeout(300 * time.Second).Should(gomega.BeNumerically(">", 0))
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			ginkgo.By("undeploying DSA plugin and its ConfigMap")
			utils.DeletePluginAndWait(ctx, f, kustomizationPath, dpPodName, 30*time.Second)
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "configmap", "intel-dsa-config")
		})
		ginkgo.It("deploys a demo app (accel-config)", ginkgo.Label("accel-config"), func(ctx context.Context) {
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoPath)

			ginkgo.By("waiting for the DSA demo to succeed")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, podName, podName, 200*time.Second)
		})

		ginkgo.It("deploys a demo app (dpdk-test)", ginkgo.Label("dpdk-test"), func(ctx context.Context) {
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoDpdkPath)

			ginkgo.By("waiting for the DSA DPDK demo to succeed")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, dpdkPodName, dpdkPodName, 200*time.Second)
		})
	})

	ginkgo.Context("When DSA VFIO resources are available", ginkgo.Label("dsa"), ginkgo.Label("vfio"), func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			kustomizationPath = utils.MustLocateRepoFile(kustomVfioYaml)

			expectedResource = "dsa.intel.com/vfio"

			ginkgo.By("deploying DSA plugin")
			pluginPods := utils.ApplyPluginAndWait(ctx, f, kustomizationPath, "intel-dsa-plugin", 300*time.Second)
			dpPodName = pluginPods[0].Name

			ginkgo.By("checking DSA plugin's securityContext")
			if err := utils.TestPodsFileSystemInfo(pluginPods); err != nil {
				framework.Failf("container filesystem info checks failed: %v", err)
			}

			ginkgo.By("checking if the resource is allocatable")
			gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, expectedResource)).
				WithTimeout(300 * time.Second).Should(gomega.BeNumerically(">", 0))
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			ginkgo.By("undeploying DSA plugin and its ConfigMap")
			utils.DeletePluginAndWait(ctx, f, kustomizationPath, dpPodName, 30*time.Second)
		})

		ginkgo.It("deploys a demo app", ginkgo.Label("dpdk-vfio-test"), func(ctx context.Context) {
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoDpdkVfioPath)

			ginkgo.By("waiting for the DSA DPDK VFIO demo to succeed")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, dpdkVfioPodName, dpdkVfioPodName, 200*time.Second)
		})
	})
}
