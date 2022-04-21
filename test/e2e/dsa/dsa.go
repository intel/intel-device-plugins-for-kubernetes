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

package dsa

import (
	"path/filepath"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	kustomizationYaml = "deployments/dsa_plugin/overlays/dsa_initcontainer/dsa_initcontainer.yaml"
	configmapYaml = "demo/dsa.conf"
)

func init() {
	ginkgo.Describe("DSA plugin", describe)
}

func describe() {
	f := framework.NewDefaultFramework("dsaplugin")

	kustomizationPath, err := utils.LocateRepoFile(kustomizationYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", kustomizationYaml, err)
	}

	configmap, err := utils.LocateRepoFile(configmapYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", configmapYaml, err)
	}

	ginkgo.It("checks availability of DSA resources", func() {
		ginkgo.By("deploying DSA plugin")
		framework.RunKubectlOrDie(f.Namespace.Name, "--namespace", f.Namespace.Name, "create", "configmap", "intel-dsa-config", "--from-file=" + configmap)

		framework.RunKubectlOrDie(f.Namespace.Name, "--namespace", f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

		ginkgo.By("waiting for DSA plugin's availability")
		if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
			labels.Set{"app": "intel-dsa-plugin"}.AsSelector(), 1 /* one replica */, 100*time.Second); err != nil {
			framework.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
			kubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}
	})
}
