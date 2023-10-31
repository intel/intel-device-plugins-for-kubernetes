// Copyright 2020-2022 Intel Corporation. All Rights Reserved.
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

// Package sgx contains SGX specific reconciliation logic.
package sgx

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/intel/intel-device-plugins-for-kubernetes/deployments"
	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

const ownerKey = ".metadata.controller.sgx"

var defaultNodeSelector = deployments.SGXPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=sgxdeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=sgxdeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=sgxdeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for SgxDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, namespace string, withWebhook bool) error {
	c := &controller{scheme: mgr.GetScheme(), ns: namespace}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "SgxDevicePlugin", ownerKey); err != nil {
		return err
	}

	if withWebhook {
		return (&devicepluginv1.SgxDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	controllers.DefaultServiceAccountFactory
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.SgxDevicePlugin)
	return controllers.UpgradeImages(ctx, &dp.Spec.Image, &dp.Spec.InitImage)
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.SgxDevicePlugin{}
}

func addVolumeIfMissing(spec *v1.PodSpec, name, path string, hpType v1.HostPathType) {
	for _, vol := range spec.Volumes {
		if vol.Name == name {
			return
		}
	}

	spec.Volumes = append(spec.Volumes, v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: path,
				Type: &hpType,
			},
		},
	})
}

func setInitContainer(spec *v1.PodSpec, imageName string) {
	yes := true
	spec.InitContainers = []v1.Container{
		{
			Image:           imageName,
			ImagePullPolicy: "IfNotPresent",
			Name:            "intel-sgx-initcontainer",
			SecurityContext: &v1.SecurityContext{
				SELinuxOptions: &v1.SELinuxOptions{
					Type: "container_device_plugin_init_t",
				},
				ReadOnlyRootFilesystem: &yes,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					MountPath: "/etc/kubernetes/node-feature-discovery/source.d/",
					Name:      "nfd-features",
				},
			},
		}}
	addVolumeIfMissing(spec, "nfd-features", "/etc/kubernetes/node-feature-discovery/source.d/", v1.HostPathDirectoryOrCreate)
}

func setNRIContainer(spec *v1.PodSpec, imageName string) {
	yes := true
	no := false
	spec.Containers = append(spec.Containers, v1.Container{
		Name:            "nri-sgx-epc",
		Image:           imageName,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &v1.SecurityContext{
			ReadOnlyRootFilesystem:   &yes,
			AllowPrivilegeEscalation: &no,
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "nrisockets",
				MountPath: "/var/run/nri",
			},
		},
	})
	addVolumeIfMissing(spec, "nrisockets", "/var/run/nri", v1.HostPathDirectoryOrCreate)
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.SgxDevicePlugin)

	daemonSet := deployments.SGXPluginDaemonSet()
	daemonSet.Name = controllers.SuffixedName(daemonSet.Name, devicePlugin.Name)

	if devicePlugin.Spec.Tolerations != nil {
		daemonSet.Spec.Template.Spec.Tolerations = devicePlugin.Spec.Tolerations
	}

	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	daemonSet.ObjectMeta.Namespace = c.ns

	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	// add the optional init container
	if devicePlugin.Spec.InitImage != "" {
		setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec.InitImage)
	}
	// add the optional NRI plugin container
	if devicePlugin.Spec.NRIImage != "" {
		setNRIContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec.NRIImage)
	}

	return daemonSet
}

func removeVolume(volumes []v1.Volume, name string) []v1.Volume {
	newVolumes := []v1.Volume{}

	for _, volume := range volumes {
		if volume.Name != name {
			newVolumes = append(newVolumes, volume)
		}
	}

	return newVolumes
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.SgxDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if dp.Spec.InitImage == "" {
		if ds.Spec.Template.Spec.InitContainers != nil {
			ds.Spec.Template.Spec.InitContainers = nil
			ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "nfd-features")
			updated = true
		}
	} else {
		setInitContainer(&ds.Spec.Template.Spec, dp.Spec.InitImage)

		updated = true
	}

	// remove NRI plugin
	if len(ds.Spec.Template.Spec.Containers) > 1 && dp.Spec.NRIImage == "" {
		ds.Spec.Template.Spec.Containers = []v1.Container{ds.Spec.Template.Spec.Containers[0]}
		ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "nrisockets")
		updated = true
	}

	// update NRI plugin image
	if len(ds.Spec.Template.Spec.Containers) > 1 && ds.Spec.Template.Spec.Containers[1].Image != dp.Spec.NRIImage {
		ds.Spec.Template.Spec.Containers[1].Image = dp.Spec.NRIImage
		updated = true
	}

	// add NRI plugin image
	if len(ds.Spec.Template.Spec.Containers) == 1 && dp.Spec.NRIImage != "" {
		setNRIContainer(&ds.Spec.Template.Spec, dp.Spec.NRIImage)

		updated = true
	}

	if len(dp.Spec.NodeSelector) > 0 {
		if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, dp.Spec.NodeSelector) {
			ds.Spec.Template.Spec.NodeSelector = dp.Spec.NodeSelector
			updated = true
		}
	} else if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, defaultNodeSelector) {
		ds.Spec.Template.Spec.NodeSelector = defaultNodeSelector
		updated = true
	}

	newargs := getPodArgs(dp)
	if strings.Join(ds.Spec.Template.Spec.Containers[0].Args, " ") != strings.Join(newargs, " ") {
		ds.Spec.Template.Spec.Containers[0].Args = newargs
		updated = true
	}

	if controllers.HasTolerationsChanged(ds.Spec.Template.Spec.Tolerations, dp.Spec.Tolerations) {
		ds.Spec.Template.Spec.Tolerations = dp.Spec.Tolerations
		updated = true
	}

	return updated
}

func (c *controller) UpdateStatus(rawObj client.Object, ds *apps.DaemonSet, nodeNames []string) (updated bool, err error) {
	dp := rawObj.(*devicepluginv1.SgxDevicePlugin)

	dsRef, err := reference.GetReference(c.scheme, ds)
	if err != nil {
		return false, errors.Wrap(err, "unable to make reference to controlled daemon set")
	}

	if dp.Status.ControlledDaemonSet.UID != dsRef.UID {
		dp.Status.ControlledDaemonSet = *dsRef
		updated = true
	}

	if dp.Status.DesiredNumberScheduled != ds.Status.DesiredNumberScheduled {
		dp.Status.DesiredNumberScheduled = ds.Status.DesiredNumberScheduled
		updated = true
	}

	if dp.Status.NumberReady != ds.Status.NumberReady {
		dp.Status.NumberReady = ds.Status.NumberReady
		updated = true
	}

	if strings.Join(dp.Status.NodeNames, ",") != strings.Join(nodeNames, ",") {
		dp.Status.NodeNames = nodeNames
		updated = true
	}

	return updated, nil
}

func getPodArgs(sdp *devicepluginv1.SgxDevicePlugin) []string {
	args := make([]string, 0, 4)
	args = append(args, "-v", strconv.Itoa(sdp.Spec.LogLevel))

	if sdp.Spec.EnclaveLimit > 0 {
		args = append(args, "-enclave-limit", strconv.Itoa(sdp.Spec.EnclaveLimit))
	} else {
		args = append(args, "-enclave-limit", "1")
	}

	if sdp.Spec.ProvisionLimit > 0 {
		args = append(args, "-provision-limit", strconv.Itoa(sdp.Spec.ProvisionLimit))
	} else {
		args = append(args, "-provision-limit", "1")
	}

	return args
}
