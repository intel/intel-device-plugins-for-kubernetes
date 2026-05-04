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

// Package operator implements e2e tests for the Intel Device Plugins
// Operator. The operator is deployed using a kustomize overlay
// (deployments/operator/overlays/ocp) that replaces cert-manager TLS handling
// with the OCP service-CA operator and grants the required SCC to the default
// service account.
//
// Each plugin (QAT, SGX, DSA) is exercised by applying the corresponding
// Custom Resource and verifying that:
//  1. The plugin DaemonSet pod becomes Running and Ready.
//  2. The expected device resource appears as allocatable on at least one node.
package operator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	operutils "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/operator/utils"
	e2eutils "github.com/intel/intel-device-plugins-for-kubernetes/test/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/yaml"
)

const (
	operatorOverlay    = "deployments/operator/default/kustomization.yaml"
	operatorOCPOverlay = "deployments/operator/overlays/ocp/kustomization.yaml"
	operatorLabel      = "control-plane=controller-manager"
	pluginTimeout      = 90 * time.Second
	resourceTimeout    = 60 * time.Second

	cryptoTestYaml   = "deployments/qat_dpdk_app/crypto-perf/crypto-perf-dpdk-pod-requesting-qat-cy.yaml"
	compressTestYaml = "deployments/qat_dpdk_app/compress-perf/compress-perf-dpdk-pod-requesting-qat-dc.yaml"

	dpdkDemoYaml = "demo/dsa-dpdk-dmadevtest.yaml"
	dpdkPodName  = "dpdk"
)

var (
	operatorNS string
)

type workloadFunc func(ctx context.Context, f *framework.Framework)

func init() {
	ginkgo.Describe("Device Plugins Operator", ginkgo.Label("operator"), ginkgo.Ordered, describe)
}

func describe() {
	f := framework.NewDefaultFramework("inteldeviceplugin-operator-test")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	// All test actions happen in operatorNS, not in a per-spec namespace.
	// Prevent the framework from creating (and leaking) unused namespaces.
	f.SkipNamespaceCreation = true

	var (
		tempDir string
		// BeforeAll/AfterAll run outside the framework's BeforeEach lifecycle so
		// f.ClientSet is nil at that point.  Load the clientset directly here.
		suiteClient clientset.Interface
	)

	ginkgo.BeforeAll(func(ctx context.Context) {
		var err error

		suiteClient, err = framework.LoadClientset()
		if err != nil {
			framework.Failf("failed to load clientset for BeforeAll: %v", err)
		}

		operatorNS = operutils.ProjectNamespace()

		var overlayFile string
		if operutils.IsRunningOnOCP(ctx, suiteClient) {
			overlayFile = operatorOCPOverlay
		} else {
			overlayFile = operatorOverlay
		}

		overlayPath, err := e2eutils.LocateRepoFile(overlayFile)
		if err != nil {
			framework.Failf("unable to locate kustomize overlay %q: %v", overlayFile, err)
		}

		tempDir, err = os.MkdirTemp("", "operator-e2e-")
		if err != nil {
			framework.Failf("unable to create temp directory: %v", err)
		}

		ginkgo.By("creating kustomize overlay (with optional image overrides)")

		if err = operutils.CreateKustomizationOverlay(filepath.Dir(overlayPath), tempDir, operutils.PluginVersion(), operatorNS); err != nil {
			framework.Failf("unable to create kustomization overlay: %v", err)
		}

		ginkgo.By("deploying the device plugins operator via kustomize overlay")
		e2ekubectl.RunKubectlOrDie("", "apply", "-k", tempDir)

		ginkgo.By(fmt.Sprintf("waiting for the operator controller-manager pod to be ready in %s namespace", operatorNS))

		if _, err = e2epod.WaitForPods(
			ctx, suiteClient, operatorNS,
			metav1.ListOptions{LabelSelector: labels.Set{"control-plane": "controller-manager"}.String()},
			e2epod.Range{MinMatching: 1},
			pluginTimeout, "be running and ready", e2epod.RunningReady,
		); err != nil {
			e2edebug.DumpAllNamespaceInfo(ctx, suiteClient, operatorNS)
			e2ekubectl.LogFailedContainers(ctx, suiteClient, operatorNS, framework.Logf)
			framework.Failf("operator controller-manager did not become ready: %v", err)
		}
	})

	ginkgo.AfterAll(func(ctx context.Context) {
		ginkgo.By("removing the device plugins operator")

		if tempDir != "" {
			e2ekubectl.RunKubectlOrDie("", "delete", "--ignore-not-found=true", "-k", tempDir)
			os.RemoveAll(tempDir)
		}
	})

	ginkgo.It("deploys crypto QAT plugin with operator and runs an optional workload", ginkgo.Label("qat", "cy"), func(ctx context.Context) {
		createQatConfigMap(ctx, f, "sym;asym")
		defer deleteQatConfigMap(ctx, f)
		testPluginWithOperator(
			ctx, f,
			buildQATPluginCR(),
			"qatdeviceplugin-sample",
			"intel-qat-plugin",
			[]v1.ResourceName{"qat.intel.com/cy"},
			qatCyWorkload,
		)
	})

	ginkgo.It("deploys data compress QAT plugin with operator and runs an optional workload", ginkgo.Label("qat", "dc"), func(ctx context.Context) {
		createQatConfigMap(ctx, f, "dc")
		defer deleteQatConfigMap(ctx, f)
		testPluginWithOperator(
			ctx, f,
			buildQATPluginCR(),
			"qatdeviceplugin-sample",
			"intel-qat-plugin",
			[]v1.ResourceName{"qat.intel.com/dc"},
			qatDcWorkload,
		)
	})

	ginkgo.It("deploys SGX plugin with operator and runs an optional workload", ginkgo.Label("sgx"), func(ctx context.Context) {
		testPluginWithOperator(
			ctx, f,
			buildSGXPluginCR(),
			"sgxdeviceplugin-sample",
			"intel-sgx-plugin",
			[]v1.ResourceName{"sgx.intel.com/epc", "sgx.intel.com/enclave", "sgx.intel.com/provision"},
			sgxWorkload,
		)
	})

	ginkgo.It("deploys IDXD DSA plugin with operator", ginkgo.Label("dsa", "idxd"), func(ctx context.Context) {
		testPluginWithOperator(
			ctx, f,
			buildDSAPluginCR("idxd"),
			"dsadeviceplugin-sample",
			"intel-dsa-plugin",
			[]v1.ResourceName{"dsa.intel.com/wq-user-dedicated"},
			dsaDpdkWorkload,
		)
	})

	ginkgo.It("deploys VFIO DSA plugin with operator", ginkgo.Label("dsa", "vfio"), func(ctx context.Context) {
		testPluginWithOperator(
			ctx, f,
			buildDSAPluginCR("vfio-pci"),
			"dsadeviceplugin-sample",
			"intel-dsa-plugin",
			[]v1.ResourceName{"dsa.intel.com/vfio"},
		)
	})

	ginkgo.It("deploys IAA plugin with operator", ginkgo.Label("iaa"), func(ctx context.Context) {
		testPluginWithOperator(
			ctx, f,
			buildIAAPluginCR(),
			"iaadeviceplugin-sample",
			"intel-iaa-plugin",
			[]v1.ResourceName{"iaa.intel.com/wq-user-dedicated"},
		)
	})

	ginkgo.It("deploys GPU plugin with operator", ginkgo.Label("gpu", "i915"), func(ctx context.Context) {
		testPluginWithOperator(
			ctx, f,
			buildGPUPluginCR(),
			"gpudeviceplugin-sample",
			"intel-gpu-plugin",
			[]v1.ResourceName{"gpu.intel.com/i915"},
		)
	})
}

// testPluginWithOperator applies the given CR YAML, waits for the plugin
// DaemonSet pod to become ready and for the expected resources to appear as
// allocatable on at least one node, then deletes the CR.
func testPluginWithOperator(
	ctx context.Context,
	f *framework.Framework,
	crYAML, crName, pluginLabel string,
	resourceNames []v1.ResourceName,
	workloadFunc ...workloadFunc,
) {
	tmpFile, err := os.CreateTemp("", "cr-*.yaml")
	if err != nil {
		framework.Failf("unable to create temp CR file: %v", err)
	}

	defer os.Remove(tmpFile.Name())

	if _, err = tmpFile.WriteString(crYAML); err != nil {
		framework.Failf("unable to write CR YAML: %v", err)
	}

	tmpFile.Close()

	ginkgo.By(fmt.Sprintf("applying %s CR", crName))
	e2ekubectl.RunKubectlOrDie("", "apply", "-f", tmpFile.Name())

	defer func() {
		ginkgo.By(fmt.Sprintf("deleting %s CR", crName))
		e2ekubectl.RunKubectlOrDie("", "delete", "--ignore-not-found=true", "-f", tmpFile.Name())
	}()

	ginkgo.By(fmt.Sprintf("waiting for %s plugin pod to be ready", pluginLabel))

	if _, err = e2epod.WaitForPods(
		ctx, f.ClientSet, operatorNS,
		metav1.ListOptions{LabelSelector: labels.Set{"app": pluginLabel}.String()},
		e2epod.Range{MinMatching: 1},
		pluginTimeout, "be running and ready", e2epod.RunningReady,
	); err != nil {
		e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, operatorNS)
		e2ekubectl.LogFailedContainers(ctx, f.ClientSet, operatorNS, framework.Logf)
		framework.Failf("%s plugin pod did not become ready: %v", pluginLabel, err)
	}

	for _, res := range resourceNames {
		ginkgo.By(fmt.Sprintf("checking that resource %s is allocatable", res))

		if err = e2eutils.WaitForNodesWithResource(ctx, f.ClientSet, res, resourceTimeout, e2eutils.WaitForPositiveResource); err != nil {
			framework.Failf("nodes did not report allocatable resource %s: %v", res, err)
		}
	}

	for _, wf := range workloadFunc {
		wf(ctx, f)
	}
}

func createQatConfigMap(ctx context.Context, f *framework.Framework, function string) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "qat-config",
			Namespace: operatorNS,
		},
		Data: map[string]string{
			"qat.conf": fmt.Sprintf("ServicesEnabled=%s\n", function),
		},
	}

	if _, err := f.ClientSet.CoreV1().ConfigMaps(operatorNS).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		framework.Failf("unable to create QAT config ConfigMap: %v", err)
	}
}

func deleteQatConfigMap(ctx context.Context, f *framework.Framework) {
	if err := f.ClientSet.CoreV1().ConfigMaps(operatorNS).Delete(ctx, "qat-config", metav1.DeleteOptions{}); err != nil {
		framework.Logf("unable to delete QAT config ConfigMap: %v", err)
	}
}

func buildQATPluginCR() string {
	qatCr := dpapi.QatDevicePlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dpapi.GroupVersion.String(),
			Kind:       "QatDevicePlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "qatdeviceplugin-sample",
		},
		Spec: dpapi.QatDevicePluginSpec{
			Image:              operutils.PluginImageName("intel-qat-plugin"),
			InitImage:          operutils.PluginImageName("intel-qat-initcontainer"),
			DpdkDriver:         "vfio-pci",
			KernelVfDrivers:    []dpapi.KernelVfDriver{"4xxxvf", "420xxvf"},
			MaxNumDevices:      16,
			ProvisioningConfig: "qat-config",
			LogLevel:           4,
			NodeSelector:       map[string]string{"intel.feature.node.kubernetes.io/qat": "true"},
		},
	}

	output, err := yaml.Marshal(&qatCr)
	if err != nil {
		framework.Failf("unable to marshal QATDevicePlugin CR to YAML: %v", err)
	}

	return string(output)
}

func buildSGXPluginCR() string {
	sgxCr := dpapi.SgxDevicePlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dpapi.GroupVersion.String(),
			Kind:       "SgxDevicePlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "sgxdeviceplugin-sample",
		},
		Spec: dpapi.SgxDevicePluginSpec{
			Image:          operutils.PluginImageName("intel-sgx-plugin"),
			LogLevel:       4,
			NodeSelector:   map[string]string{"intel.feature.node.kubernetes.io/sgx": "true"},
			EnclaveLimit:   110,
			ProvisionLimit: 110,
		},
	}

	output, err := yaml.Marshal(&sgxCr)
	if err != nil {
		framework.Failf("unable to marshal SgxDevicePlugin CR to YAML: %v", err)
	}

	return string(output)
}

func buildDSAPluginCR(driver string) string {
	dsaCr := dpapi.DsaDevicePlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dpapi.GroupVersion.String(),
			Kind:       "DsaDevicePlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "dsadeviceplugin-sample",
		},
		Spec: dpapi.DsaDevicePluginSpec{
			Image:        operutils.PluginImageName("intel-dsa-plugin"),
			InitImage:    operutils.PluginImageName("intel-idxd-config-initcontainer"),
			Driver:       driver,
			LogLevel:     4,
			SharedDevNum: 10,
			NodeSelector: map[string]string{"intel.feature.node.kubernetes.io/dsa": "true"},
		},
	}

	output, err := yaml.Marshal(&dsaCr)
	if err != nil {
		framework.Failf("unable to marshal DsaDevicePlugin CR to YAML: %v", err)
	}

	return string(output)
}

func buildIAAPluginCR() string {
	iaaCr := dpapi.IaaDevicePlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dpapi.GroupVersion.String(),
			Kind:       "IaaDevicePlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "iaadeviceplugin-sample",
		},
		Spec: dpapi.IaaDevicePluginSpec{
			Image:        operutils.PluginImageName("intel-iaa-plugin"),
			InitImage:    operutils.PluginImageName("intel-idxd-config-initcontainer"),
			LogLevel:     4,
			SharedDevNum: 10,
			NodeSelector: map[string]string{"intel.feature.node.kubernetes.io/iaa": "true"},
		},
	}

	output, err := yaml.Marshal(&iaaCr)
	if err != nil {
		framework.Failf("unable to marshal IaaDevicePlugin CR to YAML: %v", err)
	}

	return string(output)
}

func buildGPUPluginCR() string {
	gpuCr := dpapi.GpuDevicePlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dpapi.GroupVersion.String(),
			Kind:       "GpuDevicePlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpudeviceplugin-sample",
		},
		Spec: dpapi.GpuDevicePluginSpec{
			Image:            operutils.PluginImageName("intel-gpu-plugin"),
			LogLevel:         4,
			SharedDevNum:     10,
			EnableMonitoring: true,
			NodeSelector:     map[string]string{"intel.feature.node.kubernetes.io/gpu": "true"},
		},
	}

	output, err := yaml.Marshal(&gpuCr)
	if err != nil {
		framework.Failf("unable to marshal GpuDevicePlugin CR to YAML: %v", err)
	}

	return string(output)
}

func qatDcWorkload(ctx context.Context, f *framework.Framework) {
	compressTestYamlPath, errFailedToLocateRepoFile := e2eutils.LocateRepoFile(compressTestYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", compressTestYaml, errFailedToLocateRepoFile)
	}

	ginkgo.By("create kustomization yaml for workload Pod")
	tmpDir, err := operutils.CreateWorkloadKustomizationFromDir(filepath.Dir(compressTestYamlPath), operutils.PluginVersion())
	if err != nil {
		framework.Failf("unable to create kustomization: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ginkgo.By("submitting a compress pod requesting QAT resources")
	e2ekubectl.RunKubectlOrDie(operatorNS, "apply", "-k", tmpDir)

	defer func() {
		ginkgo.By("deleting the compress test pod")
		e2ekubectl.RunKubectlOrDie(operatorNS, "delete", "--ignore-not-found=true", "-k", tmpDir)
	}()

	ginkgo.By("waiting the compress pod to finish successfully")
	err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, "qat-dpdk-test-compress-perf", operatorNS, 300*time.Second)
	if err != nil {
		if logs, logErr := e2epod.GetPodLogs(ctx, f.ClientSet, operatorNS, "qat-dpdk-test-compress-perf", "compress-perf"); logErr == nil {
			framework.Logf("logs from compress-perf pod:\n%s", logs)
		}
		framework.Failf("compress pod did not finish successfully: %v", err)
	}
}

func qatCyWorkload(ctx context.Context, f *framework.Framework) {
	cryptoTestYamlPath, errFailedToLocateRepoFile := e2eutils.LocateRepoFile(cryptoTestYaml)
	if errFailedToLocateRepoFile != nil {
		framework.Failf("unable to locate %q: %v", cryptoTestYaml, errFailedToLocateRepoFile)
	}

	ginkgo.By("create kustomization yaml for workload Pod")
	tmpDir, err := operutils.CreateWorkloadKustomizationFromFile(cryptoTestYamlPath, operutils.PluginVersion())
	if err != nil {
		framework.Failf("unable to create kustomization: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ginkgo.By("submitting a crypto pod requesting QAT resources")
	e2ekubectl.RunKubectlOrDie(operatorNS, "apply", "-k", tmpDir)

	defer func() {
		ginkgo.By("deleting the crypto test pod")
		e2ekubectl.RunKubectlOrDie(operatorNS, "delete", "--ignore-not-found=true", "-k", tmpDir)
	}()

	ginkgo.By("waiting the crypto pod to finish successfully")
	err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, "qat-dpdk-test-crypto-perf", operatorNS, 300*time.Second)
	if err != nil {
		if logs, logErr := e2epod.GetPodLogs(ctx, f.ClientSet, operatorNS, "qat-dpdk-test-crypto-perf", "crypto-perf"); logErr == nil {
			framework.Logf("logs from crypto-perf pod:\n%s", logs)
		}
		framework.Failf("crypto pod did not finish successfully: %v", err)
	}
}

func sgxWorkload(ctx context.Context, f *framework.Framework) {
	ginkgo.By("creating SGX Pod requesting SGX resources")
	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sgxplugin-tester"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:       "testcontainer",
					Image:      operutils.PluginImageName("sgx-sdk-demo"),
					WorkingDir: "/opt/intel/sgx-sample-app/",
					Command:    []string{"/opt/intel/sgx-sample-app/sgx-sample-app"},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("42")},
						Limits:   v1.ResourceList{"sgx.intel.com/epc": resource.MustParse("42")},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
	pod, err := f.ClientSet.CoreV1().Pods(operatorNS).Create(ctx, podSpec, metav1.CreateOptions{})
	framework.ExpectNoError(err, "pod Create API error")

	defer func() {
		ginkgo.By("deleting the SGX test pod")
		if err = f.ClientSet.CoreV1().Pods(operatorNS).Delete(ctx, pod.ObjectMeta.Name, metav1.DeleteOptions{}); err != nil {
			framework.Logf("unable to delete SGX test pod: %v", err)
		}
	}()

	ginkgo.By("waiting the pod to finish successfully")
	err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, pod.ObjectMeta.Name, operatorNS, 60*time.Second)
	if err != nil {
		if logs, logErr := e2epod.GetPodLogs(ctx, f.ClientSet, operatorNS, pod.ObjectMeta.Name, "testcontainer"); logErr == nil {
			framework.Logf("logs from testcontainer pod:\n%s", logs)
		}
		framework.Failf("testcontainer pod did not finish successfully: %v", err)
	}
}

func dsaDpdkWorkload(ctx context.Context, f *framework.Framework) {
	ginkgo.By("creating DSA Pod requesting DSA resources")

	demoDpdkPath, err := e2eutils.LocateRepoFile(dpdkDemoYaml)
	if err != nil {
		framework.Failf("unable to locate %q: %v", dpdkDemoYaml, err)
	}

	// Create a kustomization yaml on the fly to set correct container image path and version for the deployment
	tmpDir, err := operutils.CreateWorkloadKustomizationFromFile(demoDpdkPath, operutils.PluginVersion())
	if err != nil {
		framework.Failf("unable to create kustomization yaml: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	e2ekubectl.RunKubectlOrDie(operatorNS, "apply", "-k", tmpDir)

	defer func() {
		ginkgo.By("deleting the DSA DPDK test pod")
		e2ekubectl.RunKubectlOrDie(operatorNS, "delete", "--ignore-not-found=true", "-k", tmpDir)
	}()

	ginkgo.By("waiting for the DSA DPDK demo to succeed")
	err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, dpdkPodName, operatorNS, 200*time.Second)
	if err != nil {
		if logs, logErr := e2epod.GetPodLogs(ctx, f.ClientSet, operatorNS, dpdkPodName, dpdkPodName); logErr == nil {
			framework.Logf("logs from DSA DPDK demo pod:\n%s", logs)
		}
		framework.Failf("DSA DPDK demo did not finish successfully: %v", err)
	}
}
