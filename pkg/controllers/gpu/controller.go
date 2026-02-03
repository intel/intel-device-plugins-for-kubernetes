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

// Package gpu contains GPU specific reconciliation logic.
package gpu

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"strings"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/intel/intel-device-plugins-for-kubernetes/deployments"
	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
	"github.com/pkg/errors"
)

const (
	ownerKey = ".metadata.controller.gpu"
)

var defaultNodeSelector = deployments.GPUPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for GpuDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, args controllers.ControllerOptions) error {
	c := &controller{scheme: mgr.GetScheme(), args: args}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "GpuDevicePlugin", ownerKey); err != nil {
		return err
	}

	if args.WithWebhook {
		return (&devicepluginv1.GpuDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	scheme *runtime.Scheme
	args   controllers.ControllerOptions
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.GpuDevicePlugin{}
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.GpuDevicePlugin)
	return controllers.UpgradeImages(ctx, &dp.Spec.Image, &dp.Spec.InitImage)
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)

	daemonSet := deployments.GPUPluginDaemonSet()
	daemonSet.Name = controllers.SuffixedName(daemonSet.Name, devicePlugin.Name)

	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	if devicePlugin.Spec.Tolerations != nil {
		daemonSet.Spec.Template.Spec.Tolerations = devicePlugin.Spec.Tolerations
	}

	daemonSet.ObjectMeta.Namespace = c.args.Namespace
	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	if len(c.args.ImagePullSecretName) > 0 {
		daemonSet.Spec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
			{Name: c.args.ImagePullSecretName},
		}
	}

	daemonSet.Spec.Template.Spec.InitContainers = nil

	if devicePlugin.Spec.VFIOMode {
		daemonSetToVfio(&daemonSet.Spec.Template.Spec)

		if devicePlugin.Spec.InitImage != "" {
			setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec.InitImage,
				getInitArgs(devicePlugin))
		}
	}

	return daemonSet
}

func daemonSetToVfio(spec *v1.PodSpec) {
	conts := spec.Containers

	// Add vfio related volumes and mounts
	addVolumeIfMissing(spec, "devvfio", "/dev/vfio", v1.HostPathDirectory)
	addVolumeIfMissing(spec, "sysbuspci", "/sys/bus/pci", v1.HostPathDirectory)
	addVolumeMountIfMissing(&conts[0], "devvfio", "/dev/vfio")
	addVolumeMountIfMissing(&conts[0], "sysbuspci", "/sys/bus/pci")

	// Remove devfs and sysfsdrm volumes and mounts
	spec.Volumes = removeVolume(spec.Volumes, "devfs")
	spec.Volumes = removeVolume(spec.Volumes, "sysfsdrm")
	conts[0].VolumeMounts = removeVolumeMount(conts[0].VolumeMounts, "devfs")
	conts[0].VolumeMounts = removeVolumeMount(conts[0].VolumeMounts, "sysfsdrm")
}

func daemonSetToNormal(spec *v1.PodSpec) {
	conts := spec.Containers

	// Remove vfio related volumes and mounts
	spec.Volumes = removeVolume(spec.Volumes, "devvfio")
	spec.Volumes = removeVolume(spec.Volumes, "sysbuspci")
	conts[0].VolumeMounts = removeVolumeMount(conts[0].VolumeMounts, "devvfio")
	conts[0].VolumeMounts = removeVolumeMount(conts[0].VolumeMounts, "sysbuspci")

	// Add back devfs and sysfsdrm volumes and mounts
	addVolumeIfMissing(spec, "devfs", "/dev/dri", v1.HostPathDirectory)
	addVolumeIfMissing(spec, "sysfsdrm", "/sys/class/drm", v1.HostPathDirectory)
	addVolumeMountIfMissing(&conts[0], "devfs", "/dev/dri")
	addVolumeMountIfMissing(&conts[0], "sysfsdrm", "/sys/class/drm")
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

func addVolumeMountIfMissing(spec *v1.Container, name, path string) {
	for _, mount := range spec.VolumeMounts {
		if mount.Name == name {
			return
		}
	}

	spec.VolumeMounts = append(spec.VolumeMounts, v1.VolumeMount{
		Name:      name,
		MountPath: path,
	})
}

func setInitContainer(spec *v1.PodSpec, imageName string, args []string) {
	yes := true
	spec.InitContainers = []v1.Container{
		{
			Image:           imageName,
			ImagePullPolicy: "IfNotPresent",
			Args:            args,
			Name:            "intel-gpu-initcontainer",
			SecurityContext: &v1.SecurityContext{
				Privileged: &yes,
				SELinuxOptions: &v1.SELinuxOptions{
					Type: "container_device_plugin_init_t",
				},
				ReadOnlyRootFilesystem: &yes,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					MountPath: "/sys/bus/pci",
					Name:      "sysbuspci",
				},
			},
		}}
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

func removeVolumeMount(mounts []v1.VolumeMount, name string) []v1.VolumeMount {
	newMounts := []v1.VolumeMount{}

	for _, volume := range mounts {
		if volume.Name != name {
			newMounts = append(newMounts, volume)
		}
	}

	return newMounts
}

func processNodeSelector(ds *apps.DaemonSet, dp *devicepluginv1.GpuDevicePlugin) bool {
	if len(dp.Spec.NodeSelector) > 0 {
		if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, dp.Spec.NodeSelector) {
			ds.Spec.Template.Spec.NodeSelector = dp.Spec.NodeSelector

			return true
		}
	} else if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, defaultNodeSelector) {
		ds.Spec.Template.Spec.NodeSelector = defaultNodeSelector

		return true
	}

	return false
}

func processVfioInitcontainer(ds *apps.DaemonSet, dp *devicepluginv1.GpuDevicePlugin) bool {
	initConts := ds.Spec.Template.Spec.InitContainers

	hadInit := len(initConts) == 1
	wantInit := len(dp.Spec.InitImage) > 0

	if hadInit && wantInit {
		changed := false

		if initConts[0].Image != dp.Spec.InitImage {
			initConts[0].Image = dp.Spec.InitImage
			changed = true
		}

		args := getInitArgs(dp)
		if !changed {
			changed = slices.Compare(args, initConts[0].Args) != 0
		}

		initConts[0].Args = args

		return changed
	} else if !hadInit && wantInit {
		// Add init container if it is not present but init image is specified
		setInitContainer(&ds.Spec.Template.Spec, dp.Spec.InitImage, getInitArgs(dp))

		return true
	} else if hadInit && !wantInit {
		// Remove init container if it is present but init image is not specified
		ds.Spec.Template.Spec.InitContainers = nil

		return true
	}

	return false
}

func processVfioMode(ds *apps.DaemonSet, dp *devicepluginv1.GpuDevicePlugin) bool {
	hadVfio := slices.Contains(ds.Spec.Template.Spec.Containers[0].Args, "-run-mode=vfio")
	wantVfio := dp.Spec.VFIOMode

	// VFIO is enabled currently and in future
	if hadVfio && wantVfio {
		// Reassert full VFIO configuration to make reconciliation idempotent and self-healing.
		daemonSetToVfio(&ds.Spec.Template.Spec)

		return processVfioInitcontainer(ds, dp)
	} else if hadVfio && !wantVfio {
		// VFIO is enabled but will be disabled in future
		daemonSetToNormal(&ds.Spec.Template.Spec)

		// Remove init container
		ds.Spec.Template.Spec.InitContainers = nil

		return true
	} else if !hadVfio && wantVfio {
		// VFIO is disabled currently but will be enabled in future
		daemonSetToVfio(&ds.Spec.Template.Spec)

		// Add init container if specified
		if dp.Spec.InitImage != "" {
			setInitContainer(&ds.Spec.Template.Spec, dp.Spec.InitImage, getInitArgs(dp))
		} else {
			ds.Spec.Template.Spec.InitContainers = nil
		}

		return true
	}

	return false
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.GpuDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if processNodeSelector(ds, dp) {
		updated = true
	}

	if processVfioMode(ds, dp) {
		updated = true
	}

	newargs := getPodArgs(dp)
	oldArgString := strings.Join(ds.Spec.Template.Spec.Containers[0].Args, " ")

	if oldArgString != strings.Join(newargs, " ") {
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
	dp := rawObj.(*devicepluginv1.GpuDevicePlugin)

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

func getInitArgs(gdp *devicepluginv1.GpuDevicePlugin) []string {
	args := []string{}

	if gdp.Spec.AllowIDs != "" {
		args = append(args, "-allow-ids", strings.ToLower(gdp.Spec.AllowIDs))
	}

	if gdp.Spec.DenyIDs != "" {
		args = append(args, "-deny-ids", strings.ToLower(gdp.Spec.DenyIDs))
	}

	return args
}

func getPodArgs(gdp *devicepluginv1.GpuDevicePlugin) []string {
	args := make([]string, 0, 8)
	args = append(args, "-v", strconv.Itoa(gdp.Spec.LogLevel))

	if gdp.Spec.EnableMonitoring {
		args = append(args, "-enable-monitoring")
	}

	if gdp.Spec.SharedDevNum > 0 {
		args = append(args, "-shared-dev-num", strconv.Itoa(gdp.Spec.SharedDevNum))
	} else {
		args = append(args, "-shared-dev-num", "1")
	}

	if gdp.Spec.PreferredAllocationPolicy != "" {
		args = append(args, "-allocation-policy", gdp.Spec.PreferredAllocationPolicy)
	} else {
		args = append(args, "-allocation-policy", "none")
	}

	if gdp.Spec.AllowIDs != "" {
		args = append(args, "-allow-ids", strings.ToLower(gdp.Spec.AllowIDs))
	}

	if gdp.Spec.DenyIDs != "" {
		args = append(args, "-deny-ids", strings.ToLower(gdp.Spec.DenyIDs))
	}

	if gdp.Spec.ByPathMode != "" {
		args = append(args, "-bypath", gdp.Spec.ByPathMode)
	}

	if gdp.Spec.VFIOMode {
		args = append(args, "-run-mode=vfio")
	}

	return args
}
