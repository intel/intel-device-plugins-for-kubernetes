// Copyright 2022 Intel Corporation. All Rights Reserved.
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

// Package inteldevicepluginsoperator implements E2E tests for Intel Device Plugins Operator.
package inteldevicepluginsoperator

import (
	"context"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/operator/default/kustomization.yaml"
	ns                = "inteldeviceplugins-system"
	timeout           = time.Second * 120
)

func init() {
	ginkgo.Describe("Device Plugins Operator", describe)
}

func describe() {
	f := framework.NewDefaultFramework("inteldevicepluginsoperator")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var webhook v1.Pod

	ginkgo.BeforeEach(func(ctx context.Context) {
		ginkgo.By("deploying operator")
		utils.Kubectl("", "apply", "-k", kustomizationYaml)

		if _, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, ns, labels.Set{"control-plane": "controller-manager"}.AsSelector(), 1, timeout); err != nil {
			framework.Failf("unable to wait for all pods to be running and ready: %v", err)
		}
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("undeploying operator")
		utils.Kubectl("", "delete", "-k", kustomizationYaml)
	})

	ginkgo.It("checks the operator webhook pod is safely configured", func(ctx context.Context) {
		err := utils.TestContainersRunAsNonRoot([]v1.Pod{webhook})
		gomega.Expect(err).To(gomega.BeNil())
		err = utils.TestWebhookServerTLS(ctx, f, "https://inteldeviceplugins-webhook-service")
		gomega.Expect(err).To(gomega.BeNil())
	})

	ginkgo.It("deploys IAA plugin with operator", func(ctx context.Context) {
		testPluginWithOperator("iaa", []v1.ResourceName{"iaa.intel.com/wq-user-dedicated"}, f, ctx)
	})

	ginkgo.It("deploys DSA plugin with operator", func(ctx context.Context) {
		testPluginWithOperator("dsa", []v1.ResourceName{"dsa.intel.com/wq-user-dedicated"}, f, ctx)
	})

	ginkgo.It("deploys SGX plugin with operator", func(ctx context.Context) {
		testPluginWithOperator("sgx", []v1.ResourceName{"sgx.intel.com/epc", "sgx.intel.com/enclave", "sgx.intel.com/provision"}, f, ctx)
	})
}

func testPluginWithOperator(deviceName string, resourceNames []v1.ResourceName, f *framework.Framework, ctx context.Context) {
	dpSampleYaml := "deployments/operator/samples/deviceplugin_v1_" + deviceName + "deviceplugin.yaml"

	utils.Kubectl("", "apply", "-f", dpSampleYaml)

	if _, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, ns, labels.Set{"app": "intel-" + deviceName + "-plugin"}.AsSelector(), 1, timeout); err != nil {
		framework.Failf("unable to wait for all pods to be running and ready: %v", err)
	}

	for _, resourceName := range resourceNames {
		if err := utils.WaitForNodesWithResource(ctx, f.ClientSet, resourceName, timeout, utils.WaitForPositiveResource); err != nil {
			framework.Failf("unable to wait for nodes to have positive allocatable resource: %v", err)
		}
	}

	utils.Kubectl("", "delete", "-f", dpSampleYaml)
}
