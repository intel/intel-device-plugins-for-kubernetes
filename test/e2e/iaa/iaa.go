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

package iaa

import (
	"context"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/iaa_plugin/overlays/iaa_initcontainer/iaa_initcontainer.yaml"
	configmapYaml     = "demo/iaa.conf"
	demoYaml          = "demo/iaa-accel-config-demo-pod.yaml"
	podName           = "iaa-accel-config-demo"
)

func init() {
	ginkgo.Describe("IAA plugin", ginkgo.Label("iaa"), describe)
}

func describe() {
	f := framework.NewDefaultFramework("iaaplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath := utils.MustLocateRepoFile(kustomizationYaml)

	configmap := utils.MustLocateRepoFile(configmapYaml)

	demoPath := utils.MustLocateRepoFile(demoYaml)

	var dpPodName string

	ginkgo.BeforeEach(func(ctx context.Context) {
		ginkgo.By("deploying IAA plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "intel-iaa-config", "--from-file="+configmap)

		pluginPods := utils.ApplyPluginAndWait(ctx, f, kustomizationPath, "intel-iaa-plugin", 300*time.Second)
		dpPodName = pluginPods[0].Name

		ginkgo.By("checking IAA plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(pluginPods); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.AfterEach(func(ctx context.Context) {
		ginkgo.By("undeploying IAA plugin")
		utils.DeletePluginAndWait(ctx, f, kustomizationPath, dpPodName, 30*time.Second)
	})

	ginkgo.Context("When IAA resources are available", ginkgo.Label("dedicated"), func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			ginkgo.By("checking if the resource is allocatable")
			gomega.Eventually(ctx, utils.AllocatableResource(f.ClientSet, "iaa.intel.com/wq-user-dedicated")).
				WithTimeout(300 * time.Second).Should(gomega.BeNumerically(">", 0))
		})

		ginkgo.It("deploys a demo app", ginkgo.Label("accel-config"), func(ctx context.Context) {
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoPath)

			ginkgo.By("waiting for the IAA demo to succeed")
			utils.WaitForPodSuccess(ctx, f.ClientSet, f.Namespace.Name, podName, podName, 360*time.Second)
		})
	})
}
