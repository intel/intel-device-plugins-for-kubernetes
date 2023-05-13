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

	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	kustomizationYaml = "deployments/operator/default/kustomization.yaml"
)

func init() {
	ginkgo.Describe("Device Plugins Operator", describe)
}

func describe() {
	f := framework.NewDefaultFramework("inteldevicepluginsoperator")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var webhook v1.Pod

	ginkgo.BeforeEach(func(ctx context.Context) {
		kustomizationPath, err := utils.LocateRepoFile(kustomizationYaml)
		if err != nil {
			framework.Failf("unable to locate %q: %v", kustomizationYaml, err)
		}
		webhook = utils.DeployWebhook(ctx, f, kustomizationPath)
	})

	ginkgo.It("checks the operator webhook pod is safely configured", func(ctx context.Context) {
		err := utils.TestContainersRunAsNonRoot([]v1.Pod{webhook})
		gomega.Expect(err).To(gomega.BeNil())
		err = utils.TestWebhookServerTLS(ctx, f, "https://inteldeviceplugins-webhook-service")
		gomega.Expect(err).To(gomega.BeNil())
	})
}
