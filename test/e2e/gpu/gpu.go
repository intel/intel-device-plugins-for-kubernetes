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

package gpu

import (
	"context"
	"path/filepath"
	"strings"
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
	kustomizationYaml = "deployments/gpu_plugin/kustomization.yaml"
	containerName     = "testcontainer"
)

func init() {
	ginkgo.Describe("GPU plugin", describe)
}

func describe() {
	f := framework.NewDefaultFramework("gpuplugin")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	kustomizationPath, errFailedToLocateRepoFile := utils.LocateRepoFile(kustomizationYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", kustomizationYaml, errFailedToLocateRepoFile)
	}

	ginkgo.It("checks availability of GPU resources", func(ctx context.Context) {
		ginkgo.By("deploying GPU plugin")
		e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for GPU plugin's availability")
		podList, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-gpu-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second)
		if err != nil {
			e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
			e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}

		ginkgo.By("checking GPU plugin's securityContext")
		if err = utils.TestPodsFileSystemInfo(podList.Items); err != nil {
			framework.Failf("container filesystem info checks failed: %v", err)
		}

		ginkgo.By("checking if the resource is allocatable")
		if err = utils.WaitForNodesWithResource(ctx, f.ClientSet, "gpu.intel.com/i915", 30*time.Second); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}

		ginkgo.By("submitting a pod requesting GPU resources")
		podSpec := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "gpuplugin-tester"},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Args:    []string{"-c", "ls /dev/dri"},
						Name:    containerName,
						Image:   imageutils.GetE2EImage(imageutils.BusyBox),
						Command: []string{"/bin/sh"},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{"gpu.intel.com/i915": resource.MustParse("1")},
							Limits:   v1.ResourceList{"gpu.intel.com/i915": resource.MustParse("1")},
						},
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
			},
		}
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podSpec, metav1.CreateOptions{})
		framework.ExpectNoError(err, "pod Create API error")

		ginkgo.By("waiting the pod to finish successfully")
		e2epod.NewPodClient(f).WaitForSuccess(ctx, pod.ObjectMeta.Name, 60*time.Second)

		ginkgo.By("checking log output")
		log, err := e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, pod.Name, containerName)

		if err != nil {
			framework.Failf("unable to get log from pod: %v", err)
		}

		if !strings.Contains(log, "card") || !strings.Contains(log, "renderD") {
			framework.Logf("log output: %s", log)
			framework.Failf("device mounts not found from log")
		}

		framework.Logf("found card and renderD from the log")
	})
}
