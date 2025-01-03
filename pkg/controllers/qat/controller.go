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

// Package qat contains QAT specific reconciliation logic.
package qat

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

const (
	ownerKey          = ".metadata.controller.qat"
	initcontainerName = "intel-qat-initcontainer"
	qatConfigVolume   = "intel-qat-config-volume"
)

var defaultNodeSelector = deployments.QATPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=qatdeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=qatdeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=qatdeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for QatDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, namespace string, withWebhook bool) error {
	c := &controller{scheme: mgr.GetScheme(), ns: namespace}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "QatDevicePlugin", ownerKey); err != nil {
		return err
	}

	if withWebhook {
		return (&devicepluginv1.QatDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	controllers.DefaultServiceAccountFactory
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.QatDevicePlugin{}
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.QatDevicePlugin)
	return controllers.UpgradeImages(ctx, &dp.Spec.Image, &dp.Spec.InitImage)
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.QatDevicePlugin)

	daemonSet := deployments.QATPluginDaemonSet()
	daemonSet.Name = controllers.SuffixedName(daemonSet.Name, devicePlugin.Name)

	if devicePlugin.Spec.Tolerations != nil {
		daemonSet.Spec.Template.Spec.Tolerations = devicePlugin.Spec.Tolerations
	}

	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	if devicePlugin.Spec.InitImage != "" {
		setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec)
	}

	daemonSet.ObjectMeta.Namespace = c.ns
	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	return daemonSet
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.QatDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if dp.Spec.InitImage == "" {
		if ds.Spec.Template.Spec.InitContainers != nil {
			ds.Spec.Template.Spec.InitContainers = nil
			ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "sysfs")
			ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, qatConfigVolume)
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
	dp := rawObj.(*devicepluginv1.QatDevicePlugin)

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

func removeVolume(volumes []v1.Volume, name string) []v1.Volume {
	newVolumes := []v1.Volume{}

	for _, volume := range volumes {
		if volume.Name != name {
			newVolumes = append(newVolumes, volume)
		}
	}

	return newVolumes
}

func setInitContainer(dsSpec *v1.PodSpec, dpSpec devicepluginv1.QatDevicePluginSpec) {
	yes := true

	qatDeviceDriver := map[string]string{
		"dh895xccvf": "0434 0435",
		"c3xxxvf":    "19e2",
		"c6xxvf":     "37c8",
		"d15xxvf":    "6f54",
		"4xxxvf":     "4940 4942 4944",
		"420xxvf":    "4946",
		"c4xxxvf":    "18a0",
	}

	enablingPfPciIDs := make([]string, 0, len(qatDeviceDriver))
	for _, v := range dpSpec.KernelVfDrivers {
		enablingPfPciIDs = append(enablingPfPciIDs, qatDeviceDriver[string(v)])
	}

	dsSpec.InitContainers = []v1.Container{
		{
			Image:           dpSpec.InitImage,
			ImagePullPolicy: "IfNotPresent",
			Name:            initcontainerName,
			Env: []v1.EnvVar{
				{
					Name:  "ENABLED_QAT_PF_PCIIDS",
					Value: strings.Join(enablingPfPciIDs, " "),
				},
				{
					Name: "NODE_NAME",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{
							FieldPath: "spec.nodeName",
						},
					},
				},
			},
			SecurityContext: &v1.SecurityContext{
				SELinuxOptions: &v1.SELinuxOptions{
					Type: "container_device_plugin_init_t",
				},
				Privileged:             &yes,
				ReadOnlyRootFilesystem: &yes,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "sysfs",
					MountPath: "/sys",
				},
			},
		}}
	addVolumeIfMissing(dsSpec, "sysfs", "/sys", v1.HostPathDirectoryOrCreate)

	mode := int32(0440)

	if dpSpec.ProvisioningConfig != "" {
		qatVol := v1.Volume{
			Name: qatConfigVolume,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{Name: dpSpec.ProvisioningConfig},
					DefaultMode:          &mode,
				},
			},
		}

		volumeUpdated := false

		// update ProvisioningConfig volume
		for idx, vol := range dsSpec.Volumes {
			if vol.Name == qatConfigVolume {
				dsSpec.Volumes[idx] = qatVol
				volumeUpdated = true
			}
		}

		// or add if it's completely missing
		if !volumeUpdated {
			dsSpec.Volumes = append(dsSpec.Volumes, qatVol)
		}

		for i, initcontainer := range dsSpec.InitContainers {
			if initcontainer.Name == initcontainerName {
				dsSpec.InitContainers[i].VolumeMounts = append(dsSpec.InitContainers[i].VolumeMounts, v1.VolumeMount{
					Name:      qatConfigVolume,
					MountPath: "/qat-init/conf",
				})
			}
		}
	}
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

func getPodArgs(qdp *devicepluginv1.QatDevicePlugin) []string {
	args := make([]string, 0, 8)
	args = append(args, "-v", strconv.Itoa(qdp.Spec.LogLevel))

	if qdp.Spec.DpdkDriver != "" {
		args = append(args, "-dpdk-driver", qdp.Spec.DpdkDriver)
	} else {
		args = append(args, "-dpdk-driver", "vfio-pci")
	}

	if len(qdp.Spec.KernelVfDrivers) > 0 {
		drvs := make([]string, len(qdp.Spec.KernelVfDrivers))
		for i, v := range qdp.Spec.KernelVfDrivers {
			drvs[i] = string(v)
		}

		args = append(args, "-kernel-vf-drivers", strings.Join(drvs, ","))
	} else {
		args = append(args, "-kernel-vf-drivers", "c6xxvf,4xxxvf")
	}

	if qdp.Spec.MaxNumDevices > 0 {
		args = append(args, "-max-num-devices", strconv.Itoa(qdp.Spec.MaxNumDevices))
	} else {
		args = append(args, "-max-num-devices", "32")
	}

	if qdp.Spec.PreferredAllocationPolicy != "" {
		args = append(args, "-allocation-policy", qdp.Spec.PreferredAllocationPolicy)
	}

	return args
}
