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

package envtest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/intel/intel-device-plugins-for-kubernetes/deployments"
	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	ctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
	dlbctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dlb"
	dsactr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dsa"
	fpgactr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/fpga"
	gpuctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/gpu"
	iaactr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/iaa"
	qatctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/qat"
	sgxctr "github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/sgx"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg         *rest.Config
	k8sClient   client.Client
	testEnv     *envtest.Environment
	ctx         context.Context
	cancel      context.CancelFunc
	err         error
	ns          = metav1.NamespaceSystem
	version     = ctr.ImageMinVersion.String()
	prevVersion = ctr.ImageMinVersion.WithMinor(ctr.ImageMinVersion.Minor() - 1).String()
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t,
		"Controller Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")

	logf.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "operator", "crd", "bases")},
	}
	cfg, err = testEnv.Start()

	Expect(err == nil && cfg != nil).To(BeTrue())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})

	Expect(err == nil && k8sClient != nil).To(BeTrue())

	Expect(devicepluginv1.AddToScheme(scheme.Scheme)).To(BeNil())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	Expect(testEnv.Stop()).To(BeNil())
})

var _ = BeforeEach(func() {
	up()
})

var _ = AfterEach(func() {
	down()
})

func up() {
	k8sManager, managerErr := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme, Metrics: metricsserver.Options{BindAddress: "0"}})
	Expect(managerErr).To(BeNil())

	withWebhook := true

	Expect(dlbctr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	Expect(dsactr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	Expect(fpgactr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	Expect(gpuctr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	Expect(iaactr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	Expect(qatctr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	Expect(sgxctr.SetupReconciler(k8sManager, ns, !withWebhook)).To(BeNil())

	ctx, cancel = context.WithCancel(context.TODO())

	go func() {
		Expect(k8sManager.Start(ctx)).To(BeNil())
	}()

	time.Sleep(time.Second)
}

func CleanupAfter(name string, cr client.Object) {
	namespacedName := types.NamespacedName{Namespace: ns, Name: name + "deviceplugin-test"}

	if err := k8sClient.Get(ctx, namespacedName, cr); err == nil {
		Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
	}
}

func down() {
	time.Sleep(time.Second)

	ctx = context.TODO()

	cancel()
}

func testUpgrade(name string, dp interface{}, pimage, pinitimage *string) {
	down()

	prefix := "intel/intel-" + name
	image0 := prefix + "-plugin:" + prevVersion
	initimage0 := prefix + "-initcontainer:" + prevVersion
	image := prefix + "-plugin:" + version
	initimage := prefix + "-initcontainer:" + version

	*pimage = image

	if pinitimage != nil {
		*pinitimage = initimage
	}

	ds0 := makeDaemonSet(name, image0, initimage0)

	Expect(k8sClient.Create(ctx, ds0)).To(BeNil())

	dp0 := makeDevicePlugin(name, image0, initimage0)

	Expect(k8sClient.Create(ctx, dp0)).To(BeNil())

	up()

	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, dp.(client.Object))).To(BeNil())

	ds := &apps.DaemonSet{}

	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "intel-" + name + "-plugin"}, ds)).To(BeNil())

	Expect(ds.Spec.Template.Spec.Containers[0].Image == image).To(BeTrue())

	if pinitimage != nil {
		Expect(ds.Spec.Template.Spec.InitContainers[0].Image == initimage).To(BeTrue())
	}

	Expect(k8sClient.Delete(ctx, dp.(client.Object))).To(BeNil())
}

func makeDevicePlugin(name, image, initimage string) client.Object {
	var obj client.Object

	switch name {
	case "dlb":
		obj = &devicepluginv1.DlbDevicePlugin{
			Spec: devicepluginv1.DlbDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	case "dsa":
		obj = &devicepluginv1.DsaDevicePlugin{
			Spec: devicepluginv1.DsaDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	case "fpga":
		obj = &devicepluginv1.FpgaDevicePlugin{
			Spec: devicepluginv1.FpgaDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	case "gpu":
		obj = &devicepluginv1.GpuDevicePlugin{
			Spec: devicepluginv1.GpuDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	case "iaa":
		obj = &devicepluginv1.IaaDevicePlugin{
			Spec: devicepluginv1.IaaDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	case "qat":
		obj = &devicepluginv1.QatDevicePlugin{
			Spec: devicepluginv1.QatDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	case "sgx":
		obj = &devicepluginv1.SgxDevicePlugin{
			Spec: devicepluginv1.SgxDevicePluginSpec{
				Image:     image,
				InitImage: initimage,
			},
		}
	}

	obj.SetName(name)

	return obj
}

func makeDaemonSet(name, image, initimage string) *apps.DaemonSet {
	var ds *apps.DaemonSet

	initcontainerName := "intel-" + name + "-initcontainer"

	switch name {
	case "dlb":
		ds = deployments.DLBPluginDaemonSet()
	case "dsa":
		ds = deployments.DSAPluginDaemonSet()
		initcontainerName = "intel-idxd-config-initcontainer"
	case "gpu":
		ds = deployments.GPUPluginDaemonSet()
	case "fpga":
		ds = deployments.FPGAPluginDaemonSet()
	case "iaa":
		ds = deployments.IAAPluginDaemonSet()
		initcontainerName = "intel-idxd-config-initcontainer"
	case "qat":
		ds = deployments.QATPluginDaemonSet()
	case "sgx":
		ds = deployments.SGXPluginDaemonSet()
	}

	ds.ObjectMeta.Namespace = ns
	ds.Spec.Template.Spec.Containers[0].Image = image

	if len(initimage) > 0 {
		ds.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  initcontainerName,
			Image: initimage,
		}}
	}

	title := cases.Title(language.English, cases.NoLower)
	yes := true
	ds.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "deviceplugin.intel.com/v1",
		Kind:       title.String(name) + "DevicePlugin",
		Name:       name,
		Controller: &yes,
		UID:        uuid.NewUUID(),
	}}

	return ds
}
