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

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v2"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/fpga"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/gpu"
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

func init() {
	// Add schemes for DaemonSets, Pods etc...
	_ = clientgoscheme.AddToScheme(scheme)

	_ = devicepluginv1.AddToScheme(scheme)
	_ = fpgav2.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		Logger:             ctrl.Log.WithName("intel-device-plugins-manager"),
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "d1c7b6d5.intel.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = gpu.SetupReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GpuDevicePlugin")
		os.Exit(1)
	}
	if err = (&devicepluginv1.GpuDevicePlugin{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GpuDevicePlugin")
		os.Exit(1)
	}

	if err = qat.SetupReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QatDevicePlugin")
		os.Exit(1)
	}
	if err = (&devicepluginv1.QatDevicePlugin{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "QatDevicePlugin")
		os.Exit(1)
	}

	if err = sgx.SetupReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SgxDevicePlugin")
		os.Exit(1)
	}
	if err = (&devicepluginv1.SgxDevicePlugin{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "SgxDevicePlugin")
		os.Exit(1)
	}

	pm := patcher.NewPatcherManager(mgr.GetLogger().WithName("webhooks").WithName("Fpga"))

	mgr.GetWebhookServer().Register("/pods", &webhook.Admission{
		Handler: admission.HandlerFunc(pm.GetPodMutator()),
	})

	mgr.GetWebhookServer().Register("/pods-sgx", &webhook.Admission{
		Handler: &sgxwebhook.SgxMutator{Client: mgr.GetClient()},
	})

	if err = fpga.SetupReconciler(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FpgaDevicePlugin")
		os.Exit(1)
	}
	if err = (&devicepluginv1.FpgaDevicePlugin{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "FpgaDevicePlugin")
		os.Exit(1)
	}

	if err = (&fpgacontroller.AcceleratorFunctionReconciler{
		Client:         mgr.GetClient(),
		Log:            mgr.GetLogger().WithName("controllers").WithName("AcceleratorFunction"),
		Scheme:         mgr.GetScheme(),
		PatcherManager: pm,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AcceleratorFunction")
		os.Exit(1)
	}

	if err = (&fpgacontroller.FpgaRegionReconciler{
		Client:         mgr.GetClient(),
		Log:            mgr.GetLogger().WithName("controllers").WithName("FpgaRegion"),
		Scheme:         mgr.GetScheme(),
		PatcherManager: pm,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FpgaRegion")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
