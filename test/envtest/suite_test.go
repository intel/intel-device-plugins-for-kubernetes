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

package envtest

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	dlbctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dlb"
	dsactr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dsa"
	fpgactr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/fpga"
	gpuctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/gpu"
	qatctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/qat"
	sgxctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/sgx"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t,
		"Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(klogr.New())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "operator", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = devicepluginv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	withWebhook := true

	err = gpuctr.SetupReconciler(k8sManager, metav1.NamespaceSystem, !withWebhook)
	Expect(err).ToNot(HaveOccurred())
	err = sgxctr.SetupReconciler(k8sManager, metav1.NamespaceSystem, !withWebhook)
	Expect(err).ToNot(HaveOccurred())
	err = qatctr.SetupReconciler(k8sManager, metav1.NamespaceSystem, !withWebhook)
	Expect(err).ToNot(HaveOccurred())
	err = fpgactr.SetupReconciler(k8sManager, metav1.NamespaceSystem, !withWebhook)
	Expect(err).ToNot(HaveOccurred())
	err = dsactr.SetupReconciler(k8sManager, metav1.NamespaceSystem, !withWebhook)
	Expect(err).ToNot(HaveOccurred())
	err = dlbctr.SetupReconciler(k8sManager, metav1.NamespaceSystem, !withWebhook)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
