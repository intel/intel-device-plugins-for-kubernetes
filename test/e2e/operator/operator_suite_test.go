// Copyright 2026 Intel Corporation. All Rights Reserved.
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

// Operator e2e test suite for Intel Device Plugins.

package operator

import (
	"context"
	"flag"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	operutils "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/operator/utils"
	"github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	nfdNamespace     = "node-feature-discovery"
	nfdNamespaceOCP  = "openshift-nfd"
	nfdTimeout       = 5 * time.Minute
	nodeReadyTimeout = 30 * time.Second
)

var (
	isOCP bool
)

func init() {
	ginkgo.SynchronizedBeforeSuite(setupCluster, func([]byte) {})
}

// setupCluster runs once on the first Ginkgo node.  It waits for cluster nodes to
// be schedulable and ensures NFD is running (deploying it via kustomize if not
// already present).
func setupCluster(ctx context.Context) []byte {
	c, err := framework.LoadClientset()
	if err != nil {
		framework.Failf("error loading client: %v", err)
	}

	framework.Logf("e2e test version: %s", version.Get().GitVersion)

	if sv, svErr := c.DiscoveryClient.ServerVersion(); svErr == nil && sv != nil {
		framework.Logf("kube-apiserver version: %s", sv.GitVersion)
	}

	framework.ExpectNoError(waitForNodes(ctx, c, nodeReadyTimeout))

	checkRequiredEnvVariables(ctx, c)

	isOCP = operutils.IsRunningOnOCP(ctx, c)

	ensureProjectNamespace(ctx, c, !isOCP)
	ensureNFD(ctx, c)
	ensureNFDRules()

	if !isOCP {
		ensureCertManager(ctx, c)
	}

	framework.Logf("Setup done")

	return []byte{}
}

func waitForNodes(ctx context.Context, c clientset.Interface, timeout time.Duration) error {
	// actually wait for all the nodes to be ready and schedulable before proceeding
	framework.Logf("waiting up to %s for all nodes to be ready and schedulable", timeout)

	nodeList, err := c.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	framework.Logf("cluster has %d node(s)", len(nodeList.Items))

	if err = e2enode.WaitForReadyNodes(ctx, c, len(nodeList.Items), timeout); err != nil {
		return err
	}

	return nil
}

// nfdRunningInNamespace returns true when the given namespace contains at
// least one NFD pod.
func nfdRunningInNamespace(ctx context.Context, c clientset.Interface, ns string) bool {
	podList, err := c.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	return err == nil && len(podList.Items) > 0
}

// ensureNFD checks whether NFD is already running in either the OCP-standard
// namespace (openshift-nfd) or the upstream namespace (node-feature-discovery).
// If NFD is not found in either, it is deployed via the standard kustomize
// manifests.
func ensureNFD(ctx context.Context, c clientset.Interface) {
	for _, ns := range []string{nfdNamespaceOCP, nfdNamespace} {
		if nfdRunningInNamespace(ctx, c, ns) {
			framework.Logf("NFD already running in %s; skipping deployment", ns)
			return
		}
	}

	framework.Logf("NFD not detected in %s or %s, deploying via kustomize...", nfdNamespaceOCP, nfdNamespace)

	utils.Kubectl(nfdNamespace, "apply", "-k", "deployments/nfd/kustomization.yaml")

	if waitErr := waitForNFD(ctx, c); waitErr != nil {
		framework.Failf("NFD pods did not become ready: %v", waitErr)
	}
}

// ensureNFDRules checks whether the intel-dp-devices NodeFeatureRule already
// exists on the cluster.  If it is absent (NFD may have been installed without
// the device-plugin rules, e.g. via the OCP NFD Operator), the rules overlay
// is applied so that nodes get the labels the device-plugin tests rely on.
func ensureNFDRules() {
	// NodeFeatureRule is cluster-scoped; no namespace argument needed.
	if _, err := e2ekubectl.RunKubectl("", "get", "nodefeaturerule", "intel-dp-devices"); err == nil {
		framework.Logf("NodeFeatureRule intel-dp-devices already present; skipping rules deployment")
		return
	}

	framework.Logf("NodeFeatureRule intel-dp-devices not found, applying NFD rules overlay...")
	utils.Kubectl("", "apply", "-k", "deployments/nfd/overlays/node-feature-rules/kustomization.yaml")
}

func waitForNFD(ctx context.Context, c clientset.Interface) error {
	framework.Logf("waiting up to %s for NFD pods to be ready", nfdTimeout)

	_, err := e2epod.WaitForPodsWithLabelRunningReady(
		ctx, c, nfdNamespace,
		labels.Set{"app": "nfd-master"}.AsSelector(),
		1, nfdTimeout,
	)

	return err
}

// ensureProjectNamespace checks that the namespace specified by the PROJECT_NAMESPACE exists.
func ensureProjectNamespace(ctx context.Context, c clientset.Interface, createIfMissing bool) {
	ns := operutils.ProjectNamespace()
	if ns == "" {
		framework.Failf("project namespace cannot be empty")
	}

	if _, err := c.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err != nil {
		framework.Logf("namespace missing, should try to create it: %t", createIfMissing)

		if createIfMissing {
			_, err2 := c.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}, metav1.CreateOptions{})
			if err2 != nil {
				framework.Failf("failed to create project namespace %s: %v", ns, err2)
			}

			framework.Logf("project namespace %s created", ns)
			return
		}

		framework.Failf("project namespace HAS to exist before running. failed to get project namespace %s: %v", ns, err)
	}

	framework.Logf("Project namespace %s exists", ns)
}

func ensureCertManager(ctx context.Context, c clientset.Interface) {
	// Check if cert-manager is already running by looking for the cert-manager namespace.
	if _, err := c.CoreV1().Namespaces().Get(ctx, "cert-manager", metav1.GetOptions{}); err == nil {
		framework.Logf("cert-manager namespace already exists; assuming cert-manager is installed")
		return
	}

	framework.Failf("cert-manager is required for tests running outside of OCP, but cert-manager namespace not found")
}

// checkRequiredEnvVariables ensures that all required environment variables are set and fails the test if any are missing.
func checkRequiredEnvVariables(ctx context.Context, c clientset.Interface) {
	alwaysRequired := []string{"PROJECT_NAMESPACE", "IMAGE_PATH", "PLUGIN_VERSION"}
	for _, envVar := range alwaysRequired {
		if _, found := os.LookupEnv(envVar); !found {
			framework.Failf("environment variable %s must be set", envVar)
		}
	}
}

// TestDevicePluginsOperator is the entry point for the operator e2e test binary.
func TestDevicePluginsOperator(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Device Plugins Operator Suite")
}

func TestMain(m *testing.M) {
	klog.SetOutput(ginkgo.GinkgoWriter)

	logs.InitLogs()
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	// controller-runtime's init() registers "kubeconfig" to flag.CommandLine before TestMain runs.
	// framework.RegisterClusterFlags also registers "kubeconfig", which causes a panic.
	// Remove the controller-runtime registration so the framework's version (which binds to
	// TestContext.KubeConfig) takes over. "formal" is unexported, so unsafe is required.
	fv := reflect.ValueOf(flag.CommandLine).Elem().FieldByName("formal")
	reflect.NewAt(fv.Type(), unsafe.Pointer(fv.UnsafeAddr())).Elem().SetMapIndex(reflect.ValueOf("kubeconfig"), reflect.Value{})
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()

	framework.AfterReadingAllFlags(&framework.TestContext)

	os.Exit(m.Run())
}
