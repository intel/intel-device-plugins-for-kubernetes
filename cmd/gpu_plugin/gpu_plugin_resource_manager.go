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

package main

import (
	"context"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
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
	// ResourcePrefix is the intel resource prefix.
	resourcePrefix = namespace + "/"

	gasTSAnnotation   = "gas-ts"
	gasCardAnnotation = "gas-container-cards"

	grpcAddress    = "unix:///var/lib/kubelet/pod-resources/kubelet.sock"
	grpcBufferSize = 4 * 1024 * 1024
	grpcTimeout    = 5 * time.Second

	retryTimeout = 1 * time.Second
)

// Errors.
var (
	errMultiContainer   = errors.New("multi-container allocation request")
	errMonitoringDevice = errors.New("monitoring device is ignored")
	errNoAnnotations    = errors.New("annotations missing from container allocate response")
	errRetry            = errors.New("things didn't work out, but perhaps a retry will help")
	errZeroPending      = errors.New("there are no pending pods anymore in this node")
	errBadPath          = errors.New("bad path, major/minor not resolved")
	errBadCardName      = errors.New("bad card name, major/minor not resolved")
)

// map of resources. name -> resource amount.
type resourceMap map[string]int64

type podCandidate struct {
	pod                 *v1.Pod
	name                string
	allocatedContainers int
	allocationTargetNum int
}

type resourceManager struct {
	mutex     sync.RWMutex // for devTree updates during scan
	devTree   dpapi.DeviceTree
	nodeName  string
	clientset *kubernetes.Clientset
}

func newResourceManager(enabled bool) *resourceManager {
	if !enabled {
		return nil
	}

	clientset := getClientset()

	if clientset == nil {
		klog.Error("no clientset available, resource manager not created")
		return nil
	}

	rm := resourceManager{
		nodeName:  os.Getenv("NODE_NAME"),
		clientset: clientset,
	}

	klog.Info("GPU device plugin resource manager enabled")

	return &rm
}

// reallocateWithFractionalResources runs the fractional resource logic.
func (rm *resourceManager) reallocateWithFractionalResources(allocateResponse *pluginapi.AllocateResponse) error {
	var err error
	if err = reallocateInputCheck(allocateResponse); err != nil {
		// it is better to give some gpu devices and return nil, than halt the whole cluster into unexpected admission errors
		return nil
	}

	podCandidate, err := rm.findAllocationPodCandidate()
	if err == errRetry {
		klog.Warning("retrying POD resolving after sleeping")
		time.Sleep(retryTimeout)
		podCandidate, err = rm.findAllocationPodCandidate()
	}

	if err != nil {
		if err != errZeroPending {
			klog.Error("allocation candidate not found, perhaps the GPU scheduler extender is not called, err:" +
				err.Error())
		}
		// it is better to give some gpu devices and return nil, than halt the whole cluster into unexpected admission errors
		return nil
	}

	pod := podCandidate.pod
	cards, cardResources := containerCardsAndResources(pod, podCandidate.allocatedContainers)
	logPodInfo(pod, cards, cardResources)

	cresp := allocateResponse.ContainerResponses[0]
	// select gpus for the allocation. the cards may differ from kubelet given deviceIDs!
	for _, cardName := range cards {
		rm.addDevicesAndMounts(cresp, cardName)
	}

	// add resource annotations to container response (for cgroups integration)
	_ = addResourceAnnotations(cresp, cardResources)

	return nil
}

func reallocateInputCheck(allocateResponse *pluginapi.AllocateResponse) error {
	// so far kubelet calls allocate for each container separately. If that changes, we need to refine our logic.
	if len(allocateResponse.ContainerResponses) != 1 {
		klog.Error(errMultiContainer.Error())
		return errMultiContainer
	}

	for _, cresp := range allocateResponse.ContainerResponses {
		if cresp.Annotations == nil {
			klog.Error(errNoAnnotations.Error())
			return errNoAnnotations
		}

		// if there is a device for the monitoring resource, skip the post allocation, the pod gets full access to all cards
		devices := cresp.Annotations["INTEL_PLUGIN_DEVICES"]
		if strings.Contains(devices, monitorID) {
			return errMonitoringDevice // intentionally not printing anything
		}
	}

	return nil
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
		klog.Error(err.Error())
		return nil, err
	}

	var candidate podCandidate
	candidates, err := findAllocationPodCandidates(pendingPods)
	if err != nil {
		klog.Error(err.Error())
		return nil, err
	}

	numCandidates := len(candidates)
	switch numCandidates {
	case 0:
		// fine, this typically happens when deployment is deleted before PODs start
		klog.V(4).Info("zero pending pods")
		return nil, errZeroPending
	case 1:
		// perfect, only one option
		klog.V(4).Info("only one pending pod")
		candidate = candidates[0]
		if _, ok := candidate.pod.Annotations[gasCardAnnotation]; !ok {
			klog.Warningf("Pending POD annotations from scheduler not yet visible for pod %v", candidate.pod.Name)
			return nil, errRetry
		}
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
			return nil, errRetry
		}
		candidate = timestampedCandidates[0]
	}

	candidate.pod = pendingPods[candidate.name]
	return &candidate, nil
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=list

// getNodePendingGPUPods returns a map of pod names -> pods that are pending and use the gpu.
func (rm *resourceManager) getNodePendingGPUPods() (map[string]*v1.Pod, error) {
	selector, err := fields.ParseSelector("spec.nodeName=" + rm.nodeName +
		",status.phase=" + string(v1.PodPending))

	if err != nil {
		klog.Error(err.Error())
		return nil, err
	}

	pendingPodList, err := rm.clientset.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: selector.String(),
	})

	if err != nil {
		klog.Error(err.Error())
		return nil, err
	}

	// make a map ouf of the list, accept only GPU-using pods
	pendingPods := make(map[string]*v1.Pod)
	for i := range pendingPodList.Items {
		pod := &pendingPodList.Items[i]

		if numGPUUsingContainers(pod) > 0 {
			pendingPods[pod.Name] = pod
		}
	}

	return pendingPods, nil
}

// findAllocationPodCandidates returns a slice of all potential allocation candidate pods.
// This goes through the PODs listed in the podresources grpc service and finds those among pending
// pods which don't have all GPU devices allocated.
func findAllocationPodCandidates(pendingPods map[string]*v1.Pod) (candidates []podCandidate, err error) {
	candidates = []podCandidate{}

	resListerClient, clientConn, err := podresources.GetV1Client(grpcAddress, grpcTimeout, grpcBufferSize)
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

	for _, podRes := range resp.PodResources {
		// count allocated gpu-using containers
		numContainersAllocated := 0
		for _, cont := range podRes.Containers {
			for _, dev := range cont.Devices {
				if dev.ResourceName == resourcePrefix+deviceType {
					numContainersAllocated++
					break
				}
			}
		}

		if pod, pending := pendingPods[podRes.Name]; pending {
			allocationTargetNum := numGPUUsingContainers(pod)
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

func (rm *resourceManager) setDevTree(devTree dpapi.DeviceTree) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.devTree = devTree
}

func (rm *resourceManager) addDevicesAndMounts(cresp *pluginapi.ContainerAllocateResponse, cardName string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if devices, ok := rm.devTree[deviceType]; ok {
		newDeviceID := cardName + "-0"

		newDevice, ok := devices[newDeviceID]
		if !ok {
			klog.Warningf("No device info for %v, using original devices and mounts", newDeviceID)
			return
		}

		// remove old devices and mounts from this allocation
		cresp.Devices = []*pluginapi.DeviceSpec{}
		cresp.Mounts = []*pluginapi.Mount{}

		// add new devices
		nodes := newDevice.Nodes()
		for i := range nodes {
			cresp.Devices = append(cresp.Devices, &nodes[i])
		}

		// add new mounts
		mounts := newDevice.Mounts()
		for i := range mounts {
			cresp.Mounts = append(cresp.Mounts, &mounts[i])
		}
	}
}

func numGPUUsingContainers(pod *v1.Pod) int {
	num := 0
	for _, container := range pod.Spec.Containers {
		for reqName, quantity := range container.Resources.Requests {
			resourceName := reqName.String()
			if resourceName == resourcePrefix+deviceType {
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

// containerResources returns a gpu resource map for a single container.
// gpuUsingContainerIndex 0 == first gpu using container in the pod.
func containerResources(pod *v1.Pod, gpuUsingContainerIndex int) resourceMap {
	resources := make(resourceMap)

	i := 0
	for _, container := range pod.Spec.Containers {
		for reqName := range container.Resources.Requests {
			resourceName := reqName.String()
			if resourceName == resourcePrefix+deviceType {
				if i == gpuUsingContainerIndex { // container is found
					for name, quantity := range container.Resources.Requests {
						resourceName = name.String()
						if strings.HasPrefix(resourceName, resourcePrefix) {
							value, _ := quantity.AsInt64()
							resources[resourceName] = value
						}
					}
					return resources
				}
				i++
				break
			}
		}
	}

	return resources
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
				klog.V(3).Infof("Cards for container nr %v are %v", gpuUsingContainerIndex, cards)
				return cards
			}
			i++
		}
	}
	klog.Warningf("couldn't find cards for gpu using container index %v", gpuUsingContainerIndex)
	return nil
}

func containerCardsAndResources(pod *v1.Pod, gpuUsingContainerIndex int) ([]string, resourceMap) {
	cards := containerCards(pod, gpuUsingContainerIndex)
	resources := containerResources(pod, gpuUsingContainerIndex)

	// calculate per card resourceMap (only homogeneous resource consumption is allowed)
	cardCount := int64(len(cards))
	cardResources := resourceMap{}

	if cardCount > 0 {
		for resourceName, value := range resources {
			// calculate per card resources
			cardResources[resourceName] = value / cardCount
		}
	}

	return cards, cardResources
}

// addResourceAnnotations adds resource annotations for cgroup purposes to the container response.
func addResourceAnnotations(cresp *pluginapi.ContainerAllocateResponse, resources resourceMap) error {
	for _, device := range cresp.Devices {
		if strings.Contains(device.HostPath, "card") {
			majorMinor, err := devicePathToMajorMinorStr(device.HostPath)
			if err != nil {
				return errors.WithStack(err)
			}

			for resourceName, value := range resources {
				lastSeparatorIndex := strings.LastIndex(resourceName, "/")
				if lastSeparatorIndex == -1 || lastSeparatorIndex == len(resourceName)-1 {
					return errors.WithStack(err)
				}
				resourceSuffix := resourceName[lastSeparatorIndex+1:]
				valueString := strconv.FormatInt(value, 10)
				key := "gpu." + resourceSuffix
				oldVal, ok := cresp.Annotations[key]
				if ok {
					cresp.Annotations[key] = oldVal + "," + majorMinor + "=" + valueString
				} else {
					cresp.Annotations[key] = majorMinor + "=" + valueString
				}
			}
		}
	}
	return nil
}

func devicePathToMajorMinorStr(path string) (string, error) {
	lastPathSeparatorIndex := strings.LastIndex(path, "/")
	if lastPathSeparatorIndex == -1 || lastPathSeparatorIndex == len(path)-1 {
		return "", errBadPath
	}
	cardName := path[lastPathSeparatorIndex+1:]
	if len(cardName) < 5 {
		return "", errBadCardName
	}
	bytes, err := ioutil.ReadFile("/sys/class/drm/" + cardName + "/dev")
	return strings.TrimSpace(string(bytes)), err
}

func getClientset() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		msg := "Couldn't get cluster config"
		klog.Error(msg)

		return nil
	}
	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		msg := "Couldn't get clientset"
		klog.Error(msg)

		return nil
	}

	return clientset
}

func logPodInfo(pod *v1.Pod, cards []string, resources resourceMap) {
	if klog.V(2).Enabled() {
		klog.Info("Pending pod:" + pod.Name + " devices:")
		klog.Info(cards)
		for reqName, value := range resources {
			klog.Info(reqName, " = ", value)
		}
	}
}
