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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/klauspost/cpuid/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	namespace = "sgx.intel.com"
	epc       = "epc"
	capable   = "capable"
)

type patchNodeOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func main() {
	var register, affirm, label, daemon bool
	flag.BoolVar(&register, "register", false, "register EPC as extended resource")
	flag.BoolVar(&affirm, "affirm", false, "return error if EPC is not available")
	flag.BoolVar(&label, "node-label", false, "create node label")
	flag.BoolVar(&daemon, "daemon", false, "run as a daemon")
	flag.Parse()

	klog.Infof("starting sgx_epchook")

	// get the EPC size
	var epcSize uint64
	if cpuid.CPU.SGX.Available {
		for _, s := range cpuid.CPU.SGX.EPCSections {
			epcSize += s.EPCSize
		}
	}
	klog.Infof("epc capacity: %d bytes", epcSize)

	if epcSize == 0 && affirm {
		klog.Fatal("SGX EPC is not available")
	}

	if err := updateNode(epcSize, register, label); err != nil {
		klog.Fatal(err.Error())
	}

	// if the "register" flag is FALSE, we assume that sgx_epchook is used as NFD hook
	if !register {
		fmt.Printf("%s/%s=%d", namespace, epc, epcSize)
	}

	if daemon {
		klog.Info("waiting for termination signal")
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		<-term
	}
}

func updateNode(epcSize uint64, register, label bool) error {
	// create patch payload
	payload := []patchNodeOp{}
	if register {
		payload = append(payload, patchNodeOp{
			Op:    "add",
			Path:  fmt.Sprintf("/status/capacity/%s~1%s", namespace, epc),
			Value: epcSize,
		})
	}
	if label && epcSize > 0 {
		payload = append(payload, patchNodeOp{
			Op:    "add",
			Path:  fmt.Sprintf("/metadata/labels/%s~1%s", namespace, capable),
			Value: "true",
		})
	}
	if len(payload) == 0 {
		return nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// get the node object
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), os.Getenv("NODE_NAME"), metav1.GetOptions{})
	if err != nil {
		return err
	}

	// patch the node
	_, err = clientset.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{}, "status")
	return err
}
