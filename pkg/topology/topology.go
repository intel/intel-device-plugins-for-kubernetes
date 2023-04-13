// Copyright 2019 Intel Corporation. All Rights Reserved.
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

package topology

import (
	"sort"
	"strconv"
	"strings"

	topo "github.com/containers/nri-plugins/pkg/topology"
	"github.com/pkg/errors"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// GetTopologyInfo returns topology information for the list of device nodes.
func GetTopologyInfo(devs []string) (*pluginapi.TopologyInfo, error) {
	var result pluginapi.TopologyInfo

	nodeIDs := map[int64]struct{}{}

	for _, dev := range devs {
		sysfsDevice, err := topo.FindSysFsDevice(dev)
		if err != nil {
			return nil, err
		}

		if sysfsDevice == "" {
			return nil, errors.Errorf("device %s doesn't exist", dev)
		}

		hints, err := topo.NewTopologyHints(sysfsDevice)
		if err != nil {
			return nil, err
		}

		for _, hint := range hints {
			if hint.NUMAs != "" {
				for _, nNode := range strings.Split(hint.NUMAs, ",") {
					nNodeID, err := strconv.ParseInt(strings.TrimSpace(nNode), 10, 64)
					if err != nil {
						return nil, errors.Wrapf(err, "unable to convert numa node %s into int64", nNode)
					}

					if nNodeID < 0 {
						return nil, errors.Wrapf(err, "numa node is negative: %d", nNodeID)
					}

					if _, ok := nodeIDs[nNodeID]; !ok {
						result.Nodes = append(result.Nodes, &pluginapi.NUMANode{ID: nNodeID})
						nodeIDs[nNodeID] = struct{}{}
					}
				}
			}
		}
	}

	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })

	return &result, nil
}
