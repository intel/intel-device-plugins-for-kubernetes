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

	"github.com/klauspost/cpuid/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	namespace  = "sgx.intel.com"
	epc        = "epc"
	pathPrefix = "/status/capacity"
)

type patchExtendedResource struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint64 `json:"value"`
}

func main() {
	var register, affirm bool
	flag.BoolVar(&register, "register", false, "register EPC as extended resource")
	flag.BoolVar(&affirm, "affirm", false, "return error if EPC is not available")
	flag.Parse()

	// get the EPC size
	var epcSize uint64
	if cpuid.CPU.SGX.Available {
		for _, s := range cpuid.CPU.SGX.EPCSections {
			epcSize += s.EPCSize
		}
	}

	if epcSize == 0 && affirm {
		klog.Fatal("SGX EPC is not available")
	}

	if register {
		if err := registerExtendedResource(epcSize); err != nil {
			klog.Fatal(err.Error())
		}
	} else {
		fmt.Printf("%s/%s=%d", namespace, epc, epcSize)
	}
}

func registerExtendedResource(epcSize uint64) error {
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

	// create and send patch request
	payload := []patchExtendedResource{{
		Op:    "add",
		Path:  fmt.Sprintf("%s/%s~1%s", pathPrefix, namespace, epc),
		Value: epcSize,
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = clientset.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return err
	}
	return nil
}
