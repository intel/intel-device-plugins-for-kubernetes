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

// Package fpga implements E2E tests for FPGA device plugin.
package fpga

import (
	"context"
	"fmt"
	"os"
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
	ns                      = "inteldeviceplugins-system"
	timeout                 = time.Second * 120
	pluginKustomizationYaml = "deployments/fpga_plugin/base/kustomization.yaml"
	mappingsCollectionYaml  = "deployments/fpga_admissionwebhook/mappings-collection.yaml"
	nlb0NodeResource        = "fpga.intel.com/af-695.d84.aVKNtusxV3qMNmj5-qCB9thCTcSko8QT-J5DNoP5BAs"
	nlb0PodResource         = "fpga.intel.com/arria10.dcp1.2-nlb0-orchestrated"
	nlb3PodResource         = "fpga.intel.com/arria10.dcp1.2-nlb3-orchestrated"
	nlb0PodResourceAF       = "fpga.intel.com/arria10.dcp1.2-nlb0-preprogrammed"
	arria10NodeResource     = "fpga.intel.com/region-69528db6eb31577a8c3668f9faa081f6"
)

func init() {
	ginkgo.Describe("FPGA Plugin E2E tests", describe)
}

func describe() {
	pluginKustomizationPath, err := utils.LocateRepoFile(pluginKustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", pluginKustomizationYaml, err)
	}

	mappingsCollectionPath, err := utils.LocateRepoFile(mappingsCollectionYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", mappingsCollectionYaml, err)
	}

	fmw := framework.NewDefaultFramework("fpgaplugin-e2e")
	fmw.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.It("Run FPGA plugin tests", func() {
		// Run region test case twice to ensure that device is reprogrammed at least once
		runTestCase(fmw, pluginKustomizationPath, mappingsCollectionPath, "region", arria10NodeResource, nlb3PodResource, "nlb3", "nlb0")
		runTestCase(fmw, pluginKustomizationPath, mappingsCollectionPath, "region", arria10NodeResource, nlb0PodResource, "nlb0", "nlb3")
		// Run af test case
		runTestCase(fmw, pluginKustomizationPath, mappingsCollectionPath, "af", nlb0NodeResource, nlb0PodResourceAF, "nlb0", "nlb3")
	})

	ginkgo.It("deploys FPGA plugin with operator", func() {
		utils.Kubectl("", "apply", "-k", "deployments/operator/default/kustomization.yaml")

		if _, err := e2epod.WaitForPodsWithLabelRunningReady(fmw.ClientSet, ns, labels.Set{"control-plane": "controller-manager"}.AsSelector(), 1, timeout); err != nil {
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		utils.Kubectl("", "apply", "-f", "deployments/operator/samples/deviceplugin_v1_fpgadeviceplugin.yaml")

		if _, err := e2epod.WaitForPodsWithLabelRunningReady(fmw.ClientSet, ns, labels.Set{"app": "intel-fpga-plugin"}.AsSelector(), 1, timeout); err != nil {
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		// TODO: Add a check for resources

		utils.Kubectl("", "delete", "-f", "deployments/operator/samples/deviceplugin_v1_fpgadeviceplugin.yaml")

		utils.Kubectl("", "delete", "-k", "deployments/operator/default/kustomization.yaml")
	})
}

func runTestCase(fmw *framework.Framework, pluginKustomizationPath, mappingsCollectionPath, pluginMode, nodeResource, podResource, cmd1, cmd2 string) {
	tmpDir, err := os.MkdirTemp("", "fpgaplugine2etest-"+fmw.Namespace.Name)
	if err != nil {
		framework.Failf("unable to create temp directory: %v", err)
	}

	defer os.RemoveAll(tmpDir)

	err = utils.CreateKustomizationOverlay(fmw.Namespace.Name, filepath.Dir(pluginKustomizationPath)+"/../overlays/"+pluginMode, tmpDir)
	if err != nil {
		framework.Failf("unable to kustomization overlay: %v", err)
	}

	ginkgo.By(fmt.Sprintf("namespace %s: deploying FPGA plugin in %s mode", fmw.Namespace.Name, pluginMode))
	e2ekubectl.RunKubectlOrDie(fmw.Namespace.Name, "apply", "-k", tmpDir)

	ginkgo.By("deploying mappings")
	e2ekubectl.RunKubectlOrDie(fmw.Namespace.Name, "apply", "-f", mappingsCollectionPath)

	waitForPod(fmw, "intel-fpga-plugin")

	resource := v1.ResourceName(nodeResource)

	ginkgo.By("checking if the resource is allocatable")

	if err := utils.WaitForNodesWithResource(fmw.ClientSet, resource, 30*time.Second); err != nil {
		framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
	}

	resource = v1.ResourceName(podResource)
	image := "intel/opae-nlb-demo:devel"

	ginkgo.By("submitting a pod requesting correct FPGA resources")

	pod := createPod(fmw, fmt.Sprintf("fpgaplugin-%s-%s-%s-correct", pluginMode, cmd1, cmd2), resource, image, []string{cmd1, "-S0"})

	ginkgo.By("waiting the pod to finish successfully")
	e2epod.NewPodClient(fmw).WaitForSuccess(pod.ObjectMeta.Name, 60*time.Second)
	// If WaitForSuccess fails, ginkgo doesn't show the logs of the failed container.
	// Replacing WaitForSuccess with WaitForFinish + 'kubelet logs' would show the logs
	//fmw.PodClient().WaitForFinish(pod.ObjectMeta.Name, 60*time.Second)
	//framework.RunKubectlOrDie(fmw.Namespace.Name, "logs", pod.ObjectMeta.Name)

	ginkgo.By("submitting a pod requesting incorrect FPGA resources")

	pod = createPod(fmw, fmt.Sprintf("fpgaplugin-%s-%s-%s-incorrect", pluginMode, cmd1, cmd2), resource, image, []string{cmd2, "-S0"})

	ginkgo.By("waiting the pod failure")
	utils.WaitForPodFailure(fmw, pod.ObjectMeta.Name, 60*time.Second)
}

func createPod(fmw *framework.Framework, name string, resourceName v1.ResourceName, image string, command []string) *v1.Pod {
	resourceList := v1.ResourceList{resourceName: resource.MustParse("1"),
		"cpu":           resource.MustParse("1"),
		"hugepages-2Mi": resource.MustParse("20Mi")}
	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "testcontainer",
					Image:   image,
					Command: command,
					Resources: v1.ResourceRequirements{
						Requests: resourceList,
						Limits:   resourceList,
					},
					SecurityContext: &v1.SecurityContext{
						Capabilities: &v1.Capabilities{
							Add: []v1.Capability{"IPC_LOCK"},
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	pod, err := fmw.ClientSet.CoreV1().Pods(fmw.Namespace.Name).Create(context.TODO(),
		podSpec, metav1.CreateOptions{})
	framework.ExpectNoError(err, "pod Create API error")

	return pod
}

func waitForPod(fmw *framework.Framework, name string) {
	ginkgo.By(fmt.Sprintf("waiting for %s availability", name))

	podList, err := e2epod.WaitForPodsWithLabelRunningReady(fmw.ClientSet, fmw.Namespace.Name,
		labels.Set{"app": name}.AsSelector(), 1, 100*time.Second)
	if err != nil {
		e2edebug.DumpAllNamespaceInfo(fmw.ClientSet, fmw.Namespace.Name)
		e2ekubectl.LogFailedContainers(fmw.ClientSet, fmw.Namespace.Name, framework.Logf)
		framework.Failf("unable to wait for all pods to be running and ready: %v", err)
	}

	ginkgo.By("checking FPGA plugin's securityContext")

	if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
		framework.Failf("container filesystem info checks failed: %v", err)
	}
}
