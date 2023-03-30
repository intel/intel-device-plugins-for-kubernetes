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
	"github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	qatPluginKustomizationYaml = "deployments/qat_plugin/overlays/e2e/kustomization.yaml"
	opensslTestYaml            = "demo/openssl-qat-engine-cpa-sample-pod.yaml"
	compressTestYaml           = "deployments/qat_dpdk_app/test-compress1/kustomization.yaml"
	cryptoTestYaml             = "deployments/qat_dpdk_app/test-crypto1/kustomization.yaml"
)

func init() {
	ginkgo.Describe("QAT plugin in DPDK mode", describeQatDpdkPlugin)
}

func describeQatDpdkPlugin() {
	f := framework.NewDefaultFramework("qatplugindpdk")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath, err := utils.LocateRepoFile(qatPluginKustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", qatPluginKustomizationYaml, err)
	}

	compressTestYamlPath, err := utils.LocateRepoFile(compressTestYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", compressTestYaml, err)
	}

	cryptoTestYamlPath, err := utils.LocateRepoFile(cryptoTestYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", cryptoTestYaml, err)
	}

	opensslTestYamlPath, err := utils.LocateRepoFile(opensslTestYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", opensslTestYaml, err)
	}

	var dpPodName string

	ginkgo.BeforeEach(func() {
		ginkgo.By("deploying QAT plugin in DPDK mode")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for QAT plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-qat-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}
		dpPodName = podList.Items[0].Name

		ginkgo.By("checking QAT plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.Context("When QAT Gen4 resources are available", func() {
		ginkgo.BeforeEach(func() {
			ginkgo.By("checking if the resource is allocatable")
			if err := utils.WaitForNodesWithResource(f.ClientSet, "qat.intel.com/cy", 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
			}
		})

		ginkgo.AfterEach(func() {
			ginkgo.By("undeploying QAT plugin")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(kustomizationPath))
			if err := e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, dpPodName, f.Namespace.Name, 30*time.Second); err != nil {
				framework.Failf("failed to terminate pod: %v", err)
			}
		})

		ginkgo.It("deploys a crypto pod requesting QAT resources", func() {
			ginkgo.By("submitting a crypto pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", opensslTestYamlPath)

			ginkgo.By("waiting the crypto pod to finish successfully")
			e2epod.NewPodClient(f).WaitForSuccess("openssl-qat-engine", 300*time.Second)

			output, _ := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, "openssl-qat-engine", "openssl-qat-engine")

			framework.Logf("cpa_sample_code output:\n %s", output)
		})
	})
	ginkgo.Context("When QAT Gen2 resources are available", func() {
		ginkgo.BeforeEach(func() {
			ginkgo.By("checking if the resource is allocatable")
			if err := utils.WaitForNodesWithResource(f.ClientSet, "qat.intel.com/generic", 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
			}
		})

		ginkgo.It("deploys a crypto pod requesting QAT resources", func() {
			ginkgo.By("submitting a crypto pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(cryptoTestYamlPath))

			ginkgo.By("waiting the crypto pod to finish successfully")
			e2epod.NewPodClient(f).WaitForSuccess("qat-dpdk-test-crypto-perf-tc1", 60*time.Second)
		})

		ginkgo.It("deploys a compress pod requesting QAT resources", func() {
			ginkgo.By("submitting a compress pod requesting QAT resources")
			e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(compressTestYamlPath))

			ginkgo.By("waiting the compress pod to finish successfully")
			e2epod.NewPodClient(f).WaitForSuccess("qat-dpdk-test-compress-perf-tc1", 60*time.Second)
		})
	})
}
