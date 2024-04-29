// Copyright 2021-2022 Intel Corporation. All Rights Reserved.
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

// Package dlb contains DLB specific reconciliation logic.
package dlb

import (
	"context"
	"reflect"
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

const ownerKey = ".metadata.controller.dlb"

var defaultNodeSelector map[string]string = deployments.DLBPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dlbdeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dlbdeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dlbdeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for DlbDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, namespace string, withWebhook bool) error {
	c := &controller{scheme: mgr.GetScheme(), ns: namespace}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "DlbDevicePlugin", ownerKey); err != nil {
		return err
	}

	if withWebhook {
		return (&devicepluginv1.DlbDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	controllers.DefaultServiceAccountFactory
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.DlbDevicePlugin{}
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.DlbDevicePlugin)
	return controllers.UpgradeImages(ctx, &dp.Spec.Image, &dp.Spec.InitImage)
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.DlbDevicePlugin)

	ds := deployments.DLBPluginDaemonSet()
	ds.Name = controllers.SuffixedName(ds.Name, devicePlugin.Name)

	if len(devicePlugin.Spec.NodeSelector) > 0 {
		ds.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	if devicePlugin.Spec.Tolerations != nil {
		ds.Spec.Template.Spec.Tolerations = devicePlugin.Spec.Tolerations
	}

	if devicePlugin.Spec.InitImage == "" {
		ds.Spec.Template.Spec.InitContainers = nil
		ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "sysfs-devices", "sysfs-driver-dlb2")
	} else {
		setInitContainer(&ds.Spec.Template.Spec, devicePlugin.Spec)
	}

	ds.ObjectMeta.Namespace = c.ns

	ds.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	ds.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	return ds
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.DlbDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if dp.Spec.InitImage == "" {
		if ds.Spec.Template.Spec.InitContainers != nil {
			ds.Spec.Template.Spec.InitContainers = nil
			ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "sysfs-devices", "sysfs-driver-dlb2")
			updated = true
		}
	} else {
		containers := ds.Spec.Template.Spec.InitContainers
		if len(containers) != 1 || containers[0].Image != dp.Spec.InitImage {
			setInitContainer(&ds.Spec.Template.Spec, dp.Spec)

			updated = true
		}
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
	dp := rawObj.(*devicepluginv1.DlbDevicePlugin)

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

func removeVolume(volumes []v1.Volume, names ...string) []v1.Volume {
	newVolumes := []v1.Volume{}

	for _, volume := range volumes {
		for i, name := range names {
			if volume.Name == name {
				break
			}

			if i == len(names)-1 {
				newVolumes = append(newVolumes, volume)
			}
		}
	}

	return newVolumes
}

func setInitContainer(dsSpec *v1.PodSpec, dpSpec devicepluginv1.DlbDevicePluginSpec) {
	yes := true

	dsSpec.InitContainers = []v1.Container{
		{
			Image:           dpSpec.InitImage,
			ImagePullPolicy: "IfNotPresent",
			Name:            "enable-vfs",
			SecurityContext: &v1.SecurityContext{
				Privileged:             &yes,
				ReadOnlyRootFilesystem: &yes,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "sysfs-devices",
					MountPath: "/sys/devices",
				},
				{
					Name:      "sysfs-driver-dlb2",
					MountPath: "/sys/bus/pci/drivers/dlb2",
				},
			},
		}}

	addVolumeIfMissing(dsSpec, "sysfs-devices", "/sys/devices", v1.HostPathDirectoryOrCreate)
	addVolumeIfMissing(dsSpec, "sysfs-driver-dlb2", "/sys/bus/pci/drivers/dlb2", v1.HostPathDirectoryOrCreate)
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

func getPodArgs(gdp *devicepluginv1.DlbDevicePlugin) []string {
	args := make([]string, 0, 4)
	args = append(args, "-v", strconv.Itoa(gdp.Spec.LogLevel))

	return args
}
