// Copyright 2018 Intel Corporation. All Rights Reserved.
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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"

	fpgav1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v1"
)

const (
	namespace = "fpga.intel.com"

	resourceReplaceOp = `{
                "op": "remove",
                "path": "/spec/containers/%d/resources/%s/%s"
        }, {
                "op": "add",
                "path": "/spec/containers/%d/resources/%s/%s",
                "value": %s
        }`
	envAddOp = `{
                "op": "add",
                "path": "/spec/containers/%d/env",
                "value": [{
                        "name": "FPGA_REGION",
                        "value": "%s"
                }, {
                        "name": "FPGA_AFU",
                        "value": "%s"
                } %s]
        }`
)

var (
	rfc6901Escaper = strings.NewReplacer("~", "~0", "/", "~1")
	resourceRe     = regexp.MustCompile(namespace + `/(?P<Region>[[:alnum:]]+)(-(?P<Af>[[:alnum:]]+))?`)
)

type patcher struct {
	sync.Mutex

	mode        string
	regionMap   map[string]string
	afMap       map[string]string
	resourceMap map[string]string
}

func newPatcher(mode string) (*patcher, error) {
	if mode != preprogrammed && mode != orchestrated {
		return nil, fmt.Errorf("Unknown mode: %s", mode)
	}

	return &patcher{
		mode:        mode,
		regionMap:   make(map[string]string),
		afMap:       make(map[string]string),
		resourceMap: make(map[string]string),
	}, nil
}

func (p *patcher) addAf(af *fpgav1.AcceleratorFunction) {
	defer p.Unlock()
	p.Lock()

	p.afMap[af.Name] = af.Spec.AfuID
	p.resourceMap[namespace+"/"+af.Name] = rfc6901Escaper.Replace(namespace + "/af-" + af.Spec.AfuID)
}

func (p *patcher) addRegion(region *fpgav1.FpgaRegion) {
	defer p.Unlock()
	p.Lock()

	p.regionMap[region.Name] = region.Spec.InterfaceID
	p.resourceMap[namespace+"/"+region.Name] = rfc6901Escaper.Replace(namespace + "/region-" + region.Spec.InterfaceID)
}

func (p *patcher) removeAf(name string) {
	defer p.Unlock()
	p.Lock()

	delete(p.afMap, name)
	delete(p.resourceMap, namespace+"/"+name)
}

func (p *patcher) removeRegion(name string) {
	defer p.Unlock()
	p.Lock()

	delete(p.regionMap, name)
	delete(p.resourceMap, namespace+"/"+name)
}

func (p *patcher) getPatchOps(containerIdx int, container corev1.Container) ([]string, error) {
	switch p.mode {
	case preprogrammed:
		return p.getPatchOpsPreprogrammed(containerIdx, container)
	case orchestrated:
		return p.getPatchOpsOrchestrated(containerIdx, container)
	}

	return nil, fmt.Errorf("Uknown mode: %s", p.mode)
}

func (p *patcher) getPatchOpsPreprogrammed(containerIdx int, container corev1.Container) ([]string, error) {
	var ops []string

	for resourceName, resourceQuantity := range container.Resources.Limits {
		newName := p.translateFpgaResourceName(resourceName)
		if len(newName) > 0 {
			op := fmt.Sprintf(resourceReplaceOp, containerIdx,
				"limits", rfc6901Escaper.Replace(string(resourceName)),
				containerIdx, "limits", newName, resourceQuantity.String())
			ops = append(ops, op)
		}
	}
	for resourceName, resourceQuantity := range container.Resources.Requests {
		newName := p.translateFpgaResourceName(resourceName)
		if len(newName) > 0 {
			op := fmt.Sprintf(resourceReplaceOp, containerIdx,
				"requests", rfc6901Escaper.Replace(string(resourceName)),
				containerIdx, "requests", newName, resourceQuantity.String())
			ops = append(ops, op)
		}
	}

	return ops, nil
}

func (p *patcher) translateFpgaResourceName(oldname corev1.ResourceName) string {
	defer p.Unlock()
	p.Lock()

	if newname, ok := p.resourceMap[strings.ToLower(string(oldname))]; ok {
		return newname
	}

	return ""
}

func (p *patcher) getPatchOpsOrchestrated(containerIdx int, container corev1.Container) ([]string, error) {
	var ops []string

	mutated := false
	for resourceName, resourceQuantity := range container.Resources.Limits {
		interfaceID, afuID, err := p.parseResourceName(string(resourceName))
		if err != nil {
			return nil, err
		}

		if interfaceID == "" && afuID == "" {
			continue
		}

		if mutated {
			return nil, fmt.Errorf("Only one FPGA resource per container is supported in '%s' mode", orchestrated)
		}

		op := fmt.Sprintf(resourceReplaceOp, containerIdx, "limits", rfc6901Escaper.Replace(string(resourceName)),
			containerIdx, "limits", rfc6901Escaper.Replace(namespace+"/region-"+interfaceID), resourceQuantity.String())
		ops = append(ops, op)

		if afuID != "" {
			oldVars, err := getEnvVars(container)
			if err != nil {
				return nil, err
			}
			op = fmt.Sprintf(envAddOp, containerIdx, interfaceID, afuID, oldVars)
			ops = append(ops, op)
		}
		mutated = true
	}

	mutated = false
	for resourceName, resourceQuantity := range container.Resources.Requests {
		interfaceID, _, err := p.parseResourceName(string(resourceName))
		if err != nil {
			return nil, err
		}

		if interfaceID == "" {
			continue
		}

		if mutated {
			return nil, fmt.Errorf("Only one FPGA resource per container is supported in '%s' mode", orchestrated)
		}

		op := fmt.Sprintf(resourceReplaceOp, containerIdx, "requests", rfc6901Escaper.Replace(string(resourceName)),
			containerIdx, "requests", rfc6901Escaper.Replace(namespace+"/region-"+interfaceID), resourceQuantity.String())
		ops = append(ops, op)
		mutated = true
	}

	return ops, nil
}

func (p *patcher) parseResourceName(input string) (string, string, error) {
	var interfaceID, afuID string
	var regionName, afName string
	var ok bool

	result := resourceRe.FindStringSubmatch(input)
	if result == nil {
		return "", "", nil
	}

	defer p.Unlock()
	p.Lock()

	for num, group := range resourceRe.SubexpNames() {
		switch group {
		case "Region":
			regionName = result[num]
			if interfaceID, ok = p.regionMap[result[num]]; !ok {
				return "", "", fmt.Errorf("Unknown region name: %s", result[num])
			}
		case "Af":
			afName = result[num]
		}
	}

	if afName != "" {
		if afuID, ok = p.afMap[regionName+"-"+afName]; !ok {
			return "", "", fmt.Errorf("Unknown AF name: %s", regionName+"-"+afName)
		}
	}

	return interfaceID, afuID, nil
}

func getEnvVars(container corev1.Container) (string, error) {
	var jsonstrings []string
	for _, envvar := range container.Env {
		if envvar.Name == "FPGA_REGION" || envvar.Name == "FPGA_AFU" {
			return "", fmt.Errorf("The env var '%s' is not allowed", envvar.Name)
		}
		jsonbytes, err := json.Marshal(envvar)
		if err != nil {
			return "", err
		}
		jsonstrings = append(jsonstrings, string(jsonbytes))
	}

	if len(jsonstrings) == 0 {
		return "", nil
	}

	return fmt.Sprintf(", %s", strings.Join(jsonstrings, ",")), nil
}
