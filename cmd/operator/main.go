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
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga/v2"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dlb"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/dsa"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/fpga"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/gpu"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/iaa"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers/npu"
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

type devicePluginControllerAndWebhook map[string](func(ctrl.Manager, controllers.ControllerOptions) error)

type flagList []string

var supportedDevices = flagList{"dsa", "dlb", "fpga", "gpu", "iaa", "qat", "sgx", "npu"}
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

func createTLSCfgs(enableHTTP2 bool) []func(*tls.Config) {
	tlsCfgFuncs := []func(*tls.Config){
		func(cfg *tls.Config) {
			cfg.MinVersion = tls.VersionTLS12
			cfg.MaxVersion = tls.VersionTLS12
			cfg.CipherSuites = []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			}
		},
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(cfg *tls.Config) {
		setupLog.Info("disabling http/2")

		cfg.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsCfgFuncs = append(tlsCfgFuncs, disableHTTP2)
	}

	return tlsCfgFuncs
}

func main() {
	var (
		metricsAddr           string
		probeAddr             string
		devicePluginNamespace string
		enableLeaderElection  bool
		enableHTTP2           bool
		secureMetrics         bool
		pm                    *patcher.Manager
	)

	tlConf := textlogger.NewConfig()
	tlConf.AddFlags(flag.CommandLine)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&devicePluginNamespace, "deviceplugin-namespace", metav1.NamespaceSystem, "The namespace where deviceplugin daemonsets are created")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"Enable role based authentication/authorization for the metrics endpoint")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"Enable HTTP/2 for the metrics and webhook servers")
	flag.Var(&devices, "devices", "Device(s) to set up.")
	flag.Parse()

	ctrl.SetLogger(textlogger.NewLogger(tlConf))

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
		"npu":  npu.SetupReconciler,
	}

	tlsCfgFuncs := createTLSCfgs(enableHTTP2)

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsCfgFuncs,
	})

	// Metrics endpoint is enabled in 'deployments/operator/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsCfgFuncs,
	}

	if secureMetrics {
		// More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		Logger:                 ctrl.Log.WithName("intel-device-plugins-manager"),
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "d1c7b6d5.intel.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cargs := controllers.ControllerOptions{WithWebhook: true}

	cargs.Namespace = os.Getenv("DEVICEPLUGIN_NAMESPACE")
	if cargs.Namespace == "" {
		cargs.Namespace = devicePluginNamespace
	}

	cargs.ImagePullSecretName = os.Getenv("DEVICEPLUGIN_SECRET")

	for _, device := range devices {
		if err = setupControllerAndWebhook[device](mgr, cargs); err != nil {
			setupLog.Error(err, "unable to initialize controller", "controller", device)
			os.Exit(1)
		}
	}

	if contains(devices, "sgx") {
		if err = (&sgxwebhook.Mutator{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Pod")
			os.Exit(1)
		}
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

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
