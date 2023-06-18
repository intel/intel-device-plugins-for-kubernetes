// Copyright 2022 Intel Corporation. All Rights Reserved.
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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type testCase struct {
	name               string
	metricsData        []string
	expectedLabels     []string
	minLaneCount       int
	allowSubdeviceless bool
}

func createTestCases() []testCase {
	return []testCase{
		{
			name:         "Garbage metrics",
			minLaneCount: 4,
			metricsData: []string{
				`xpum_some_other_data{with_some_label]]]]}`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links="},
		},
		{
			name:         "No xelinks reported",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_some_other_data{with_some_label="foo"} 42`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="0"} 0`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="0"} 4`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links="},
		},
		{
			name:         "Xelinks not on sub devices",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="0",lane_count="4"} 1`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="1",lane_count="4"} 1`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links="},
		},
		{
			name:         "Xelinks not on sub devices when it's allowed",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="0",lane_count="4"} 1`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="1",lane_count="4"} 1`,
				"",
			},
			expectedLabels:     []string{"xpumanager.intel.com/xe-links=0.0-1.0_0.0-1.1"},
			allowSubdeviceless: true,
		},
		{
			name:         "Xelinks without lan counts",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="0"} 1.0`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links="},
		},
		{
			name:         "One xelink",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="0", lan_count="4"} 1.0`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links=0.0-1.0"},
		},
		{
			name:         "One xelink with non xelink",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{local_device_id="99",local_on_subdevice="false",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="0"} 0`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="0", lan_count="4"} 1.0`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links=0.0-1.0"},
		},
		{
			name:         "Cross linked subdevs",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="0",local_numa_index="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,
				`xpum_topology_link{dev_file="card1",dev_name="Intel(R) Graphics [0x0bdb]",pci_bdf="0000:51:00.0",pci_dev="0xbdb",src="direct",uuid="01000000-0000-0000-0000-000000510000",vendor="Intel(R) Corporation",local_cpu_affinity="0-23,48-71",local_device_id="1",local_numa_index="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="1", lan_count="4"} 1`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links=0.0-1.1_0.1-1.0"},
		},
		{
			name:         "One to many",
			minLaneCount: 4,
			metricsData: []string{
				`xpum_topology_link{local_device_id="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="1",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="2", lane_count="4"} 1`,
				`xpum_topology_link{local_device_id="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="3",remote_subdevice_id="0", lan_count="4"} 1`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links=0.0-1.0_0.0-2.2_0.0-3.0"},
		},
		{
			name:         "Many to many",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{local_device_id="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="1",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="3",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="3",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="0",remote_subdevice_id="1", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="2",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links=0.0-2.0_1.0-3.0_0.1-3.1_1.1-2.1"},
		},
		{
			name:         "Too few lanes",
			minLaneCount: 8,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{local_device_id="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="1",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="3",remote_subdevice_id="0", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="3",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="0",remote_subdevice_id="1", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="2",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,
				"",
			},
			expectedLabels: []string{"xpumanager.intel.com/xe-links=1.0-3.0_0.1-3.1"},
		},
		{
			name:         "Multi line label",
			minLaneCount: 4,
			metricsData: []string{
				`# HELP xpum_topology_link Connection type fo two GPU tiles`,
				`# TYPE xpum_topology_link gauge`,
				`xpum_topology_link{local_device_id="0",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="1",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="3",remote_subdevice_id="0", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="3",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="0",remote_subdevice_id="1", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="2",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,

				`xpum_topology_link{local_device_id="4",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="4",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="3",remote_subdevice_id="0", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="5",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="1", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="5",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,

				`xpum_topology_link{local_device_id="6",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="6",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="3",remote_subdevice_id="0", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="7",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="1", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="7",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,

				`xpum_topology_link{local_device_id="8",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="2",remote_subdevice_id="0", lan_count="4"} 1`,
				`xpum_topology_link{local_device_id="8",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="3",remote_subdevice_id="0", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="9",local_on_subdevice="true",local_subdevice_id="0",remote_device_id="0",remote_subdevice_id="1", lan_count="8"} 1`,
				`xpum_topology_link{local_device_id="9",local_on_subdevice="true",local_subdevice_id="1",remote_device_id="1",remote_subdevice_id="1", lan_count="4"} 1`,

				"",
			},
			expectedLabels: []string{
				"xpumanager.intel.com/xe-links=0.0-2.0_1.0-3.0_0.1-3.1_1.1-2.1_2.0-4.0_3.0-4.1_0.1-5.0_1.1-5.1",
				"xpumanager.intel.com/xe-links2=Z_2.0-6.0_3.0-6.1_0.1-7.0_1.1-7.1_2.0-8.0_3.0-8.1_0.1-9.0_1.1-9",
				"xpumanager.intel.com/xe-links3=Z.1",
			},
		},
	}
}

func (tc *testCase) createFakeXMS(data []string, minLaneCount int) *xpuManagerSidecar {
	bytes := []byte(strings.Join(data, "\n"))

	metricsGetter := func() []byte {
		return bytes
	}

	xms := createXPUManagerSidecar()
	xms.getMetricsData = metricsGetter
	xms.laneCount = uint64(minLaneCount)
	xms.labelNamespace = "xpumanager.intel.com"

	return xms
}

func TestLabeling(t *testing.T) {
	tcs := createTestCases()

	for _, tc := range tcs {
		print("Testcase (labeling): ", tc.name, "\n")
		xms := tc.createFakeXMS(tc.metricsData, tc.minLaneCount)

		xms.allowSubdevicelessLinks = tc.allowSubdeviceless

		topologyInfos := xms.GetTopologyFromXPUMMetrics([]byte(strings.Join(tc.metricsData, "\n")))

		labels := xms.createLabels(topologyInfos)
		if !reflect.DeepEqual(labels, tc.expectedLabels) {
			t.Errorf("got %v expected %v\n", labels, tc.expectedLabels)
		}
	}
}

func TestIterate(t *testing.T) {
	tcs := createTestCases()

	for _, tc := range tcs {
		print("Testcase (iterate): ", tc.name, "\n")
		xms := tc.createFakeXMS(tc.metricsData, tc.minLaneCount)

		xms.allowSubdevicelessLinks = tc.allowSubdeviceless

		root, err := os.MkdirTemp("", "test_new_xms")
		if err != nil {
			t.Fatalf("can't create temporary directory: %+v", err)
		}
		// dirs/files need to be removed for the next test
		defer os.RemoveAll(root)

		xms.tmpDirPrefix = root
		xms.dstFilePath = filepath.Join(root, "labels.txt")

		xms.iterate()

		if !xms.compareLabels(tc.expectedLabels) {
			t.Errorf("output file didn't have expected labels\n")
		}
	}
}
