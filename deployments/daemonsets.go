// Copyright 2021 Intel Corporation. All Rights Reserved.
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

// Package deployments generates plugin DaemonSet object out of plugin yaml.
package deployments

import (
	_ "embed"

	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

//go:embed dlb_plugin/base/*plugin*.yaml
var contentDLB []byte

func DLBPluginDaemonSet() *apps.DaemonSet {
	return getDaemonset(contentDLB).DeepCopy()
}

//go:embed dsa_plugin/base/*plugin*.yaml
var contentDSA []byte

func DSAPluginDaemonSet() *apps.DaemonSet {
	return getDaemonset(contentDSA).DeepCopy()
}

//go:embed fpga_plugin/base/*plugin*.yaml
var contentFPGA []byte

func FPGAPluginDaemonSet() *apps.DaemonSet {
	return getDaemonset(contentFPGA).DeepCopy()
}

//go:embed gpu_plugin/base/*plugin.yaml
var contentGPU []byte

func GPUPluginDaemonSet() *apps.DaemonSet {
	return getDaemonset(contentGPU).DeepCopy()
}

//go:embed qat_plugin/base/*qat-plugin.yaml
var contentQAT []byte

func QATPluginDaemonSet() *apps.DaemonSet {
	return getDaemonset(contentQAT).DeepCopy()
}

//go:embed sgx_plugin/base/*plugin*.yaml
var contentSGX []byte

func SGXPluginDaemonSet() *apps.DaemonSet {
	return getDaemonset(contentSGX).DeepCopy()
}

// getDaemonset unmarshalls yaml content into a DaemonSet object.
func getDaemonset(content []byte) *apps.DaemonSet {
	var result apps.DaemonSet
	err := yaml.Unmarshal(content, &result)
	if err != nil {
		panic(err)
	}
	return &result
}
