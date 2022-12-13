// Copyright 2022 Intel Corporation. All Rights Reserved.
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
)

func init() {
	ginkgo.Describe("QAT Gen4 plugin in Crypto mode", describeQatPluginCy)
}

func describeQatPluginCy() {
	f := framework.NewDefaultFramework("qatplugincy")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath, err := utils.LocateRepoFile(qatPluginKustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", qatPluginKustomizationYaml, err)
	}

	opensslTestYamlPath, err := utils.LocateRepoFile(opensslTestYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", opensslTestYaml, err)
	}

	ginkgo.It("measures performance of QAT Cy Services", func() {
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

		ginkgo.By("checking QAT plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking if the resource is allocatable")
		if err := utils.WaitForNodesWithResource(f.ClientSet, "qat.intel.com/cy", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		ginkgo.By("submitting a crypto pod requesting QAT resources")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", opensslTestYamlPath)

		ginkgo.By("waiting the crypto pod to finish successfully")
		e2epod.NewPodClient(f).WaitForSuccess("openssl-qat-engine", 300*time.Second)

		output, _ := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, "openssl-qat-engine", "openssl-qat-engine")

		framework.Logf("cpa_sample_code output:\n %s", output)
	})
}
