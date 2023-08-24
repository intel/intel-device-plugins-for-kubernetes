// Copyright 2021-2022 Intel Corporation. All Rights Reserved.
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
	"os"

	sgxwebhook "github.com/intel/intel-device-plugins-for-kubernetes/pkg/webhooks/sgx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	klog.InitFlags(nil)
}

func main() {
	ctrl.SetLogger(klogr.New())

	tlsCfgFunc := func(cfg *tls.Config) {
		cfg.MinVersion = tls.VersionTLS13
	}

	webhookOptions := webhook.Options{
		Port: 9443,
		TLSOpts: []func(*tls.Config){
			tlsCfgFunc,
		},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Metrics:       metricsserver.Options{BindAddress: "0"},
		Logger:        ctrl.Log.WithName("SgxAdmissionWebhook"),
		WebhookServer: webhook.NewServer(webhookOptions),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := builder.WebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithDefaulter(&sgxwebhook.Mutator{}).
		Complete(); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Pod")
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
