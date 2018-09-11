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
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/pkg/errors"

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
	resourceRemoveOp = `{
                "op": "remove",
                "path": "/spec/containers/%d/resources/%s/%s"
        }`
	resourceAddOp = `{
                "op": "add",
                "path": "/spec/containers/%d/resources/%s/%s",
                "value": "%d"
        }`
	envAddOpTpl = `{
                "op": "add",
                "path": "/spec/containers/{{- .ContainerIdx -}}/env",
                "value": [
                     {{- $first := true -}}
                     {{- range $key, $value := .EnvVars -}}
                       {{- if not $first -}},{{- end -}}{
                          "name": "{{$key}}",
                          "value": "{{$value}}"
                       }
                       {{- $first = false -}}
                     {{- end -}}
                ]
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
		return nil, errors.Errorf("Unknown mode: %s", mode)
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

	return nil, errors.Errorf("Uknown mode: %s", p.mode)
}

func (p *patcher) getPatchOpsPreprogrammed(containerIdx int, container corev1.Container) ([]string, error) {
	var ops []string

	for resourceName, resourceQuantity := range container.Resources.Limits {
		newName, err := p.translateFpgaResourceName(resourceName)
		if err != nil {
			return nil, err
		}
		if len(newName) > 0 {
			op := fmt.Sprintf(resourceReplaceOp, containerIdx,
				"limits", rfc6901Escaper.Replace(string(resourceName)),
				containerIdx, "limits", newName, resourceQuantity.String())
			ops = append(ops, op)
		}
	}
	for resourceName, resourceQuantity := range container.Resources.Requests {
		newName, err := p.translateFpgaResourceName(resourceName)
		if err != nil {
			return nil, err
		}
		if len(newName) > 0 {
			op := fmt.Sprintf(resourceReplaceOp, containerIdx,
				"requests", rfc6901Escaper.Replace(string(resourceName)),
				containerIdx, "requests", newName, resourceQuantity.String())
			ops = append(ops, op)
		}
	}

	return ops, nil
}

func (p *patcher) translateFpgaResourceName(oldname corev1.ResourceName) (string, error) {
	rname := strings.ToLower(string(oldname))
	if !strings.HasPrefix(rname, namespace) {
		return "", nil
	}

	defer p.Unlock()
	p.Lock()

	if newname, ok := p.resourceMap[rname]; ok {
		return newname, nil
	}

	return "", errors.Errorf("Unknown FPGA resource: %s", rname)
}

func (p *patcher) checkResourceRequests(container corev1.Container) error {
	for resourceName, resourceQuantity := range container.Resources.Requests {
		interfaceID, _, err := p.parseResourceName(string(resourceName))
		if err != nil {
			return err
		}

		if interfaceID == "" {
			// Skip non-FPGA resources
			continue
		}
		if container.Resources.Limits[resourceName] != resourceQuantity {
			return errors.Errorf("'limits' and 'requests' for %s must be equal", string(resourceName))
		}
	}

	return nil
}

func (p *patcher) getPatchOpsOrchestrated(containerIdx int, container corev1.Container) ([]string, error) {
	var ops []string

	for _, v := range container.Env {
		if strings.HasPrefix(v.Name, "FPGA_REGION") || strings.HasPrefix(v.Name, "FPGA_AFU") {
			return nil, errors.Errorf("The environment variable '%s' is not allowed", v.Name)
		}
	}

	if err := p.checkResourceRequests(container); err != nil {
		return nil, err
	}

	regions := make(map[string]int64)
	envVars := make(map[string]string)
	counter := 0
	for resourceName, resourceQuantity := range container.Resources.Limits {
		interfaceID, afuID, err := p.parseResourceName(string(resourceName))
		if err != nil {
			return nil, err
		}

		if interfaceID == "" && afuID == "" {
			// Skip non-FPGA resources
			continue
		}

		if container.Resources.Requests[resourceName] != resourceQuantity {
			return nil, errors.Errorf("'limits' and 'requests' for %s must be equal", string(resourceName))
		}

		quantity, ok := resourceQuantity.AsInt64()
		if !ok {
			return nil, errors.New("Resource quantity isn't of integral type")
		}
		regions[interfaceID] = regions[interfaceID] + quantity

		for i := int64(0); i < quantity; i++ {
			counter++
			envVars[fmt.Sprintf("FPGA_REGION_%d", counter)] = interfaceID
			envVars[fmt.Sprintf("FPGA_AFU_%d", counter)] = afuID
		}

		ops = append(ops, fmt.Sprintf(resourceRemoveOp, containerIdx, "limits", rfc6901Escaper.Replace(string(resourceName))))
		ops = append(ops, fmt.Sprintf(resourceRemoveOp, containerIdx, "requests", rfc6901Escaper.Replace(string(resourceName))))
	}

	for interfaceID, quantity := range regions {
		op := fmt.Sprintf(resourceAddOp, containerIdx, "limits", rfc6901Escaper.Replace(namespace+"/region-"+interfaceID), quantity)
		ops = append(ops, op)
		op = fmt.Sprintf(resourceAddOp, containerIdx, "requests", rfc6901Escaper.Replace(namespace+"/region-"+interfaceID), quantity)
		ops = append(ops, op)
	}

	if len(envVars) > 0 {
		for _, envvar := range container.Env {
			envVars[envvar.Name] = envvar.Value
		}
		data := struct {
			ContainerIdx int
			EnvVars      map[string]string
		}{
			ContainerIdx: containerIdx,
			EnvVars:      envVars,
		}
		t := template.Must(template.New("add_operation").Parse(envAddOpTpl))
		buf := new(bytes.Buffer)
		t.Execute(buf, data)
		ops = append(ops, buf.String())
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
				return "", "", errors.Errorf("Unknown region name: %s", result[num])
			}
		case "Af":
			afName = result[num]
		}
	}

	if afName != "" {
		if afuID, ok = p.afMap[regionName+"-"+afName]; !ok {
			return "", "", errors.Errorf("Unknown AF name: %s", regionName+"-"+afName)
		}
	}

	return interfaceID, afuID, nil
}
