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

package qat

import (
	"context"
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
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	qatPluginKernelYaml = "deployments/qat_plugin/base/intel-qat-kernel-plugin.yaml"
)

func init() {
	ginkgo.Describe("QAT plugin in kernel mode", describeQatKernelPlugin)
}

func describeQatKernelPlugin() {
	f := framework.NewDefaultFramework("qatpluginkernel")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	yamlPath, err := utils.LocateRepoFile(qatPluginKernelYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", qatPluginKernelYaml, err)
	}

	ginkgo.It("checks availability of QAT resources", func() {
		ginkgo.By("deploying QAT plugin in kernel mode")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "create", "-f", yamlPath)

		ginkgo.By("waiting for QAT plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-qat-kernel-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking QAT plugin's securityContext")
		if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking the resource is allocatable")
		if err = utils.WaitForNodesWithResource(f.ClientSet, "qat.intel.com/cy1_dc0", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		ginkgo.By("submitting a pod requesting QAT resources")
		podSpec := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "qatplugin-tester"},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Args:    []string{"-c", "echo mode"},
						Name:    "testcontainer",
						Image:   imageutils.GetE2EImage(imageutils.BusyBox),
						Command: []string{"/bin/sh"},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{"qat.intel.com/cy1_dc0": resource.MustParse("1")},
							Limits:   v1.ResourceList{"qat.intel.com/cy1_dc0": resource.MustParse("1")},
						},
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
			},
		}
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(),
			podSpec, metav1.CreateOptions{})
		framework.ExpectNoError(err, "pod Create API error")

		ginkgo.By("waiting the pod to finnish successfully")
		e2epod.NewPodClient(f).WaitForFinish(pod.ObjectMeta.Name, 60*time.Second)
	})
}
