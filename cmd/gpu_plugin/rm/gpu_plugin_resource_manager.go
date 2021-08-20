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
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	"k8s.io/kubernetes/pkg/kubelet/apis/podresources"
)

const (
	gasTSAnnotation   = "gas-ts"
	gasCardAnnotation = "gas-container-cards"

	grpcAddress    = "unix:///var/lib/kubelet/pod-resources/kubelet.sock"
	grpcBufferSize = 4 * 1024 * 1024
	grpcTimeout    = 5 * time.Second

	retryTimeout = 1 * time.Second
)

// Errors.
type retryErr struct{}
type zeroPendingErr struct{}

func (e *retryErr) Error() string {
	return "things didn't work out, but perhaps a retry will help"
}
func (e *zeroPendingErr) Error() string {
	return "there are no pending pods anymore in this node"
}

type podCandidate struct {
	pod                 *v1.Pod
	name                string
	allocatedContainers int
	allocationTargetNum int
}

type DeviceInfo struct {
	nodes  []pluginapi.DeviceSpec
	mounts []pluginapi.Mount
	envs   map[string]string
}

type getClientFunc func(string, time.Duration, int) (podresourcesv1.PodResourcesListerClient, *grpc.ClientConn, error)

type ResourceManager interface {
	ReallocateWithFractionalResources(*pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error)
	SetDevInfos(DeviceInfoMap)
}

type resourceManager struct {
	mutex            sync.RWMutex // for devTree updates during scan
	deviceInfos      DeviceInfoMap
	nodeName         string
	clientset        kubernetes.Interface
	skipID           string
	fullResourceName string
	prGetClientFunc  getClientFunc
}

func NewDeviceInfo(nodes []pluginapi.DeviceSpec, mounts []pluginapi.Mount, envs map[string]string) *DeviceInfo {
	return &DeviceInfo{
		nodes:  nodes,
		mounts: mounts,
		envs:   envs,
	}
}

type DeviceInfoMap map[string]*DeviceInfo

func NewDeviceInfoMap() DeviceInfoMap {
	return DeviceInfoMap{}
}

// NewResourceManager creates a new resource manager.
func NewResourceManager(skipID, fullResourceName string) (ResourceManager, error) {
	clientset, err := getClientset()

	if err != nil {
		return nil, errors.Wrap(err, "couldn't get clientset")
	}

	rm := resourceManager{
		nodeName:         os.Getenv("NODE_NAME"),
		clientset:        clientset,
		skipID:           skipID,
		fullResourceName: fullResourceName,
		prGetClientFunc:  podresources.GetV1Client,
	}

	klog.Info("GPU device plugin resource manager enabled")

	return &rm, nil
}

// ReallocateWithFractionalResources runs the fractional resource logic.
// This intentionally only logs errors and returns with the UseDefaultMethodError,
// in case any errors are hit. This is to avoid clusters filling up with unexpected admission errors.
func (rm *resourceManager) ReallocateWithFractionalResources(request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	if !isInputOk(request, rm.skipID) {
		// it is better to leave allocated gpu devices as is and return
		return nil, &dpapi.UseDefaultMethodError{}
	}

	podCandidate, err := rm.findAllocationPodCandidate()
	if _, ok := err.(*retryErr); ok {
		klog.Warning("retrying POD resolving after sleeping")
		time.Sleep(retryTimeout)
		podCandidate, err = rm.findAllocationPodCandidate()
	}

	if err != nil {
		if _, ok := err.(*zeroPendingErr); !ok {
			klog.Error("allocation candidate not found, perhaps the GPU scheduler extender is not called, err:", err)
		}
		// it is better to leave allocated gpu devices as is and return
		return nil, &dpapi.UseDefaultMethodError{}
	}

	pod := podCandidate.pod
	cards := containerCards(pod, podCandidate.allocatedContainers)

	return rm.createAllocateResponse(cards)
}

func isInputOk(rqt *pluginapi.AllocateRequest, skipID string) bool {
	// so far kubelet calls allocate for each container separately. If that changes, we need to refine our logic.
	if len(rqt.ContainerRequests) != 1 {
		klog.Warning("multi-container allocation request not supported")
		return false
	}

	crqt := rqt.ContainerRequests[0]
	for _, id := range crqt.DevicesIDs {
		if id == skipID {
			return false // intentionally not printing anything, this request is skipped
		}
	}

	return true
}

// findAllocationPodCandidate tries to find the best allocation candidate pod, which must be:
//  -pending for this node
//  -using GPU resources in its spec
//  -is found via grpc service with unallocated GPU devices
// returns:
//  -the candidate pod struct pointer and no error, or
//  -errRetry if unsuccessful, but there is perhaps hope of trying again with better luck
//  -errZeroPending if no pending pods exist anymore (which is fine)
//  -any grpc communication errors
func (rm *resourceManager) findAllocationPodCandidate() (*podCandidate, error) {
	// get map of pending pods for this node
	pendingPods, err := rm.getNodePendingGPUPods()
	if err != nil {
		return nil, err
	}

	candidates, err := rm.findAllocationPodCandidates(pendingPods)
	if err != nil {
		return nil, err
	}

	numCandidates := len(candidates)
	switch numCandidates {
	case 0:
		// fine, this typically happens when deployment is deleted before PODs start
		klog.V(4).Info("zero pending pods")
		return nil, &zeroPendingErr{}
	case 1:
		// perfect, only one option
		klog.V(4).Info("only one pending pod")
		if _, ok := candidates[0].pod.Annotations[gasCardAnnotation]; !ok {
			klog.Warningf("Pending POD annotations from scheduler not yet visible for pod %q", candidates[0].pod.Name)
			return nil, &retryErr{}
		}
		return &candidates[0], nil
	default: // > 1 candidates, not good, need to pick the best
		// look for scheduler timestamps and sort by them
		klog.V(4).Infof("%v pods pending, picking oldest", numCandidates)
		timestampedCandidates := []podCandidate{}
		for _, candidate := range candidates {
			if _, ok := pendingPods[candidate.name].Annotations[gasTSAnnotation]; ok {
				timestampedCandidates = append(timestampedCandidates, candidate)
			}
		}
		sort.Slice(timestampedCandidates,
			func(i, j int) bool {
				return pendingPods[timestampedCandidates[i].name].Annotations[gasTSAnnotation] <
					pendingPods[timestampedCandidates[j].name].Annotations[gasTSAnnotation]
			})
		if len(timestampedCandidates) == 0 {
			klog.Warning("Pending POD annotations from scheduler not yet visible")
			return nil, &retryErr{}
		}
		return &timestampedCandidates[0], nil
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=list

// getNodePendingGPUPods returns a map of pod names -> pods that are pending and use the gpu.
func (rm *resourceManager) getNodePendingGPUPods() (map[string]*v1.Pod, error) {
	selector, err := fields.ParseSelector("spec.nodeName=" + rm.nodeName +
		",status.phase=" + string(v1.PodPending))

	if err != nil {
		return nil, errors.Wrap(err, "unable to parse selector")
	}

	pendingPodList, err := rm.clientset.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		FieldSelector: selector.String(),
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to list pods")
	}

	// make a map ouf of the list, accept only GPU-using pods
	pendingPods := make(map[string]*v1.Pod)
	for i := range pendingPodList.Items {
		pod := &pendingPodList.Items[i]

		if numGPUUsingContainers(pod, rm.fullResourceName) > 0 {
			pendingPods[pod.Name] = pod
		}
	}

	return pendingPods, nil
}

// findAllocationPodCandidates returns a slice of all potential allocation candidate pods.
// This goes through the PODs listed in the podresources grpc service and finds those among pending
// pods which don't have all GPU devices allocated.
func (rm *resourceManager) findAllocationPodCandidates(pendingPods map[string]*v1.Pod) ([]podCandidate, error) {
	resListerClient, clientConn, err := rm.prGetClientFunc(grpcAddress, grpcTimeout, grpcBufferSize)
	if err != nil {
		return nil, errors.Wrap(err, "Could not get a grpc client for reading plugin resources")
	}

	defer clientConn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
	defer cancel()

	resp, err := resListerClient.List(ctx, &podresourcesv1.ListPodResourcesRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "Could not read plugin resources via grpc")
	}

	candidates := []podCandidate{}
	for _, podRes := range resp.PodResources {
		// count allocated gpu-using containers
		numContainersAllocated := 0
		for _, cont := range podRes.Containers {
			for _, dev := range cont.Devices {
				if dev.ResourceName == rm.fullResourceName {
					numContainersAllocated++
					break
				}
			}
		}

		if pod, pending := pendingPods[podRes.Name]; pending {
			allocationTargetNum := numGPUUsingContainers(pod, rm.fullResourceName)
			if numContainersAllocated < allocationTargetNum {
				candidate := podCandidate{
					pod:                 pod,
					name:                podRes.Name,
					allocatedContainers: numContainersAllocated,
					allocationTargetNum: allocationTargetNum,
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates, nil
}

func (rm *resourceManager) SetDevInfos(deviceInfos DeviceInfoMap) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.deviceInfos = deviceInfos
}

func (rm *resourceManager) createAllocateResponse(cards []string) (*pluginapi.AllocateResponse, error) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	allocateResponse := pluginapi.AllocateResponse{}
	cresp := pluginapi.ContainerAllocateResponse{}

	for _, card := range cards {
		newDeviceID := card + "-0"

		dev, ok := rm.deviceInfos[newDeviceID]
		if !ok {
			klog.Warningf("No device info for %q, using default allocation method devices", newDeviceID)
			return nil, &dpapi.UseDefaultMethodError{}
		}

		// add new devices
		nodes := dev.nodes
		for i := range nodes {
			cresp.Devices = append(cresp.Devices, &nodes[i])
		}

		// add new mounts
		mounts := dev.mounts
		for i := range mounts {
			cresp.Mounts = append(cresp.Mounts, &mounts[i])
		}

		for key, value := range dev.envs {
			if cresp.Envs == nil {
				cresp.Envs = make(map[string]string)
			}
			cresp.Envs[key] = value
		}
	}

	allocateResponse.ContainerResponses = append(allocateResponse.ContainerResponses, &cresp)

	return &allocateResponse, nil
}

func numGPUUsingContainers(pod *v1.Pod, fullResourceName string) int {
	num := 0
	for _, container := range pod.Spec.Containers {
		for reqName, quantity := range container.Resources.Requests {
			resourceName := reqName.String()
			if resourceName == fullResourceName {
				value, _ := quantity.AsInt64()
				if value > 0 {
					num++
					break
				}
			}
		}
	}
	return num
}

// containerCards returns the cards to use for a single container.
// gpuUsingContainerIndex 0 == first gpu-using container in the pod.
func containerCards(pod *v1.Pod, gpuUsingContainerIndex int) []string {
	fullAnnotation := pod.Annotations[gasCardAnnotation]
	cardLists := strings.Split(fullAnnotation, "|")
	klog.V(3).Infof("%s:%v", fullAnnotation, cardLists)

	i := 0
	for _, cardList := range cardLists {
		cards := strings.Split(cardList, ",")
		if len(cards) > 0 && len(cardList) > 0 {
			if gpuUsingContainerIndex == i {
				klog.V(3).Infof("Cards for container nr %v in pod %v are %v", gpuUsingContainerIndex, pod.Name, cards)
				return cards
			}
			i++
		}
	}
	klog.Warningf("couldn't find cards for gpu using container index %v", gpuUsingContainerIndex)
	return nil
}

func getClientset() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
