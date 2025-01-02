// Copyright 2021-2023 Intel Corporation. All Rights Reserved.
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

package rm

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

type mockPodResources struct {
	pods []v1.Pod
}

func (w *mockPodResources) List(ctx context.Context,
	in *podresourcesv1.ListPodResourcesRequest,
	opts ...grpc.CallOption) (*podresourcesv1.ListPodResourcesResponse, error) {
	resp := podresourcesv1.ListPodResourcesResponse{}
	for _, pod := range w.pods {
		resp.PodResources = append(resp.PodResources, &podresourcesv1.PodResources{
			Name: pod.ObjectMeta.Name, Namespace: pod.ObjectMeta.Namespace, Containers: []*podresourcesv1.ContainerResources{{}},
		})
	}

	return &resp, nil
}
func (w *mockPodResources) GetAllocatableResources(ctx context.Context,
	in *podresourcesv1.AllocatableResourcesRequest,
	opts ...grpc.CallOption) (*podresourcesv1.AllocatableResourcesResponse, error) {
	return nil, nil
}

func (w *mockPodResources) Get(ctx context.Context,
	in *podresourcesv1.GetPodResourcesRequest,
	opts ...grpc.CallOption) (*podresourcesv1.GetPodResourcesResponse, error) {
	return nil, nil
}

func newMockResourceManager(pods []v1.Pod) ResourceManager {
	client, err := grpc.NewClient("fake", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)

		os.Exit(1)
	}

	mc := fake.NewClientset()

	for _, p := range pods {
		_, err = mc.CoreV1().Pods(p.Namespace).Create(context.Background(), &p, metav1.CreateOptions{})
		if err != nil {
			fmt.Printf("failed to Create Pod: %v\n", err)
		}
	}

	rm := resourceManager{
		clientset: mc,
		nodeName:  "TestNode",
		prGetClientFunc: func(string, time.Duration, int) (podresourcesv1.PodResourcesListerClient, *grpc.ClientConn, error) {
			return &mockPodResources{pods: pods}, client, nil
		},
		skipID:            "all",
		fullResourceNames: []string{"gpu.intel.com/i915", "gpu.intel.com/xe"},
		assignments:       make(map[string]podAssignmentDetails),
		retryTimeout:      1 * time.Millisecond,
		useKubelet:        false,
	}

	deviceInfoMap := NewDeviceInfoMap()
	deviceInfoMap["card0-0"] = NewDeviceInfo([]v1beta1.DeviceSpec{
		{
			ContainerPath: "containerpath",
			HostPath:      "hostpath",
			Permissions:   "rw",
		},
	},
		[]v1beta1.Mount{{}},
		map[string]string{"more": "coverage"})
	deviceInfoMap["card1-0"] = NewDeviceInfo([]v1beta1.DeviceSpec{{}}, nil, nil)
	deviceInfoMap["card2-0"] = NewDeviceInfo([]v1beta1.DeviceSpec{{}}, nil, nil)
	rm.SetDevInfos(deviceInfoMap)

	return &rm
}

type preferredTestCase struct {
	name                 string
	pods                 []v1.Pod
	containerRequests    []*v1beta1.ContainerPreferredAllocationRequest
	expectDevices        []string
	expectedContainerLen int
}

type testCase struct {
	name                  string
	pods                  []v1.Pod
	prefContainerRequests []*v1beta1.ContainerPreferredAllocationRequest
	containerRequests     []*v1beta1.ContainerAllocateRequest
	prefExpectErr         bool
	expectErr             bool
}

func TestNewResourceManager(t *testing.T) {
	// normal clientset is unavailable inside the unit tests
	_, err := NewResourceManager("foo", []string{"bar"})

	if err == nil {
		t.Errorf("unexpected success")
	}
}

func TestGetPreferredFractionalAllocation(t *testing.T) {
	properTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{gasCardAnnotation: "card0"},
			Name:        "TestPod",
			Namespace:   "neimspeis",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("1"),
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
	}

	gpuLessTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "TestPodLessGpu",
			Namespace: "neimspeis",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.less.com/i915": resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	properTestPodMultiGpu := *properTestPod.DeepCopy()
	properTestPodMultiGpu.ObjectMeta.Annotations[gasCardAnnotation] = "card0,card1"

	properTestPodMultiGpu2 := *properTestPod.DeepCopy()
	properTestPodMultiGpu2.ObjectMeta.Annotations[gasCardAnnotation] = "card0,card1,card0"

	monitoringPod := *properTestPod.DeepCopy()
	delete(monitoringPod.Spec.Containers[0].Resources.Requests, "gpu.intel.com/i915")
	monitoringPod.Spec.Containers[0].Resources.Requests["gpu.intel.com/i915_monitoring"] = resource.MustParse("1")

	allContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"all"},
			AllocationSize: 1},
	}

	properPrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1"},
			AllocationSize: 1},
	}

	outofRangePrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card6-0", "card5-1"},
			AllocationSize: 1},
	}

	mustHaveContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1"},
			MustIncludeDeviceIDs: []string{"card0-2"},
			AllocationSize:       2},
	}

	properPrefContainerRequests3 := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1"},
			AllocationSize: 3},
	}

	testCases := []preferredTestCase{
		{
			name:                 "Wrong number of container requests should result in empty response",
			pods:                 []v1.Pod{properTestPod},
			containerRequests:    nil,
			expectedContainerLen: 0,
		},
		{
			name:                 "Proper number of containers with good devices",
			pods:                 []v1.Pod{properTestPod},
			containerRequests:    properPrefContainerRequests,
			expectDevices:        []string{"card0-0"},
			expectedContainerLen: 1,
		},
		{
			name:                 "Inconsistent devices vs. gas' annotated ones",
			pods:                 []v1.Pod{properTestPod},
			containerRequests:    outofRangePrefContainerRequests,
			expectDevices:        []string{},
			expectedContainerLen: 1,
		},
		{
			name:                 "Preferred allocation is with must have device ids",
			pods:                 []v1.Pod{properTestPodMultiGpu},
			containerRequests:    mustHaveContainerRequests,
			expectDevices:        []string{"card0-2", "card1-0"},
			expectedContainerLen: 1,
		},
		{
			name:                 "Duplicate card requesting pod",
			pods:                 []v1.Pod{properTestPodMultiGpu2},
			containerRequests:    properPrefContainerRequests3,
			expectDevices:        []string{"card0-0", "card1-0", "card0-1"},
			expectedContainerLen: 1,
		},
		{
			name:                 "Allocation size is larger than cards assigned",
			pods:                 []v1.Pod{properTestPodMultiGpu},
			containerRequests:    properPrefContainerRequests3,
			expectDevices:        []string{"card0-0", "card1-0"},
			expectedContainerLen: 1,
		},
		{
			name:                 "Monitoring pod is being allocated",
			pods:                 []v1.Pod{monitoringPod},
			containerRequests:    allContainerRequests,
			expectDevices:        []string{},
			expectedContainerLen: 0,
		},
		{
			name:                 "Two pods with one without GPU",
			pods:                 []v1.Pod{properTestPod, gpuLessTestPod},
			containerRequests:    properPrefContainerRequests,
			expectDevices:        []string{"card0-0"},
			expectedContainerLen: 1,
		},
	}

	for _, tCase := range testCases {
		rm := newMockResourceManager(tCase.pods)
		resp, perr := rm.GetPreferredFractionalAllocation(&v1beta1.PreferredAllocationRequest{
			ContainerRequests: tCase.containerRequests,
		})

		if perr != nil {
			t.Errorf("test %v unexpected failure, err:%v", tCase.name, perr)
		}

		if perr == nil {
			// check response
			expectTruef(len(resp.ContainerResponses) == tCase.expectedContainerLen, t, tCase.name, "wrong number of container responses, expected 1")

			if len(tCase.expectDevices) > 0 {
				expectTruef(len(resp.ContainerResponses[0].DeviceIDs) == len(tCase.expectDevices), t, tCase.name,
					"wrong number of device ids: %d (%v)", len(resp.ContainerResponses[0].DeviceIDs), resp.ContainerResponses[0].DeviceIDs)

				for i, expecteDevice := range tCase.expectDevices {
					expectTruef(resp.ContainerResponses[0].DeviceIDs[i] == expecteDevice, t, tCase.name,
						"wrong device id selected: %s", resp.ContainerResponses[0].DeviceIDs[i])
				}
			}
		}
	}
}

func TestCreateFractionalResourceResponse(t *testing.T) {
	properTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{gasCardAnnotation: "card0"},
			Name:        "TestPod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("1"),
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
	}
	unAnnotatedTestPod := *properTestPod.DeepCopy()
	unAnnotatedTestPod.ObjectMeta.Annotations = nil
	properTestPod2 := *properTestPod.DeepCopy()
	properTestPod2.ObjectMeta.Name = "TestPod2"

	timeStampedProperTestPod := *properTestPod.DeepCopy()
	timeStampedProperTestPod.ObjectMeta.Annotations[gasTSAnnotation] = "2"

	timeStampedProperTestPod2 := *properTestPod2.DeepCopy()
	timeStampedProperTestPod2.ObjectMeta.Annotations[gasTSAnnotation] = "1"

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card0-0"}}}

	properPrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1"},
			AllocationSize: 1},
	}

	testCases := []testCase{
		{
			name:                  "Wrong number of container requests should fail",
			pods:                  []v1.Pod{properTestPod},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         false,
			containerRequests:     []*v1beta1.ContainerAllocateRequest{},
			expectErr:             true,
		},
		{
			name:                  "Request for monitor resource should fail",
			pods:                  []v1.Pod{properTestPod},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         true,
			containerRequests:     []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"all"}}},
			expectErr:             true,
		},
		{
			name:                  "Zero pending pods should fail",
			pods:                  []v1.Pod{},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         true,
			containerRequests:     properContainerRequests,
			expectErr:             true,
		},
		{
			name:                  "Single pending pod without annotations should fail",
			pods:                  []v1.Pod{unAnnotatedTestPod},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         true,
			containerRequests:     properContainerRequests,
			expectErr:             true,
		},
		{
			name:                  "Single pending pod should succeed",
			pods:                  []v1.Pod{properTestPod},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         true,
			containerRequests:     properContainerRequests,
			expectErr:             false,
		},
		{
			name:                  "Two pending pods without timestamps should fail",
			pods:                  []v1.Pod{properTestPod, properTestPod2},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         true,
			containerRequests:     properContainerRequests,
			expectErr:             true,
		},
		{
			name:                  "Two pending pods with timestamps should reduce to one candidate and succeed",
			pods:                  []v1.Pod{timeStampedProperTestPod, timeStampedProperTestPod2},
			prefContainerRequests: properPrefContainerRequests,
			prefExpectErr:         true,
			containerRequests:     properContainerRequests,
			expectErr:             false,
		},
	}

	for _, tCase := range testCases {
		rm := newMockResourceManager(tCase.pods)
		rm.SetTileCountPerCard(uint64(1))

		_, perr := rm.GetPreferredFractionalAllocation(&v1beta1.PreferredAllocationRequest{
			ContainerRequests: tCase.prefContainerRequests,
		})

		if (perr != nil) && !tCase.prefExpectErr {
			t.Errorf("test %v unexpected failure, err:%v", tCase.name, perr)
		}

		resp, err := rm.CreateFractionalResourceResponse(&v1beta1.AllocateRequest{
			ContainerRequests: tCase.containerRequests,
		})

		if (err != nil) && !tCase.expectErr {
			t.Errorf("test %v unexpected failure, err:%v", tCase.name, err)
		}

		if err == nil {
			if tCase.expectErr {
				t.Errorf("test %v unexpected success", tCase.name)
			} else {
				// check response
				expectTruef(len(resp.ContainerResponses) == 1, t, tCase.name, "wrong number of container responses, expected 1")
				expectTruef(len(resp.ContainerResponses[0].Envs) == 1, t, tCase.name, "wrong number of env variables in container response, expected 1")
				expectTruef(resp.ContainerResponses[0].Envs["more"] == "coverage", t, tCase.name, "env not set for container response")
				expectTruef(len(resp.ContainerResponses[0].Devices) == 1, t, tCase.name, "wrong number of devices, expected 1")
				expectTruef(resp.ContainerResponses[0].Devices[0].HostPath == "hostpath", t, tCase.name, "HostPath not set for device")
				expectTruef(resp.ContainerResponses[0].Devices[0].ContainerPath == "containerpath", t, tCase.name, "ContainerPath not set for device")
				expectTruef(resp.ContainerResponses[0].Devices[0].Permissions == "rw", t, tCase.name, "permissions not set for device")
			}
		}
	}
}

func TestCreateFractionalResourceResponseWithOneCardTwoTiles(t *testing.T) {
	properTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				gasCardAnnotation: "card0",
				gasTileAnnotation: "card0:gt0+gt1"},
			Name: "TestPod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("1"),
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  levelzeroHierarchyEnvVar,
							Value: hierarchyModeComposite,
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
	}

	properPrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1"},
			AllocationSize: 1},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card0-0"}}}

	tCase := testCase{
		name:                  "Single pending pod with two tiles should succeed",
		pods:                  []v1.Pod{properTestPod},
		prefContainerRequests: properPrefContainerRequests,
		prefExpectErr:         false,
		containerRequests:     properContainerRequests,
		expectErr:             false,
	}

	rm := newMockResourceManager(tCase.pods)
	rm.SetTileCountPerCard(uint64(2))

	_, perr := rm.GetPreferredFractionalAllocation(&v1beta1.PreferredAllocationRequest{
		ContainerRequests: tCase.prefContainerRequests,
	})

	if (perr != nil) && !tCase.prefExpectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, perr)
	}

	resp, err := rm.CreateFractionalResourceResponse(&v1beta1.AllocateRequest{
		ContainerRequests: tCase.containerRequests,
	})

	if (err != nil) && !tCase.expectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, err)
	}

	// check response
	expectTruef(len(resp.ContainerResponses) == 1, t, tCase.name, "wrong number of container responses, expected 1")
	expectTruef(len(resp.ContainerResponses[0].Envs) == 2, t, tCase.name, "wrong number of env variables in container response, expected 2")
	expectTruef(resp.ContainerResponses[0].Envs[LevelzeroAffinityMaskEnvVar] != "", t, tCase.name, "l0 tile mask not set")
	expectTruef(resp.ContainerResponses[0].Envs[LevelzeroAffinityMaskEnvVar] == "0.0,0.1", t, tCase.name, "l0 affinity mask is incorrect")
	expectTruef(len(resp.ContainerResponses[0].Devices) == 1, t, tCase.name, "wrong number of devices, expected 1")
}

func TestCreateFractionalResourceResponseWithTwoCardsOneTile(t *testing.T) {
	properTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				gasCardAnnotation: "card1,card2",
				gasTileAnnotation: "card1:gt3,card2:gt4"},
			Name: "TestPod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("2"),
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  levelzeroHierarchyEnvVar,
							Value: hierarchyModeComposite,
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
	}

	properPrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1"},
			AllocationSize: 1},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card1-0", "card2-0"}}}

	tCase := testCase{
		name:                  "Single pending pod with two cards and one tile each should succeed",
		pods:                  []v1.Pod{properTestPod},
		prefContainerRequests: properPrefContainerRequests,
		prefExpectErr:         false,
		containerRequests:     properContainerRequests,
		expectErr:             false,
	}

	rm := newMockResourceManager(tCase.pods)
	rm.SetTileCountPerCard(uint64(5))

	_, perr := rm.GetPreferredFractionalAllocation(&v1beta1.PreferredAllocationRequest{
		ContainerRequests: tCase.prefContainerRequests,
	})

	if (perr != nil) && !tCase.prefExpectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, perr)
	}

	resp, err := rm.CreateFractionalResourceResponse(&v1beta1.AllocateRequest{
		ContainerRequests: tCase.containerRequests,
	})

	if (err != nil) && !tCase.expectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, err)
	}

	if err == nil {
		if tCase.expectErr {
			t.Errorf("test %v unexpected success", tCase.name)
		} else {
			// check response
			expectTruef(len(resp.ContainerResponses) == 1, t, tCase.name, "wrong number of container responses, expected 1")
			expectTruef(resp.ContainerResponses[0].Envs[LevelzeroAffinityMaskEnvVar] != "", t, tCase.name, "l0 tile mask not set")
			expectTruef(resp.ContainerResponses[0].Envs[LevelzeroAffinityMaskEnvVar] == "0.3,1.4", t, tCase.name, "l0 affinity mask is incorrect: ")
			expectTruef(len(resp.ContainerResponses[0].Devices) == 2, t, tCase.name, "wrong number of devices, expected 2")
		}
	}
}

func TestCreateFractionalResourceResponseWithThreeCardsTwoTiles(t *testing.T) {
	properTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				gasCardAnnotation: "card0,card1,card2",
				gasTileAnnotation: "card0:gt0+gt1,card1:gt2+gt3,card2:gt3+gt4"},
			Name: "TestPod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("3"),
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  levelzeroHierarchyEnvVar,
							Value: hierarchyModeComposite,
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
	}

	properPrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1", "card2-0", "card2-1"},
			AllocationSize: 1},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card0-0", "card1-0", "card2-0"}}}

	tCase := testCase{
		name:                  "Single pending pod with three cards and two tiles each should succeed",
		pods:                  []v1.Pod{properTestPod},
		prefContainerRequests: properPrefContainerRequests,
		prefExpectErr:         false,
		containerRequests:     properContainerRequests,
		expectErr:             false,
	}

	rm := newMockResourceManager(tCase.pods)
	rm.SetTileCountPerCard(uint64(5))

	_, perr := rm.GetPreferredFractionalAllocation(&v1beta1.PreferredAllocationRequest{
		ContainerRequests: tCase.prefContainerRequests,
	})

	if (perr != nil) && !tCase.prefExpectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, perr)
	}

	resp, err := rm.CreateFractionalResourceResponse(&v1beta1.AllocateRequest{
		ContainerRequests: tCase.containerRequests,
	})

	if (err != nil) && !tCase.expectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, err)
	}

	if err == nil {
		if tCase.expectErr {
			t.Errorf("test %v unexpected success", tCase.name)
		} else {
			// check response
			expectTruef(len(resp.ContainerResponses) == 1, t, tCase.name, "wrong number of container responses, expected 1")
			expectTruef(resp.ContainerResponses[0].Envs[LevelzeroAffinityMaskEnvVar] != "", t, tCase.name, "l0 tile mask not set")
			expectTruef(resp.ContainerResponses[0].Envs[LevelzeroAffinityMaskEnvVar] == "0.0,0.1,1.2,1.3,2.3,2.4", t, tCase.name, "l0 affinity mask is incorrect: ")
			expectTruef(len(resp.ContainerResponses[0].Devices) == 3, t, tCase.name, "wrong number of devices, expected 3")
		}
	}
}

func TestCreateFractionalResourceResponseWithMultipleContainersTileEach(t *testing.T) {
	properTestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				gasCardAnnotation: "card1|card2",
				gasTileAnnotation: "card1:gt1|card2:gt0"},
			Name: "TestPod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("1"),
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  levelzeroHierarchyEnvVar,
							Value: hierarchyModeComposite,
						},
					},
				},
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("1"),
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  levelzeroHierarchyEnvVar,
							Value: hierarchyModeComposite,
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
		},
	}

	properPrefContainerRequests := []*v1beta1.ContainerPreferredAllocationRequest{
		{AvailableDeviceIDs: []string{"card0-0", "card0-1", "card1-0", "card1-1", "card2-0", "card2-1"},
			AllocationSize: 1},
	}
	_ = properPrefContainerRequests

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{
		{DevicesIDs: []string{"card1-0"}},
		{DevicesIDs: []string{"card2-0"}},
	}

	tCase := testCase{
		name:                  "Single pending pod with two containers with one tile each should FAIL",
		pods:                  []v1.Pod{properTestPod},
		prefContainerRequests: properPrefContainerRequests,
		prefExpectErr:         false,
		containerRequests:     properContainerRequests,
		expectErr:             true,
	}

	rm := newMockResourceManager(tCase.pods)
	rm.SetTileCountPerCard(uint64(2))

	_, perr := rm.GetPreferredFractionalAllocation(&v1beta1.PreferredAllocationRequest{
		ContainerRequests: properPrefContainerRequests,
	})

	if (perr != nil) && !tCase.prefExpectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, perr)
	}

	_, err := rm.CreateFractionalResourceResponse(&v1beta1.AllocateRequest{
		ContainerRequests: tCase.containerRequests,
	})

	if (err != nil) && !tCase.expectErr {
		t.Errorf("test %v unexpected failure, err:%v", tCase.name, err)
	}

	if err == nil {
		if tCase.expectErr {
			t.Errorf("test %v unexpected success", tCase.name)
		}
	}
}

func TestTileAnnotationParsing(t *testing.T) {
	type parseTest struct {
		line         string
		result       string
		hierarchys   []string
		index        int
		tilesPerCard int
	}

	parseTests := []parseTest{
		{
			line:         "card1:gt1",
			index:        0,
			result:       "0.1",
			hierarchys:   []string{"COMPOSITE"},
			tilesPerCard: 2,
		},
		{
			line:         "card1:gt0",
			index:        0,
			result:       "",
			hierarchys:   []string{"COMPOSITE"},
			tilesPerCard: 1,
		},
		{
			line:         "card1:gt1+gt2",
			index:        0,
			result:       "0.1,0.2",
			hierarchys:   []string{"COMPOSITE"},
			tilesPerCard: 3,
		},
		// Invalid hierarchy defaults to FLAT
		{
			line:         "card1:gt1+gt2,card2:gt0",
			index:        0,
			result:       "1,2,3",
			hierarchys:   []string{"FOOBAR"},
			tilesPerCard: 3,
		},
		{
			line:         "card1:gt1+gt2,card2:gt0",
			index:        0,
			result:       "1,2,3",
			hierarchys:   []string{"FLAT"},
			tilesPerCard: 3,
		},
		{
			line:         "||card1:gt1+gt2,card2:gt0",
			index:        0,
			result:       "1,2,3",
			hierarchys:   []string{"", "", "FLAT"},
			tilesPerCard: 3,
		},
		{
			line:         "||||card1:gt3,card5:gt1",
			index:        0,
			result:       "3,9",
			hierarchys:   []string{"", "", "", "", "FLAT"},
			tilesPerCard: 8,
		},
		{
			line:         "card1:gt1+gt2,card2:gt1",
			index:        0,
			result:       "1,2,4",
			hierarchys:   []string{"COMBINED"},
			tilesPerCard: 3,
		},
		{
			line:         "card1:gt1,card2:gt1",
			index:        0,
			result:       "1,3",
			hierarchys:   []string{"COMBINED"},
			tilesPerCard: 2,
		},
		{
			line:   "card1:gt1",
			index:  1,
			result: "",
		},
		{
			line:         "card1:gt1|card2:gt4",
			index:        1,
			result:       "4",
			tilesPerCard: 5,
		},
		{
			line:         "card1:gt1|card2:gt4,card3:gt2",
			index:        1,
			result:       "0.4,1.2",
			hierarchys:   []string{"COMPOSITE", "COMPOSITE"},
			tilesPerCard: 5,
		},
		{
			line:         "card1:gt1|card2:gt4,card3:gt2|card5:gt0",
			index:        2,
			result:       "0.0",
			hierarchys:   []string{"COMPOSITE", "COMPOSITE", "COMPOSITE"},
			tilesPerCard: 5,
		},
		{
			line:         "||card5:gt0,card6:gt4||",
			index:        0,
			result:       "0.0,1.4",
			hierarchys:   []string{"", "", "COMPOSITE"},
			tilesPerCard: 5,
		},
		{
			line:   "||card5:gt0,card6:gt4||",
			index:  1,
			result: "",
		},
		{
			line:   "||card5:gt0,card:6:gt4||",
			index:  0,
			result: "",
		},
		{
			line:   "||card5:gt0,card6:gt+gt+gt||",
			index:  0,
			result: "",
		},
		{
			line:   "card1:gtX",
			index:  0,
			result: "",
		},
		{
			line:   "card1:64X",
			index:  0,
			result: "",
		},
		{
			line:   "|",
			index:  0,
			result: "",
		},
		{
			line:         "card1:gt1||card2:gt4,card3:gt2",
			index:        1,
			result:       "0.4,1.2",
			hierarchys:   []string{"", "", "COMPOSITE"},
			tilesPerCard: 6,
		},
		{
			line:         "|||card2:gt7",
			index:        0,
			result:       "0.7",
			hierarchys:   []string{"", "", "", "COMPOSITE"},
			tilesPerCard: 8,
		},
		{
			line:   "card5",
			index:  0,
			result: "",
		},
	}

	for testIndex, pt := range parseTests {
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					gasTileAnnotation: pt.line},
			},
		}

		if pt.hierarchys != nil {
			// Create enough containers
			pod.Spec.Containers = make([]v1.Container, 10)

			for i := range pod.Spec.Containers {
				if i < len(pt.hierarchys) {
					pod.Spec.Containers[i].Env = []v1.EnvVar{
						{
							Name:  levelzeroHierarchyEnvVar,
							Value: pt.hierarchys[i],
						},
					}
				}
			}
		}

		ret := containerTileAffinityMask(&pod, pt.index, max(1, pt.tilesPerCard))

		expectTruef(ret == pt.result, t, pt.line, "resulting mask is wrong (test index=%d). correct: %v, got: %v", testIndex, pt.result, ret)
	}
}

func TestSelectDeviceIDsForContainerDoubleCards(t *testing.T) {
	cards := []string{
		"card0",
		"card1",
	}

	deviceIds := []string{
		"card0-0",
		"card0-1",
		"card0-2",
		"card1-0",
		"card1-1",
		"card1-2",
	}

	selected := selectDeviceIDsForContainer(2, cards, deviceIds, []string{})
	if len(selected) != 2 {
		t.Errorf("Not the correct amount of devices were selected")
	}

	correctDevices := map[string]bool{
		"card0-0": false,
		"card1-0": false,
	}

	for _, selected := range selected {
		correctDevices[selected] = true
	}

	for dev, used := range correctDevices {
		if !used {
			t.Errorf("correct device was not used: %s", dev)
		}
	}
}

func TestSelectDeviceIDsForContainerSingleCard(t *testing.T) {
	cards := []string{
		"card2",
	}

	deviceIds := []string{
		"card0-0",
		"card0-1",
		"card1-0",
		"card2-0",
		"card2-1",
	}

	selected := selectDeviceIDsForContainer(1, cards, deviceIds, []string{})
	if len(selected) != 1 {
		t.Errorf("Not the correct amount of devices were selected")
	}

	if selected[0] != "card2-0" {
		t.Errorf("First selection is wrong: %s vs %s", selected[0], "card2-0")
	}
}

func expectTruef(predicate bool, t *testing.T, testName, format string, args ...interface{}) {
	if !predicate {
		t.Helper()
		t.Errorf(fmt.Sprintf("in test %q ", testName)+format, args...)
	}
}
