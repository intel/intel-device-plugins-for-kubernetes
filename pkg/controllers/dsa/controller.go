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

// Package dsa contains DSA specific reconciliation logic.
package dsa

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
	ownerKey         = ".metadata.controller.dsa"
	inicontainerName = "intel-idxd-config-initcontainer"
)

var defaultNodeSelector = deployments.DSAPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dsadeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dsadeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dsadeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for DsaDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, namespace string, withWebhook bool) error {
	c := &controller{scheme: mgr.GetScheme(), ns: namespace}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "DsaDevicePlugin", ownerKey); err != nil {
		return err
	}

	if withWebhook {
		return (&devicepluginv1.DsaDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	controllers.DefaultServiceAccountFactory
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.DsaDevicePlugin{}
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.DsaDevicePlugin)
	return controllers.UpgradeImages(&dp.Spec.Image, &dp.Spec.InitImage)
}

func (c *controller) GetTotalObjectCount(ctx context.Context, clnt client.Client) (int, error) {
	var list devicepluginv1.DsaDevicePluginList
	if err := clnt.List(ctx, &list); err != nil {
		return 0, err
	}

	return len(list.Items), nil
}

func removeInitContainer(ds *apps.DaemonSet, dp *devicepluginv1.DsaDevicePlugin) {
	newInitContainers := []v1.Container{}

	for _, container := range ds.Spec.Template.Spec.InitContainers {
		if container.Name == inicontainerName {
			continue
		}

		newInitContainers = append(newInitContainers, container)
	}

	ds.Spec.Template.Spec.InitContainers = newInitContainers
	newVolumes := []v1.Volume{}

	for _, volume := range ds.Spec.Template.Spec.Volumes {
		if volume.Name == "intel-dsa-config-volume" || volume.Name == "sys-devices" || volume.Name == "scratch" {
			continue
		}

		newVolumes = append(newVolumes, volume)
	}

	ds.Spec.Template.Spec.Volumes = newVolumes
}

func addInitContainer(ds *apps.DaemonSet, dp *devicepluginv1.DsaDevicePlugin) {
	yes := true

	ds.Spec.Template.Spec.InitContainers = append(ds.Spec.Template.Spec.InitContainers, v1.Container{
		Image:           dp.Spec.InitImage,
		ImagePullPolicy: "IfNotPresent",
		Name:            inicontainerName,
		Env: []v1.EnvVar{
			{
				Name: "NODE_NAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
			{
				Name:  "IDXD_DEVICE_TYPE",
				Value: "dsa",
			},
		},
		SecurityContext: &v1.SecurityContext{
			ReadOnlyRootFilesystem: &yes,
			Privileged:             &yes,
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "sys-devices",
				MountPath: "/sys/devices",
			},
			{
				Name:      "scratch",
				MountPath: "/idxd-init/scratch",
			},
		},
	})
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, v1.Volume{
		Name: "sys-devices",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/sys/devices",
			},
		},
	})
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, v1.Volume{
		Name: "scratch",
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	})

	if dp.Spec.ProvisioningConfig != "" {
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, v1.Volume{
			Name: "intel-dsa-config-volume",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{Name: dp.Spec.ProvisioningConfig}},
			},
		})

		for i, initcontainer := range ds.Spec.Template.Spec.InitContainers {
			if initcontainer.Name == inicontainerName {
				ds.Spec.Template.Spec.InitContainers[i].VolumeMounts = append(ds.Spec.Template.Spec.InitContainers[i].VolumeMounts, v1.VolumeMount{
					Name:      "intel-dsa-config-volume",
					MountPath: "/idxd-init/conf",
				})
			}
		}
	}
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.DsaDevicePlugin)

	daemonSet := deployments.DSAPluginDaemonSet()
	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	daemonSet.ObjectMeta.Namespace = c.ns
	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	if devicePlugin.Spec.InitImage != "" {
		addInitContainer(daemonSet, devicePlugin)
	}

	return daemonSet
}

func provisioningUpdate(ds *apps.DaemonSet, dp *devicepluginv1.DsaDevicePlugin) bool {
	update := false
	found := false

	for _, container := range ds.Spec.Template.Spec.InitContainers {
		if container.Name == inicontainerName && container.Image != dp.Spec.InitImage {
			found = true
			update = true

			break
		}
	}

	for _, volume := range ds.Spec.Template.Spec.Volumes {
		if volume.Name == "intel-dsa-config-volume" && volume.ConfigMap.Name != dp.Spec.ProvisioningConfig {
			update = true
			break
		}
	}

	if !found && dp.Spec.InitImage != "" {
		update = true
	}

	return update
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.DsaDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if provisioningUpdate(ds, dp) {
		removeInitContainer(ds, dp)

		if dp.Spec.InitImage != "" {
			addInitContainer(ds, dp)
		}

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

	return updated
}

func (c *controller) UpdateStatus(rawObj client.Object, ds *apps.DaemonSet, nodeNames []string) (updated bool, err error) {
	dp := rawObj.(*devicepluginv1.DsaDevicePlugin)

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

func getPodArgs(gdp *devicepluginv1.DsaDevicePlugin) []string {
	args := make([]string, 0, 4)
	args = append(args, "-v", strconv.Itoa(gdp.Spec.LogLevel))

	if gdp.Spec.SharedDevNum > 0 {
		args = append(args, "-shared-dev-num", strconv.Itoa(gdp.Spec.SharedDevNum))
	} else {
		args = append(args, "-shared-dev-num", "1")
	}

	return args
}
