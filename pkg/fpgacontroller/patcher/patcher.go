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

// Package patcher provides functionality required to patch pods by the FPGA admission webhook.
package patcher

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v2"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/internal/containers"
)

const (
	namespace = "fpga.intel.com"

	af     = "af"
	region = "region"
	// "regiondevel" corresponds to the FPGA plugin's regiondevel mode. It requires
	// FpgaRegion CRDs to be added to the cluster.
	regiondevel = "regiondevel"

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
)

type patcher struct {
	sync.Mutex

	log logr.Logger

	afMap           map[string]*fpgav2.AcceleratorFunction
	resourceMap     map[string]string
	resourceModeMap map[string]string
}

func newPatcher(log logr.Logger) *patcher {
	return &patcher{
		log:             log,
		afMap:           make(map[string]*fpgav2.AcceleratorFunction),
		resourceMap:     make(map[string]string),
		resourceModeMap: make(map[string]string),
	}
}

func (p *patcher) AddAf(accfunc *fpgav2.AcceleratorFunction) error {
	defer p.Unlock()
	p.Lock()

	p.afMap[namespace+"/"+accfunc.Name] = accfunc
	if accfunc.Spec.Mode == af {
		devtype, err := fpga.GetAfuDevType(accfunc.Spec.InterfaceID, accfunc.Spec.AfuID)
		if err != nil {
			return err
		}

		p.resourceMap[namespace+"/"+accfunc.Name] = rfc6901Escaper.Replace(namespace + "/" + devtype)
	} else {
		p.resourceMap[namespace+"/"+accfunc.Name] = rfc6901Escaper.Replace(namespace + "/region-" + accfunc.Spec.InterfaceID)
	}
	p.resourceModeMap[namespace+"/"+accfunc.Name] = accfunc.Spec.Mode

	return nil
}

func (p *patcher) AddRegion(region *fpgav2.FpgaRegion) {
	defer p.Unlock()
	p.Lock()

	p.resourceModeMap[namespace+"/"+region.Name] = regiondevel
	p.resourceMap[namespace+"/"+region.Name] = rfc6901Escaper.Replace(namespace + "/region-" + region.Spec.InterfaceID)
}

func (p *patcher) RemoveAf(name string) {
	defer p.Unlock()
	p.Lock()

	delete(p.afMap, namespace+"/"+name)
	delete(p.resourceMap, namespace+"/"+name)
	delete(p.resourceModeMap, namespace+"/"+name)
}

func (p *patcher) RemoveRegion(name string) {
	defer p.Unlock()
	p.Lock()

	delete(p.resourceMap, namespace+"/"+name)
	delete(p.resourceModeMap, namespace+"/"+name)
}

func validateContainer(container corev1.Container) error {
	for _, v := range container.Env {
		if strings.HasPrefix(v.Name, "FPGA_REGION") || strings.HasPrefix(v.Name, "FPGA_AFU") {
			return errors.Errorf("environment variable '%s' is not allowed", v.Name)
		}
	}
	return nil
}

func (p *patcher) getPatchOps(containerIdx int, container corev1.Container) ([]string, error) {
	if err := validateContainer(container); err != nil {
		return nil, err
	}

	requestedResources, err := containers.GetRequestedResources(container, namespace)
	if err != nil {
		return nil, err
	}

	defer p.Unlock()
	p.Lock()

	fpgaPluginMode := ""
	resources := make(map[string]int64)
	envVars := make(map[string]string)
	counter := 0
	ops := make([]string, 0, 2*len(requestedResources))
	for rname, quantity := range requestedResources {
		mode, found := p.resourceModeMap[rname]
		if !found {
			return nil, errors.Errorf("no such resource: %q", rname)
		}

		switch mode {
		case regiondevel, af:
			// Do nothing.
			// The requested resources are exposed by FPGA plugins working in "regiondevel/af" mode.
			// In "regiondevel" mode the workload is supposed to program FPGA regions.
			// A cluster admin has to add FpgaRegion CRDs to allow this.
		case region:
			// Let fpga_crihook know how to program the regions by setting ENV variables.
			// The requested resources are exposed by FPGA plugins working in "region" mode.
			for i := int64(0); i < quantity; i++ {
				counter++
				envVars[fmt.Sprintf("FPGA_REGION_%d", counter)] = p.afMap[rname].Spec.InterfaceID
				envVars[fmt.Sprintf("FPGA_AFU_%d", counter)] = p.afMap[rname].Spec.AfuID
			}
		default:
			// Let admin know about broken af CRD.
			err := errors.Errorf("%q is registered with unknown mode %q instead of %q or %q",
				rname, p.resourceModeMap[rname], af, region)
			p.log.Error(err, "unable to construct patching operations")
			return nil, err
		}

		if fpgaPluginMode == "" {
			fpgaPluginMode = mode
		} else if fpgaPluginMode != mode {
			return nil, errors.New("container cannot be scheduled as it requires resources operated in different modes")
		}

		mappedName := p.resourceMap[rname]
		resources[mappedName] = resources[mappedName] + quantity

		// Add operations to remove unresolved resources from the pod.
		ops = append(ops, fmt.Sprintf(resourceRemoveOp, containerIdx, "limits", rfc6901Escaper.Replace(rname)))
		ops = append(ops, fmt.Sprintf(resourceRemoveOp, containerIdx, "requests", rfc6901Escaper.Replace(rname)))
	}

	// Add operations to add resolved resources to the pod.
	for resource, quantity := range resources {
		op := fmt.Sprintf(resourceAddOp, containerIdx, "limits", resource, quantity)
		ops = append(ops, op)
		op = fmt.Sprintf(resourceAddOp, containerIdx, "requests", resource, quantity)
		ops = append(ops, op)
	}

	// Add the ENV variables to the pod if needed.
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
		if err := t.Execute(buf, data); err != nil {
			return nil, errors.Wrap(err, "unable to execute template")
		}
		ops = append(ops, buf.String())
	}

	return ops, nil
}
