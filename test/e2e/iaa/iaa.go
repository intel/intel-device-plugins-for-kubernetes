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
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/iaa_plugin/overlays/iaa_initcontainer/iaa_initcontainer.yaml"
	configmapYaml     = "demo/iax.conf"
	demoYaml          = "demo/iaa-accel-config-demo-pod.yaml"
	podName           = "iaa-accel-config-demo"
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

	ginkgo.It("runs IAA plugin", func() {
		ginkgo.By("deploying IAA plugin")
		framework.RunKubectlOrDie(f.Namespace.Name, "create", "configmap", "intel-iaa-config", "--from-file="+configmap)

		framework.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for IAA plugin's availability")
		if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-iaa-plugin"}.AsSelector(), 1 /* one replica */, 300*time.Second); err != nil {
			framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}
	})

	ginkgo.It("checks the availability of IAA resources", func() {
		ginkgo.By("checking if the resource is allocatable")
		if err := utils.WaitForNodesWithResource(f.ClientSet, "iaa.intel.com/wq-user-dedicated", 300*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}
	})

	ginkgo.It("deploys a demo app", func() {
		framework.RunKubectlOrDie(f.Namespace.Name, "apply", "-f", demoPath)

		ginkgo.By("waiting for the IAA demo to succeed")
		f.PodClient().WaitForSuccess(podName, 200*time.Second)

		ginkgo.By("getting workload log")
		log, err := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, podName, podName)

		if err != nil {
			framework.Failf("unable to get log from pod: %v", err)
		}

		framework.Logf("log output: %s", log)
	})
}
