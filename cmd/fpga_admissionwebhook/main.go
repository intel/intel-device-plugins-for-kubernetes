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
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga/v2"
	controllers "github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpgacontroller"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpgacontroller/patcher"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	klog.InitFlags(nil)

	_ = fpgav2.AddToScheme(scheme)
}

func main() {
	var (
		enableLeaderElection bool
	)

	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(klogr.New())

	webHook := &webhook.Server{
		Port:          9443,
		TLSMinVersion: "1.3",
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Logger:             ctrl.Log.WithName("FpgaAdmissionWebhook"),
		WebhookServer:      webHook,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "f2c6a4df.intel.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	pm := patcher.NewPatcherManager(ctrl.Log.WithName("webhooks").WithName("Fpga"))

	mgr.GetWebhookServer().Register("/pods", &webhook.Admission{
		Handler: admission.HandlerFunc(pm.GetPodMutator()),
	})

	if err = (&controllers.AcceleratorFunctionReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		PatcherManager: pm,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AcceleratorFunction")
		os.Exit(1)
	}

	if err = (&controllers.FpgaRegionReconciler{
		Client:         mgr.GetClient(),
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
