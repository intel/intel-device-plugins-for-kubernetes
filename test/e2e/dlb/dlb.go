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
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/dlb_plugin/overlays/dlb_initcontainer/kustomization.yaml"
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

	var dpPodName string

	ginkgo.JustBeforeEach(func() {
		ginkgo.By("deploying DLB plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for DLB plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-dlb-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}
		dpPodName = podList.Items[0].Name

		ginkgo.By("checking DLB plugin's securityContext")
		if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("undeploying DLB plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(kustomizationPath))
		if err := e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, dpPodName, f.Namespace.Name, 30*time.Second); err != nil {
			framework.Failf("failed to terminate pod: %v", err)
		}
	})

	ginkgo.Context("When PF resources are available", func() {
		ginkgo.JustBeforeEach(func() {
			resource := v1.ResourceName("dlb.intel.com/pf")
			if err := utils.WaitForNodesWithResource(f.ClientSet, resource, 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable resource %s: %v", resource, err)
			}
		})

		ginkgo.It("can run demo app", func() {
			runDemoApp("PF", demoPFYaml, f)
		})
	})

	ginkgo.Context("When VF resources are available", func() {
		ginkgo.JustBeforeEach(func() {
			resource := v1.ResourceName("dlb.intel.com/vf")
			if err := utils.WaitForNodesWithResource(f.ClientSet, resource, 30*time.Second); err != nil {
				framework.Failf("unable to wait for nodes to have positive allocatable resource %s: %v", resource, err)
			}
		})

		ginkgo.It("can run demo app", func() {
			runDemoApp("VF", demoVFYaml, f)
		})
	})
}

func runDemoApp(function, yaml string, f *framework.Framework) {
	demoPath, err := utils.LocateRepoFile(yaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", yaml, err)
	}

	podName := strings.TrimSuffix(filepath.Base(yaml), filepath.Ext(yaml))

	ginkgo.By("submitting a pod requesting DLB " + function + " resources")
	e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoPath)

	ginkgo.By("waiting for the DLB demo to succeed")
	e2epod.NewPodClient(f).WaitForSuccess(podName, 200*time.Second)

	ginkgo.By("getting workload log")

	log, err := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, podName, podName)

	if err != nil {
		framework.Failf("unable to get log from pod: %v", err)
	}

	framework.Logf("log output: %s", log)
}
