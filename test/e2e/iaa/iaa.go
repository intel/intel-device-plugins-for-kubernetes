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
	ns                = "inteldeviceplugins-system"
	timeout           = time.Second * 120
	kustomizationYaml = "deployments/iaa_plugin/overlays/iaa_initcontainer/iaa_initcontainer.yaml"
	configmapYaml     = "demo/iaa.conf"
	demoYaml          = "demo/iaa-accel-config-demo-pod.yaml"
)

func init() {
	ginkgo.Describe("IAA plugin", describe)
}

func describe() {
	f := framework.NewDefaultFramework("iaaplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath, err := utils.LocateRepoFile(kustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationYaml, err)
	}

	configmap, err := utils.LocateRepoFile(configmapYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", configmapYaml, err)
	}

	demoPath, err := utils.LocateRepoFile(demoYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", demoYaml, err)
	}

	ginkgo.It("runs IAA plugin and a demo workload", func() {
		ginkgo.By("deploying IAA plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "intel-iaa-config", "--from-file="+configmap)

		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for IAA plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-iaa-plugin"}.AsSelector(), 1 /* one replica */, 300*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking IAA plugin's securityContext")
		if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking if the resource is allocatable")
		if err = utils.WaitForNodesWithResource(f.ClientSet, "iaa.intel.com/wq-user-dedicated", 300*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoPath)

		ginkgo.By("waiting for the IAA demo to succeed")

		podName := "iaa-accel-config-demo"
		e2epod.NewPodClient(f).WaitForSuccess(podName, 600*time.Second)

		ginkgo.By("getting workload log")
		log, err := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, podName, podName)

		if err != nil {
			framework.Failf("unable to get log from pod: %v", err)
		}

		framework.Logf("log output: %s", log)

		utils.Kubectl(f.Namespace.Name, "apply", "-f", "demo/iaa-qpl-demo-pod.yaml")

		ginkgo.By("waiting for the IAA QPL demo to succeed")

		podName = "iaa-qpl-demo"
		e2epod.NewPodClient(f).WaitForSuccess(podName, 300*time.Second)

		ginkgo.By("getting workload log")
		log, err = e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, podName, podName)

		if err != nil {
			framework.Failf("unable to get log from pod: %v", err)
		}

		framework.Logf("log output: %s", log)
	})

	ginkgo.It("deploys IAA plugin with operator", func() {
		utils.Kubectl("", "apply", "-k", "deployments/operator/default/kustomization.yaml")

		if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, ns, labels.Set{"control-plane": "controller-manager"}.AsSelector(), 1, timeout); err != nil {
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		utils.Kubectl("", "apply", "-f", "deployments/operator/samples/deviceplugin_v1_iaadeviceplugin.yaml")

		if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, ns, labels.Set{"app": "intel-iaa-plugin"}.AsSelector(), 1, timeout); err != nil {
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		if err := utils.WaitForNodesWithResource(f.ClientSet, "iaa.intel.com/wq-user-dedicated", timeout); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		utils.Kubectl("", "delete", "-f", "deployments/operator/samples/deviceplugin_v1_iaadeviceplugin.yaml")

		utils.Kubectl("", "delete", "-k", "deployments/operator/default/kustomization.yaml")
	})
}
