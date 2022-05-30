// Copyright 2020-2022 Intel Corporation. All Rights Reserved.
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

package e2e_test

import (
	"flag"
	"os"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/dlb"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/dsa"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/fpga"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/fpgaadmissionwebhook"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/gpu"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/qat"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/sgx"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/sgxadmissionwebhook"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

func init() {
	ginkgo.SynchronizedBeforeSuite(setupFirstNode, func(data []byte) {})
}

func setupFirstNode() []byte {
	c, err := framework.LoadClientset()
	if err != nil {
		framework.Failf("Error loading client: %v", err)
	}

	// Delete any namespaces except those created by the system. This ensures no
	// lingering resources are left over from a previous test run.
	deleted, err := framework.DeleteNamespaces(c, nil, /* deleteFilter */
		[]string{
			metav1.NamespaceSystem,
			metav1.NamespaceDefault,
			metav1.NamespacePublic,
			v1.NamespaceNodeLease,
			"cert-manager",
		})
	if err != nil {
		framework.Failf("Error deleting orphaned namespaces: %v", err)
	}

	framework.Logf("Waiting for deletion of the following namespaces: %v", deleted)

	if err = framework.WaitForNamespacesDeleted(c, deleted, framework.DefaultPodDeletionTimeout); err != nil {
		framework.Failf("Failed to delete orphaned namespaces %v: %v", deleted, err)
	}

	framework.ExpectNoError(framework.WaitForAllNodesSchedulable(c, framework.TestContext.NodeSchedulableTimeout))

	// Ensure all pods are running and ready before starting tests (otherwise,
	// cluster infrastructure pods that are being pulled or started can block
	// test pods from running, and tests that ensure all pods are running and
	// ready will fail).
	if err = e2epod.WaitForPodsRunningReady(c, metav1.NamespaceSystem, int32(framework.TestContext.MinStartupPods),
		int32(framework.TestContext.AllowedNotReadyNodes), framework.TestContext.SystemPodsStartupTimeout,
		map[string]string{}); err != nil {
		framework.DumpAllNamespaceInfo(c, metav1.NamespaceSystem)
		kubectl.LogFailedContainers(c, metav1.NamespaceSystem, framework.Logf)
		framework.Failf("Error waiting for all pods to be running and ready: %v", err)
	}

	// Log the version of the server and this client.
	framework.Logf("e2e test version: %s", version.Get().GitVersion)

	serverVersion, err := c.DiscoveryClient.ServerVersion()
	if err != nil {
		framework.Logf("Unexpected server error retrieving version: %v", err)
	}

	if serverVersion != nil {
		framework.Logf("kube-apiserver version: %s", serverVersion.GitVersion)
	}

	return []byte{}
}

func TestDevicePlugins(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Device Plugins Suite")
}

func TestMain(m *testing.M) {
	klog.SetOutput(ginkgo.GinkgoWriter)

	logs.InitLogs()
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()

	// Register framework flags, then handle flags.
	framework.AfterReadingAllFlags(&framework.TestContext)

	// Now run the test suite.
	os.Exit(m.Run())
}
