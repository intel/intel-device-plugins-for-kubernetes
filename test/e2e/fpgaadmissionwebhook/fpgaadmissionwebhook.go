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

package fpgaadmissionwebhook

import (
	"context"
	"os"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	deployScript = "scripts/webhook-deploy.sh"
)

func init() {
	ginkgo.Describe("FPGA Admission Webhook", describe)
}

func describe() {
	f := framework.NewDefaultFramework("webhook")

	webhookDeployPath, err := utils.LocateRepoFile(deployScript)
	if err != nil {
		framework.Failf("unable to locate %q: %v", deployScript, err)
	}

	getEnv := func() []string {
		return append(os.Environ(), "KUBECONFIG="+framework.TestContext.KubeConfig)
	}

	ginkgo.It("mutates created pods to reference resolved AFs", func() {
		ginkgo.By("deploying webhook")
		_, _, err := framework.RunCmdEnv(getEnv(), webhookDeployPath, "--kubectl", framework.TestContext.KubectlPath, "--namespace", f.Namespace.Name)
		framework.ExpectNoError(err)

		checkPodMutation(f, "fpga.intel.com/d5005-nlb3-preprogrammed",
			"fpga.intel.com/af-bfa.f7d.v6xNhR7oVv6MlYZc4buqLfffQFy9es9yIvFEsLk6zRg")
	})

	ginkgo.It("mutates created pods to reference resolved Regions", func() {
		ginkgo.By("deploying webhook")
		_, _, err := framework.RunCmdEnv(getEnv(), webhookDeployPath, "--kubectl", framework.TestContext.KubectlPath, "--namespace", f.Namespace.Name)
		framework.ExpectNoError(err)

		checkPodMutation(f, "fpga.intel.com/arria10.dcp1.0-nlb0-orchestrated",
			"fpga.intel.com/region-ce48969398f05f33946d560708be108a")
	})

	ginkgo.It("mutates created pods to reference resolved Regions in regiondevel mode", func() {
		ginkgo.By("deploying webhook")
		_, _, err := framework.RunCmdEnv(getEnv(), webhookDeployPath, "--kubectl", framework.TestContext.KubectlPath, "--namespace", f.Namespace.Name)
		framework.ExpectNoError(err)

		checkPodMutation(f, "fpga.intel.com/arria10.dcp1.0",
			"fpga.intel.com/region-ce48969398f05f33946d560708be108a")
	})
}

func checkPodMutation(f *framework.Framework, source, expectedMutation v1.ResourceName) {
	ginkgo.By("waiting for webhook's availability")
	if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
		labels.Set{"app": "intel-fpga-webhook"}.AsSelector(), 1 /* one replica */, 10*time.Second); err != nil {
		framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
		kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
		framework.Failf("unable to wait for all pods to be running and ready: %v", err)
	}

	ginkgo.By("submitting a pod for admission")
	podSpec := f.NewTestPod("webhook-tester",
		v1.ResourceList{source: resource.MustParse("1")},
		v1.ResourceList{source: resource.MustParse("1")})
	pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(),
		podSpec, metav1.CreateOptions{})
	framework.ExpectNoError(err, "pod Create API error")

	ginkgo.By("checking the pod has been mutated")
	q, ok := pod.Spec.Containers[0].Resources.Limits[expectedMutation]
	if !ok {
		framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
		kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
		framework.Fail("pod hasn't been mutated")
	}
	gomega.Expect(q.String()).To(gomega.Equal("1"))
}
