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
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/dlb"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/dsa"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/fpga"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/fpgaadmissionwebhook"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/gpu"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/iaa"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/operator"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/qat"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/sgx"
	_ "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/sgxadmissionwebhook"
	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

func init() {
	ginkgo.SynchronizedBeforeSuite(setupFirstNode, func(data []byte) {})
}

func setupFirstNode(ctx context.Context) []byte {
	c, err := framework.LoadClientset()
	if err != nil {
		framework.Failf("Error loading client: %v", err)
	}

	// Delete any namespaces except those created by the system. This ensures no
	// lingering resources are left over from a previous test run.
	if framework.TestContext.CleanStart {
		deleted, err2 := framework.DeleteNamespaces(ctx, c, nil, /* deleteFilter */
			[]string{
				metav1.NamespaceSystem,
				metav1.NamespaceDefault,
				metav1.NamespacePublic,
				v1.NamespaceNodeLease,
				"cert-manager",
			})
		if err2 != nil {
			framework.Failf("Error deleting orphaned namespaces: %v", err2)
		}

		framework.Logf("Waiting for deletion of the following namespaces: %v", deleted)

		if err2 = framework.WaitForNamespacesDeleted(ctx, c, deleted, e2epod.DefaultPodDeletionTimeout); err2 != nil {
			framework.Failf("Failed to delete orphaned namespaces %v: %v", deleted, err2)
		}
	}

	timeouts := framework.NewTimeoutContext()

	framework.ExpectNoError(e2enode.WaitForAllNodesSchedulable(ctx, c, timeouts.NodeSchedulable))

	// Ensure all pods are running and ready before starting tests (otherwise,
	// cluster infrastructure pods that are being pulled or started can block
	// test pods from running, and tests that ensure all pods are running and
	// ready will fail).
	if err = e2epod.WaitForPodsRunningReady(ctx, c, metav1.NamespaceSystem, framework.TestContext.MinStartupPods,
		timeouts.SystemPodsStartup); err != nil {
		e2edebug.DumpAllNamespaceInfo(ctx, c, metav1.NamespaceSystem)
		e2ekubectl.LogFailedContainers(ctx, c, metav1.NamespaceSystem, framework.Logf)
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

	utils.Kubectl("node-feature-discovery", "apply", "-k", "deployments/nfd/kustomization.yaml")

	utils.Kubectl("node-feature-discovery", "apply", "-k", "deployments/nfd/overlays/node-feature-rules/kustomization.yaml")

	if err = e2epod.WaitForPodsRunningReady(ctx, c, "node-feature-discovery", 2,
		300*time.Second); err != nil {
		framework.Failf("unable to wait for NFD pods to be running and ready: %v", err)
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
