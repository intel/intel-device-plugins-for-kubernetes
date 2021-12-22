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
	"path/filepath"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	dpdkKustomizationYaml = "deployments/qat_plugin/overlays/e2e/kustomization.yaml"
	compressTestYaml      = "deployments/qat_dpdk_app/test-compress1/kustomization.yaml"
	cryptoTestYaml        = "deployments/qat_dpdk_app/test-crypto1/kustomization.yaml"
)

func init() {
	ginkgo.Describe("QAT plugin in DPDK mode", describeQatDpdkPlugin)
}

func describeQatDpdkPlugin() {
	f := framework.NewDefaultFramework("qatplugindpdk")

	kustomizationPath, err := utils.LocateRepoFile(dpdkKustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", dpdkKustomizationYaml, err)
	}

	compressTestYamlPath, err := utils.LocateRepoFile(compressTestYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", compressTestYaml, err)
	}

	cryptoTestYamlPath, err := utils.LocateRepoFile(cryptoTestYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", cryptoTestYaml, err)
	}

	ginkgo.It("measures performance of DPDK", func() {
		ginkgo.By("deploying QAT plugin in DPDK mode")
		framework.RunKubectlOrDie(f.Namespace.Name, "--namespace", f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for QAT plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-qat-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking QAT plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking the resource is allocatable")
		if err := utils.WaitForNodesWithResource(f.ClientSet, "qat.intel.com/generic", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		ginkgo.By("submitting a crypto pod requesting QAT resources")
		framework.RunKubectlOrDie(f.Namespace.Name, "--namespace", f.Namespace.Name, "apply", "-k", filepath.Dir(cryptoTestYamlPath))

		ginkgo.By("waiting the crypto pod to finnish successfully")
		f.PodClient().WaitForSuccess("qat-dpdk-test-crypto-perf-tc1", 60*time.Second)

		ginkgo.By("submitting a compress pod requesting QAT resources")
		framework.RunKubectlOrDie(f.Namespace.Name, "--namespace", f.Namespace.Name, "apply", "-k", filepath.Dir(compressTestYamlPath))

		ginkgo.By("waiting the compress pod to finnish successfully")
		f.PodClient().WaitForSuccess("qat-dpdk-test-compress-perf-tc1", 60*time.Second)
	})
}
