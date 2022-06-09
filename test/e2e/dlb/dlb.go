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
	"context"
	"path/filepath"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/dlb_plugin/kustomization.yaml"
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

	ginkgo.It("checks availability of DLB resources", func() {
		ginkgo.By("deploying DLB plugin")
		framework.RunKubectlOrDie(f.Namespace.Name, "--namespace", f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for DLB plugin's availability")
		if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-dlb-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second); err != nil {
			framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking the pf resource is allocatable")
		if err := utils.WaitForNodesWithResource(f.ClientSet, "dlb.intel.com/pf", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}
		ginkgo.By("checking the vf resource is allocatable")
		if err := utils.WaitForNodesWithResource(f.ClientSet, "dlb.intel.com/vf", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		ginkgo.By("submitting a pod requesting DLB resources")
		podSpec := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "dlbplugin-tester"},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "testcontainer-pf",
						Image: "intel/dlb-libdlb-demo:devel",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{"dlb.intel.com/pf": resource.MustParse("1")},
							Limits:   v1.ResourceList{"dlb.intel.com/pf": resource.MustParse("1")},
						},
					},
					{
						Name:  "testcontainer-vf",
						Image: "intel/dlb-dpdk-demo:devel",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{"dlb.intel.com/vf": resource.MustParse("1")},
							Limits:   v1.ResourceList{"dlb.intel.com/vf": resource.MustParse("1")},
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
