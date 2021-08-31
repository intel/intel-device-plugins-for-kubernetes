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

// Package fpgaadmissionwebhook implements E2E tests for FPGA admission webhook.
package fpgaadmissionwebhook

import (
	"context"
	"path/filepath"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	kustomizationYaml = "deployments/fpga_admissionwebhook/default/kustomization.yaml"
)

func init() {
	ginkgo.Describe("FPGA Admission Webhook", describe)
}

func describe() {
	f := framework.NewDefaultFramework("webhook")

	ginkgo.It("mutates created pods to reference resolved AFs", func() {
		checkPodMutation(f, f.Namespace.Name, "fpga.intel.com/d5005-nlb3-preprogrammed",
			"fpga.intel.com/af-bfa.f7d.v6xNhR7oVv6MlYZc4buqLfffQFy9es9yIvFEsLk6zRg")
	})

	ginkgo.It("mutates created pods to reference resolved Regions", func() {
		checkPodMutation(f, f.Namespace.Name, "fpga.intel.com/arria10.dcp1.0-nlb0-orchestrated",
			"fpga.intel.com/region-ce48969398f05f33946d560708be108a")
	})

	ginkgo.It("mutates created pods to reference resolved Regions in regiondevel mode", func() {
		checkPodMutation(f, f.Namespace.Name, "fpga.intel.com/arria10.dcp1.0",
			"fpga.intel.com/region-ce48969398f05f33946d560708be108a")
	})

	ginkgo.It("doesn't mutate a pod if it's created in a namespace different from mappings'", func() {
		ginkgo.By("create another namespace for mappings")
		ns, err := f.CreateNamespace("mappings", nil)
		framework.ExpectNoError(err, "unable to create a namespace")

		checkPodMutation(f, ns.Name, "fpga.intel.com/arria10.dcp1.0-nlb0-orchestrated",
			"fpga.intel.com/arria10.dcp1.0-nlb0-orchestrated")
	})
}

func checkPodMutation(f *framework.Framework, mappingsNamespace string, source, expectedMutation v1.ResourceName) {
	kustomizationPath, err := utils.LocateRepoFile(kustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationYaml, err)
	}

	ginkgo.By("deploying webhook")
	_ = utils.DeployWebhook(f, kustomizationPath)

	ginkgo.By("deploying mappings")
	framework.RunKubectlOrDie(f.Namespace.Name, "apply", "-n", mappingsNamespace, "-f", filepath.Dir(kustomizationPath)+"/../mappings-collection.yaml")

	ginkgo.By("submitting a pod for admission")
	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "webhook-tester"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "testcontainer",
					Image: imageutils.GetPauseImageName(),
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{source: resource.MustParse("1")},
						Limits:   v1.ResourceList{source: resource.MustParse("1")},
					},
				},
			},
		},
	}
	pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(),
		podSpec, metav1.CreateOptions{})

	if source.String() == expectedMutation.String() {
		framework.ExpectError(err, "pod mistakenly got accepted")
		return
	}

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
