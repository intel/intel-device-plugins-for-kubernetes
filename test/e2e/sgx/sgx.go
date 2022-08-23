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

package sgx

import (
	"context"
	"path/filepath"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationWebhook  = "deployments/sgx_admissionwebhook/overlays/default-with-certmanager/kustomization.yaml"
	kustomizationPlugin   = "deployments/sgx_plugin/overlays/epc-hook-initcontainer/kustomization.yaml"
	kustomizationNFD      = "deployments/nfd/overlays/sgx/kustomization.yaml"
	kustomizationNFDRules = "deployments/nfd/overlays/node-feature-rules/kustomization.yaml"
)

func init() {
	ginkgo.Describe("SGX plugin", describe)
}

func describe() {
	f := framework.NewDefaultFramework("sgxplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	deploymentWebhookPath, err := utils.LocateRepoFile(kustomizationWebhook)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationWebhook, err)
	}

	deploymentNFDPath, err := utils.LocateRepoFile(kustomizationNFD)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationNFD, err)
	}

	nodeFeatureRulesPath, err := utils.LocateRepoFile(kustomizationNFDRules)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationNFDRules, err)
	}

	ginkgo.BeforeEach(func() {
		_ = utils.DeployWebhook(f, deploymentWebhookPath)

		msg := framework.RunKubectlOrDie("node-feature-discovery", "apply", "-k", filepath.Dir(deploymentNFDPath))
		framework.Logf("Deploy node-feature-discovery:\n%s", msg)

		msg = framework.RunKubectlOrDie("node-feature-discovery", "apply", "-k", filepath.Dir(nodeFeatureRulesPath))
		framework.Logf("Create NodeFeatureRules:\n%s", msg)

		if err = e2epod.WaitForPodsRunningReady(f.ClientSet, "node-feature-discovery", 2, 0,
			100*time.Second, map[string]string{}); err != nil {
			framework.Failf("unable to wait for NFD pods to be running and ready: %v", err)
		}
	})

	ginkgo.AfterEach(func() {
		msg := framework.RunKubectlOrDie("node-feature-discovery", "delete", "-k", filepath.Dir(deploymentNFDPath))
		framework.Logf("Delete node-feature-discovery:\n%s", msg)
	})

	ginkgo.It("checks availability of SGX resources", func() {
		ginkgo.By("deploying SGX plugin")

		deploymentPluginPath, err := utils.LocateRepoFile(kustomizationPlugin)
		if err != nil {
			framework.Failf("unable to locate %q: %v", kustomizationPlugin, err)
		}

		framework.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(deploymentPluginPath))

		ginkgo.By("waiting for SGX plugin's availability")
		if _, err = e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-sgx-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second); err != nil {
			framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking the resource is allocatable")
		if err = utils.WaitForNodesWithResource(f.ClientSet, "sgx.intel.com/epc", 150*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable epc resource: %v", err)
		}
		if err = utils.WaitForNodesWithResource(f.ClientSet, "sgx.intel.com/enclave", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable enclave resource: %v", err)
		}
		if err = utils.WaitForNodesWithResource(f.ClientSet, "sgx.intel.com/provision", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable provision resource: %v", err)
		}

		ginkgo.By("submitting a pod requesting SGX enclave resources")
		podSpec := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "sgxplugin-tester"},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Args:    []string{"-c", "echo hello world"},
						Name:    "testcontainer",
						Image:   imageutils.GetE2EImage(imageutils.BusyBox),
						Command: []string{"/bin/sh"},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("42")},
							Limits:   v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("42")},
						},
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
			},
		}
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), podSpec, metav1.CreateOptions{})
		framework.ExpectNoError(err, "pod Create API error")

		ginkgo.By("waiting the pod to finnish successfully")
		f.PodClient().WaitForSuccess(pod.ObjectMeta.Name, 60*time.Second)
	})
}
