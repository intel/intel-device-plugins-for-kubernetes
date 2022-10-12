// Copyright 2021 Intel Corporation. All Rights Reserved.
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

package dlb

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/dlb_plugin/kustomization.yaml"
	demoPFYaml        = "demo/dlb-libdlb-demo-pf-pod.yaml"
	demoVFYaml        = "demo/dlb-libdlb-demo-vf-pod.yaml"
)

func init() {
	ginkgo.Describe("DLB plugin", describe)
}

func describe() {
	f := framework.NewDefaultFramework("dlbplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath, err := utils.LocateRepoFile(kustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationYaml, err)
	}

	ginkgo.It("runs DLB plugin", func() {
		ginkgo.By("deploying DLB plugin")
		framework.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for DLB plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-dlb-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking DLB plugin's securityContext")
		if err := utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.It("checks the availability of DLB resources", func() {
		for _, resource := range []v1.ResourceName{"dlb.intel.com/pf", "dlb.intel.com/vf"} {
			ginkgo.By("checking the " + resource.String() + " resource is allocatable")
			if err := utils.WaitForNodesWithResource(f.ClientSet, resource, 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable resource %s: %v", resource, err)
			}
		}
	})

	ginkgo.It("deploys demo apps", func() {
		for function, yaml := range map[string]string{"PF": demoPFYaml, "VF": demoVFYaml} {
			demoPath, err := utils.LocateRepoFile(yaml)
			if err != nil {
				framework.Failf("unable to locate %q: %v", yaml, err)
			}

			podName := strings.TrimSuffix(filepath.Base(yaml), filepath.Ext(yaml))

			ginkgo.By("submitting a pod requesting DLB " + function + " resources")
			framework.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoPath)

			ginkgo.By("waiting for the DLB demo to succeed")
			f.PodClient().WaitForSuccess(podName, 200*time.Second)

			ginkgo.By("getting workload log")
			log, err := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, podName, podName)

			if err != nil {
				framework.Failf("unable to get log from pod: %v", err)
			}

			framework.Logf("log output: %s", log)
		}
	})
}
