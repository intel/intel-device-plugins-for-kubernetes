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
	"strconv"
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
	gasTileAnnotation = "gas-container-tiles"

	levelZeroAffinityMaskEnvVar = "ZE_AFFINITY_MASK"

	grpcAddress    = "unix:///var/lib/kubelet/pod-resources/kubelet.sock"
	grpcBufferSize = 4 * 1024 * 1024
	grpcTimeout    = 5 * time.Second
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
	pod                     *v1.Pod
	name                    string
	allocatedContainerCount int
	allocationTargetNum     int
}

// DeviceInfo is a subset of deviceplugin.DeviceInfo
// It's a lighter version of the full DeviceInfo as it is used
// to store fractional devices.
type DeviceInfo struct {
	envs   map[string]string
	nodes  []pluginapi.DeviceSpec
	mounts []pluginapi.Mount
}

type getClientFunc func(string, time.Duration, int) (podresourcesv1.PodResourcesListerClient, *grpc.ClientConn, error)

// ResourceManager interface for the fractional resource handling.
type ResourceManager interface {
	CreateFractionalResourceResponse(*pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error)
	GetPreferredFractionalAllocation(*pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error)
	SetDevInfos(DeviceInfoMap)
}

type ContainerAssignments struct {
	deviceIds map[string]bool
	tileEnv   string
}

type PodAssignmentDetails struct {
	containers []ContainerAssignments
}

type resourceManager struct {
	clientset        kubernetes.Interface
	deviceInfos      DeviceInfoMap
	prGetClientFunc  getClientFunc
	assignments      map[string]PodAssignmentDetails // pod name -> assignment details
	nodeName         string
	skipID           string
	fullResourceName string
	retryTimeout     time.Duration
	cleanupInterval  time.Duration
	mutex            sync.RWMutex // for devTree updates during scan
	cleanupMutex     sync.RWMutex // for assignment details during cleanup
}

// NewDeviceInfo creates a new DeviceInfo.
func NewDeviceInfo(nodes []pluginapi.DeviceSpec, mounts []pluginapi.Mount, envs map[string]string) *DeviceInfo {
	return &DeviceInfo{
		nodes:  nodes,
		mounts: mounts,
		envs:   envs,
	}
}

// DeviceInfoMap is a map of device infos. deviceId -> *DeviceInfo.
type DeviceInfoMap map[string]*DeviceInfo

// NewDeviceInfoMap creates a new DeviceInfoMap.
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
		assignments:      make(map[string]PodAssignmentDetails),
		retryTimeout:     1 * time.Second,
		cleanupInterval:  2 * time.Minute,
	}

	klog.Info("GPU device plugin resource manager enabled")

	go func() {
		ticker := time.NewTicker(rm.cleanupInterval)

		for range ticker.C {
			klog.V(4).Info("Running cleanup")

			// Gather both running and pending pods. It might happen that
			// cleanup is triggered between GetPreferredAllocation and Allocate
			// and it would remove the assignment data for the soon-to-be allocated pod
			running := rm.listPodsOnNodeWithState(string(v1.PodRunning))
			for podName, podItem := range rm.listPodsOnNodeWithState(string(v1.PodPending)) {
				running[podName] = podItem
			}

			func() {
				rm.cleanupMutex.Lock()
				defer rm.cleanupMutex.Unlock()

				for podName := range rm.assignments {
					if _, found := running[podName]; !found {
						klog.V(4).Info("Removing from assignments: ", podName)
						delete(rm.assignments, podName)
					}
				}
			}()

			klog.V(4).Info("Cleanup done")
		}
	}()

	return &rm, nil
}

// Generate a unique key for Pod.
func getPodKey(pod *v1.Pod) string {
	return pod.Namespace + "&" + pod.Name
}

// Generate a unique key for PodResources.
func getPodResourceKey(res *podresourcesv1.PodResources) string {
	return res.Namespace + "&" + res.Name
}

func (rm *resourceManager) listPodsOnNodeWithState(state string) map[string]*v1.Pod {
	pods := make(map[string]*v1.Pod)

	selector, err := fields.ParseSelector("spec.nodeName=" + rm.nodeName +
		",status.phase=" + state)

	if err != nil {
		return pods
	}

	podList, err := rm.clientset.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		FieldSelector: selector.String(),
	})

	if err != nil {
		return pods
	}

	for i := range podList.Items {
		key := getPodKey(&podList.Items[i])
		pods[key] = &podList.Items[i]
	}

	return pods
}

// CreateFractionalResourceResponse returns allocate response with the details
// assigned in GetPreferredFractionalAllocation
// This intentionally only logs errors and returns with the UseDefaultMethodError,
// in case any errors are hit. This is to avoid clusters filling up with unexpected admission errors.
func (rm *resourceManager) CreateFractionalResourceResponse(request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	if !isAllocateRequestOk(request, rm.skipID) {
		// it is better to leave allocated gpu devices as is and return
		return nil, &dpapi.UseDefaultMethodError{}
	}

	klog.V(4).Info("Proposed device ids: ", request.ContainerRequests[0].DevicesIDs)

	podCandidate, err := rm.findAllocationPodCandidate()
	if errors.Is(err, &retryErr{}) {
		klog.Warning("retrying POD resolving after sleeping")
		time.Sleep(rm.retryTimeout)

		podCandidate, err = rm.findAllocationPodCandidate()
	}

	if err != nil {
		if !errors.Is(err, &zeroPendingErr{}) {
			klog.Error("allocation candidate not found, perhaps the GPU scheduler extender is not called, err:", err)
		}
		// it is better to leave allocated gpu devices as is and return
		return nil, &dpapi.UseDefaultMethodError{}
	}

	pod := podCandidate.pod

	rm.cleanupMutex.Lock()

	assignment, found := rm.assignments[getPodKey(pod)]
	if !found {
		rm.cleanupMutex.Unlock()
		klog.Error("couldn't find allocation info from assignments:", getPodKey(pod))

		return nil, &dpapi.UseDefaultMethodError{}
	}

	containerIndex := podCandidate.allocatedContainerCount

	affinityMask := assignment.containers[containerIndex].tileEnv
	getPrefDevices := assignment.containers[containerIndex].deviceIds

	rm.cleanupMutex.Unlock()

	devIds := request.ContainerRequests[0].DevicesIDs

	// Check if all the preferred devices were also used
	if len(devIds) != len(getPrefDevices) {
		klog.Warningf("Allocate called with odd number of device IDs: %d vs %d", len(devIds), len(getPrefDevices))
	}

	for _, devID := range devIds {
		if _, found := getPrefDevices[devID]; !found {
			klog.Warningf("Not preferred device used in Allocate: %s (%v)", devID, getPrefDevices)
		}
	}

	klog.V(4).Info("Allocate affinity mask: ", affinityMask)
	klog.V(4).Info("Allocate device ids: ", devIds)

	return rm.createAllocateResponse(devIds, affinityMask)
}

func (rm *resourceManager) GetPreferredFractionalAllocation(request *pluginapi.PreferredAllocationRequest) (
	*pluginapi.PreferredAllocationResponse, error) {
	if !isPreferredAllocationRequestOk(request, rm.skipID) {
		// it is better to leave allocated gpu devices as is and return
		return &pluginapi.PreferredAllocationResponse{}, nil
	}

	klog.V(4).Info("GetPreferredAllocation request: ", request)

	podCandidate, err := rm.findAllocationPodCandidate()
	if errors.Is(err, &retryErr{}) {
		klog.Warning("retrying POD resolving after sleeping")
		time.Sleep(rm.retryTimeout)

		podCandidate, err = rm.findAllocationPodCandidate()
	}

	if err != nil {
		if !errors.Is(err, &zeroPendingErr{}) {
			klog.Error("allocation candidate not found, perhaps the GPU scheduler extender is not called, err:", err)
		}

		// Return empty response as returning an error causes
		// the pod to be labeled as UnexpectedAdmissionError
		return &pluginapi.PreferredAllocationResponse{}, nil
	}

	pod := podCandidate.pod
	containerIndex := podCandidate.allocatedContainerCount
	cards := containerCards(pod, containerIndex)
	affinityMask := containerTileAffinityMask(pod, containerIndex)
	podKey := getPodKey(pod)

	creq := request.ContainerRequests[0]

	klog.V(4).Info("Get preferred fractional allocation: ",
		podKey, creq.AllocationSize, creq.MustIncludeDeviceIDs, creq.AvailableDeviceIDs)

	deviceIds := selectDeviceIDsForContainer(
		int(creq.AllocationSize), cards, creq.AvailableDeviceIDs, creq.MustIncludeDeviceIDs)

	// Map container assignment details per pod name

	rm.cleanupMutex.Lock()

	assignments, found := rm.assignments[podKey]

	if !found {
		assignments.containers = make([]ContainerAssignments, podCandidate.allocationTargetNum)
	}

	assignments.containers[containerIndex].tileEnv = affinityMask
	// Store device ids so we can double check the ones in Allocate
	assignments.containers[containerIndex].deviceIds = make(map[string]bool)
	for _, devID := range deviceIds {
		assignments.containers[containerIndex].deviceIds[devID] = true
	}

	rm.assignments[podKey] = assignments

	rm.cleanupMutex.Unlock()

	klog.V(4).Info("Selected devices for container: ", deviceIds)

	response := pluginapi.PreferredAllocationResponse{
		ContainerResponses: []*pluginapi.ContainerPreferredAllocationResponse{
			{DeviceIDs: deviceIds},
		},
	}

	return &response, nil
}

// selectDeviceIDsForContainer selects suitable device ids from deviceIds and mustHaveDeviceIds
// the selection is guided by the cards list.
func selectDeviceIDsForContainer(requestedCount int, cards, deviceIds, mustHaveDeviceIds []string) []string {
	getBaseCard := func(deviceId string) string {
		return strings.Split(deviceId, "-")[0]
	}

	if requestedCount < len(cards) {
		klog.Warningf("Requested count is less than card count: %d vs %d.", requestedCount, len(cards))
		cards = cards[0:requestedCount]
	}

	if requestedCount > len(cards) {
		klog.Warningf("Requested count is higher than card count: %d vs %d.", requestedCount, len(cards))
	}

	// map of cardX -> device id list
	available := map[string][]string{}
	// Keep the last used index so we can pick the next one
	availableIndex := map[string]int{}

	// Place must have IDs first so they get used
	for _, devID := range mustHaveDeviceIds {
		baseCard := getBaseCard(devID)
		available[baseCard] = append(available[baseCard], devID)
	}

	for _, devID := range deviceIds {
		baseCard := getBaseCard(devID)
		available[baseCard] = append(available[baseCard], devID)
	}

	selected := []string{}

	for _, card := range cards {
		indexNow := availableIndex[card]

		availableDevices, found := available[card]
		if !found {
			klog.Warningf("card %s is not found from known devices: %v", card, available)
			continue
		}

		if indexNow < len(availableDevices) {
			selected = append(selected, availableDevices[indexNow])
			indexNow++
			availableIndex[card] = indexNow
		}
	}

	return selected
}

func isAllocateRequestOk(rqt *pluginapi.AllocateRequest, skipID string) bool {
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

func isPreferredAllocationRequestOk(rqt *pluginapi.PreferredAllocationRequest, skipID string) bool {
	// so far kubelet calls allocate for each container separately. If that changes, we need to refine our logic.
	if len(rqt.ContainerRequests) != 1 {
		klog.Warning("multi-container allocation request not supported")
		return false
	}

	crqt := rqt.ContainerRequests[0]
	for _, id := range crqt.AvailableDeviceIDs {
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

		// .name here refers to a namespace+name combination
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
	pendingPods := rm.listPodsOnNodeWithState(string(v1.PodPending))

	for podName, pod := range pendingPods {
		if numGPUUsingContainers(pod, rm.fullResourceName) == 0 {
			delete(pendingPods, podName)
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

		key := getPodResourceKey(podRes)

		if pod, pending := pendingPods[key]; pending {
			allocationTargetNum := numGPUUsingContainers(pod, rm.fullResourceName)
			if numContainersAllocated < allocationTargetNum {
				candidate := podCandidate{
					pod:                     pod,
					name:                    key,
					allocatedContainerCount: numContainersAllocated,
					allocationTargetNum:     allocationTargetNum,
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

func (rm *resourceManager) createAllocateResponse(deviceIds []string, tileAffinityMask string) (*pluginapi.AllocateResponse, error) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	allocateResponse := pluginapi.AllocateResponse{}
	cresp := pluginapi.ContainerAllocateResponse{}

	for _, devID := range deviceIds {
		dev, ok := rm.deviceInfos[devID]
		if !ok {
			klog.Warningf("No device info for %q, using default allocation method devices", devID)
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

	if tileAffinityMask != "" {
		if cresp.Envs == nil {
			cresp.Envs = make(map[string]string)
		}

		cresp.Envs[levelZeroAffinityMaskEnvVar] = tileAffinityMask
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
				klog.V(3).Infof("Cards for container nr %v in pod %v are %v", gpuUsingContainerIndex, getPodKey(pod), cards)
				return cards
			}
			i++
		}
	}

	klog.Warningf("couldn't find cards for gpu using container index %v", gpuUsingContainerIndex)

	return nil
}

func convertTileInfoToEnvMask(tileInfo string) string {
	cards := strings.Split(tileInfo, ",")

	tileIndices := make([]string, len(cards))

	for i, cardTileCombos := range cards {
		cardTileSplit := strings.Split(cardTileCombos, ":")
		if len(cardTileSplit) != 2 {
			klog.Warningf("invalid card tile combo string (%v)", cardTileCombos)
			return ""
		}

		tiles := strings.Split(cardTileSplit[1], "+")

		var combos []string

		for _, tile := range tiles {
			if !strings.HasPrefix(tile, "gt") {
				klog.Warningf("invalid tile syntax (%v)", tile)
				return ""
			}

			tileNoStr := strings.TrimPrefix(tile, "gt")
			tileNo, err := strconv.ParseInt(tileNoStr, 10, 16)

			if err != nil {
				klog.Warningf("invalid tile syntax (%v)", tile)
				return ""
			}

			levelZeroCardTileCombo :=
				strconv.FormatInt(int64(i), 10) + "." +
					strconv.FormatInt(tileNo, 10)
			combos = append(combos, levelZeroCardTileCombo)
		}

		tileIndices[i] = strings.Join(combos, ",")
	}

	return strings.Join(tileIndices, ",")
}

// containerTiles returns the tile indices to use for a single container.
// Indices should be passed to level zero env variable to guide execution
// gpuUsingContainerIndex 0 == first gpu-using container in the pod.
// annotation example:
// gas-container-tiles=card0:gt0+gt1,card1:gt0|card2:gt1+gt2||card0:gt3.
func containerTileAffinityMask(pod *v1.Pod, gpuUsingContainerIndex int) string {
	fullAnnotation := pod.Annotations[gasTileAnnotation]
	onlyDividers := strings.Count(fullAnnotation, "|") == len(fullAnnotation)

	if fullAnnotation == "" || onlyDividers {
		return ""
	}

	tileLists := strings.Split(fullAnnotation, "|")
	klog.Infof("%s:%v", fullAnnotation, tileLists)

	i := 0

	for _, containerTileInfo := range tileLists {
		if len(containerTileInfo) == 0 {
			continue
		}

		if i == gpuUsingContainerIndex {
			return convertTileInfoToEnvMask(containerTileInfo)
		}

		i++
	}

	klog.Warningf("couldn't find tile info for gpu using container index %v", gpuUsingContainerIndex)

	return ""
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
