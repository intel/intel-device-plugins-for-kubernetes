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

package rm

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

// mockClient implements enough of k8s API for the resource manager tests.
type mockClient struct {
	fake.Clientset
	mockCoreV1
}

func (m *mockClient) CoreV1() corev1.CoreV1Interface {
	return m
}

type mockCoreV1 struct {
	fakecorev1.FakeCoreV1
	mockPods
}

func (m *mockCoreV1) Pods(namespace string) corev1.PodInterface {
	return m
}

type mockPods struct {
	fakecorev1.FakePods
	pods []v1.Pod
}

func (m *mockPods) List(ctx context.Context, opts metav1.ListOptions) (*v1.PodList, error) {
	return &v1.PodList{
		Items: m.pods,
	}, nil
}

type mockPodResources struct {
	pods []v1.Pod
}

func (w *mockPodResources) List(ctx context.Context,
	in *podresourcesv1.ListPodResourcesRequest,
	opts ...grpc.CallOption) (*podresourcesv1.ListPodResourcesResponse, error) {
	resp := podresourcesv1.ListPodResourcesResponse{}
	for _, pod := range w.pods {
		resp.PodResources = append(resp.PodResources, &podresourcesv1.PodResources{
			Name: pod.ObjectMeta.Name, Containers: []*podresourcesv1.ContainerResources{{}},
		})
	}

	return &resp, nil
}
func (w *mockPodResources) GetAllocatableResources(ctx context.Context,
	in *podresourcesv1.AllocatableResourcesRequest,
	opts ...grpc.CallOption) (*podresourcesv1.AllocatableResourcesResponse, error) {
	return nil, nil
}

func newMockResourceManager(pods []v1.Pod) ResourceManager {
	//nolint: staticcheck
	client, err := grpc.Dial("", grpc.WithInsecure())
	if err != nil {
		os.Exit(1)
	}

	mc := &mockClient{}
	mc.mockCoreV1.mockPods.pods = pods
	rm := resourceManager{
		clientset: mc,
		nodeName:  "TestNode",
		prGetClientFunc: func(string, time.Duration, int) (podresourcesv1.PodResourcesListerClient, *grpc.ClientConn, error) {
			return &mockPodResources{pods: pods}, client, nil
		},
		skipID:           "all",
		fullResourceName: "gpu.intel.com/i915",
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

type testCase struct {
	name              string
	pods              []v1.Pod
	containerRequests []*v1beta1.ContainerAllocateRequest
	expectErr         bool
}

func TestNewResourceManager(t *testing.T) {
	// normal clientset is unavailable inside the unit tests
	_, err := NewResourceManager("foo", "bar")

	if err == nil {
		t.Errorf("unexpected success")
	}
}

func TestReallocateWithFractionalResources(t *testing.T) {
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

	testCases := []testCase{
		{
			name:              "Wrong number of container requests should fail",
			pods:              []v1.Pod{properTestPod},
			containerRequests: []*v1beta1.ContainerAllocateRequest{},
			expectErr:         true,
		},
		{
			name:              "Request for monitor resource should fail",
			pods:              []v1.Pod{properTestPod},
			containerRequests: []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"all"}}},
			expectErr:         true,
		},
		{
			name:              "Zero pending pods should fail",
			pods:              []v1.Pod{},
			containerRequests: properContainerRequests,
			expectErr:         true,
		},
		{
			name:              "Single pending pod without annotations should fail",
			pods:              []v1.Pod{unAnnotatedTestPod},
			containerRequests: properContainerRequests,
			expectErr:         true,
		},
		{
			name:              "Single pending pod should succeed",
			pods:              []v1.Pod{properTestPod},
			containerRequests: properContainerRequests,
			expectErr:         false,
		},
		{
			name:              "Two pending pods without timestamps should fail",
			pods:              []v1.Pod{properTestPod, properTestPod2},
			containerRequests: properContainerRequests,
			expectErr:         true,
		},
		{
			name:              "Two pending pods with timestamps should reduce to one candidate and succeed",
			pods:              []v1.Pod{timeStampedProperTestPod, timeStampedProperTestPod2},
			containerRequests: properContainerRequests,
			expectErr:         false,
		},
	}

	for _, tCase := range testCases {
		rm := newMockResourceManager(tCase.pods)
		resp, err := rm.ReallocateWithFractionalResources(&v1beta1.AllocateRequest{
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

func TestReallocateWithFractionalResourcesWithOneCardTwoTiles(t *testing.T) {
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
				},
			},
		},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card0-0"}}}

	tCase := testCase{
		name:              "Single pending pod with two tiles should succeed",
		pods:              []v1.Pod{properTestPod},
		containerRequests: properContainerRequests,
		expectErr:         false,
	}

	rm := newMockResourceManager(tCase.pods)
	resp, err := rm.ReallocateWithFractionalResources(&v1beta1.AllocateRequest{
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
			expectTruef(len(resp.ContainerResponses[0].Envs) == 2, t, tCase.name, "wrong number of env variables in container response, expected 2")
			expectTruef(resp.ContainerResponses[0].Envs[levelZeroAffinityMaskEnvVar] != "", t, tCase.name, "l0 tile mask not set")
			expectTruef(resp.ContainerResponses[0].Envs[levelZeroAffinityMaskEnvVar] == "0.0,0.1", t, tCase.name, "l0 affinity mask is incorrect")
			expectTruef(len(resp.ContainerResponses[0].Devices) == 1, t, tCase.name, "wrong number of devices, expected 1")
		}
	}
}

func TestReallocateWithFractionalResourcesWithTwoCardsOneTile(t *testing.T) {
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
				},
			},
		},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card1-0", "card2-0"}}}

	tCase := testCase{
		name:              "Single pending pod with two cards and one tile each should succeed",
		pods:              []v1.Pod{properTestPod},
		containerRequests: properContainerRequests,
		expectErr:         false,
	}

	rm := newMockResourceManager(tCase.pods)
	resp, err := rm.ReallocateWithFractionalResources(&v1beta1.AllocateRequest{
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
			expectTruef(resp.ContainerResponses[0].Envs[levelZeroAffinityMaskEnvVar] != "", t, tCase.name, "l0 tile mask not set")
			expectTruef(resp.ContainerResponses[0].Envs[levelZeroAffinityMaskEnvVar] == "0.3,1.4", t, tCase.name, "l0 affinity mask is incorrect: ")
			expectTruef(len(resp.ContainerResponses[0].Devices) == 2, t, tCase.name, "wrong number of devices, expected 2")
		}
	}
}

func TestReallocateWithFractionalResourcesWithThreeCardsTwoTiles(t *testing.T) {
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
				},
			},
		},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card0-0", "card1-0", "card2-0"}}}

	tCase := testCase{
		name:              "Single pending pod with three cards and two tiles each should succeed",
		pods:              []v1.Pod{properTestPod},
		containerRequests: properContainerRequests,
		expectErr:         false,
	}

	rm := newMockResourceManager(tCase.pods)
	resp, err := rm.ReallocateWithFractionalResources(&v1beta1.AllocateRequest{
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
			expectTruef(resp.ContainerResponses[0].Envs[levelZeroAffinityMaskEnvVar] != "", t, tCase.name, "l0 tile mask not set")
			expectTruef(resp.ContainerResponses[0].Envs[levelZeroAffinityMaskEnvVar] == "0.0,0.1,1.2,1.3,2.3,2.4", t, tCase.name, "l0 affinity mask is incorrect: ")
			expectTruef(len(resp.ContainerResponses[0].Devices) == 3, t, tCase.name, "wrong number of devices, expected 3")
		}
	}
}

func TestReallocateWithFractionalResourcesWithMultipleContainersTileEach(t *testing.T) {
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
				},
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"gpu.intel.com/i915": resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	properContainerRequests := []*v1beta1.ContainerAllocateRequest{{DevicesIDs: []string{"card1-0"}}, {DevicesIDs: []string{"card2-0"}}}

	tCase := testCase{
		name:              "Single pending pod with two containers with one tile each should fail",
		pods:              []v1.Pod{properTestPod},
		containerRequests: properContainerRequests,
		expectErr:         true,
	}

	rm := newMockResourceManager(tCase.pods)
	_, err := rm.ReallocateWithFractionalResources(&v1beta1.AllocateRequest{
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
		line   string
		index  int
		result string
	}

	parseTests := []parseTest{
		{
			line:   "card1:gt1",
			index:  0,
			result: "0.1",
		},
		{
			line:   "card1:gt1+gt2",
			index:  0,
			result: "0.1,0.2",
		},
		{
			line:   "card1:gt1+gt2,card2:gt0",
			index:  0,
			result: "0.1,0.2,1.0",
		},
		{
			line:   "card1:gt1",
			index:  1,
			result: "",
		},
		{
			line:   "card1:gt1|card2:gt4",
			index:  1,
			result: "0.4",
		},
		{
			line:   "card1:gt1|card2:gt4,card3:gt2",
			index:  1,
			result: "0.4,1.2",
		},
		{
			line:   "card1:gt1|card2:gt4,card3:gt2|card5:gt0",
			index:  2,
			result: "0.0",
		},
		{
			line:   "card1:gtX",
			index:  0,
			result: "",
		},
		{
			line:   "|",
			index:  0,
			result: "",
		},
		{
			line:   "card1:gt1||card2:gt4,card3:gt2",
			index:  1,
			result: "0.4,1.2",
		},
		{
			line:   "|||card2:gt7",
			index:  0,
			result: "0.7",
		},
		{
			line:   "card5",
			index:  0,
			result: "",
		},
	}

	for _, pt := range parseTests {
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					gasTileAnnotation: pt.line},
			},
		}

		ret := containerTileAffinityMask(&pod, pt.index)

		expectTruef(ret == pt.result, t, pt.line, "resulting mask is wrong. correct: %v, got: %v", pt.result, ret)
	}
}

func expectTruef(predicate bool, t *testing.T, testName, format string, args ...interface{}) {
	if !predicate {
		t.Helper()
		t.Errorf(fmt.Sprintf("in test %q ", testName)+format, args...)
	}
}
