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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga/v2"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dlb"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dsa"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/fpga"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/gpu"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/iaa"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/qat"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/sgx"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpgacontroller"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpgacontroller/patcher"
	sgxwebhook "github.com/intel/intel-device-plugins-for-kubernetes/pkg/webhooks/sgx"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

//nolint:wsl
func init() {
	// Add schemes for DaemonSets, Pods etc...
	_ = clientgoscheme.AddToScheme(scheme)

	_ = devicepluginv1.AddToScheme(scheme)
	_ = fpgav2.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

type devicePluginControllerAndWebhook map[string](func(ctrl.Manager, string, bool) error)

type flagList []string

var supportedDevices = flagList{"dsa", "dlb", "fpga", "gpu", "iaa", "qat", "sgx"}
var devices flagList

func (flag *flagList) String() string {
	return strings.Join(*flag, ", ")
}

func (flag *flagList) Set(value string) error {
	if !contains(supportedDevices, value) {
		setupLog.Error(nil, fmt.Sprintf("Unsupported device: %s", value))
		os.Exit(1)
	}

	if contains(devices, value) {
		setupLog.Error(nil, fmt.Sprintf("Duplicate device: %s", value))
		os.Exit(1)
	}

	*flag = append(*flag, value)

	return nil
}

func contains(arr []string, val string) bool {
	for _, s := range arr {
		if s == val {
			return true
		}
	}

	return false
}

func main() {
	var (
		metricsAddr           string
		devicePluginNamespace string
		enableLeaderElection  bool
		pm                    *patcher.Manager
	)

	ctrl.SetLogger(klogr.NewWithOptions(klogr.WithFormat(klogr.FormatKlog)))

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&devicePluginNamespace, "deviceplugin-namespace", metav1.NamespaceSystem, "The namespace where deviceplugin daemonsets are created")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Var(&devices, "devices", "Device(s) to set up.")
	flag.Parse()

	if len(devices) == 0 {
		devices = supportedDevices
	}

	setupControllerAndWebhook := devicePluginControllerAndWebhook{
		"dlb":  dlb.SetupReconciler,
		"dsa":  dsa.SetupReconciler,
		"fpga": fpga.SetupReconciler,
		"gpu":  gpu.SetupReconciler,
		"iaa":  iaa.SetupReconciler,
		"qat":  qat.SetupReconciler,
		"sgx":  sgx.SetupReconciler,
	}

	webHook := &webhook.Server{
		Port:          9443,
		TLSMinVersion: "1.3",
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Logger:             ctrl.Log.WithName("intel-device-plugins-manager"),
		WebhookServer:      webHook,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "d1c7b6d5.intel.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ns := os.Getenv("DEVICEPLUGIN_NAMESPACE")
	if ns == "" {
		ns = devicePluginNamespace
	}

	withWebhook := true

	for _, device := range devices {
		if err = setupControllerAndWebhook[device](mgr, ns, withWebhook); err != nil {
			setupLog.Error(err, "unable to initialize controller", "controller", device)
			os.Exit(1)
		}
	}

	if contains(devices, "sgx") {
		mgr.GetWebhookServer().Register("/pods-sgx", &webhook.Admission{
			Handler: &sgxwebhook.Mutator{Client: mgr.GetClient()},
		})
	}

	if contains(devices, "fpga") {
		pm = patcher.NewPatcherManager(mgr.GetLogger().WithName("webhooks").WithName("Fpga"))

		mgr.GetWebhookServer().Register("/pods", &webhook.Admission{
			Handler: admission.HandlerFunc(pm.GetPodMutator()),
		})

		if err = (&fpgacontroller.AcceleratorFunctionReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			PatcherManager: pm,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "AcceleratorFunction")
			os.Exit(1)
		}

		if err = (&fpgacontroller.FpgaRegionReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			PatcherManager: pm,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "FpgaRegion")
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
