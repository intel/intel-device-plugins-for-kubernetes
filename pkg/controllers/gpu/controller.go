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
	"strconv"
	"strings"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ownerKey           = ".metadata.controller.gpu"
	serviceAccountName = "gpu-manager-sa"
)

var defaultNodeSelector = deployments.GPUPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for GpuDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, namespace string, withWebhook bool) error {
	c := &controller{scheme: mgr.GetScheme(), ns: namespace}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "GpuDevicePlugin", ownerKey); err != nil {
		return err
	}

	if withWebhook {
		return (&devicepluginv1.GpuDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.GpuDevicePlugin{}
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.GpuDevicePlugin)
	return controllers.UpgradeImages(&dp.Spec.Image, &dp.Spec.InitImage)
}

func (c *controller) GetTotalObjectCount(ctx context.Context, clnt client.Client) (int, error) {
	var list devicepluginv1.GpuDevicePluginList
	if err := clnt.List(ctx, &list); err != nil {
		return 0, err
	}

	return len(list.Items), nil
}

func (c *controller) NewServiceAccount(rawObj client.Object) *v1.ServiceAccount {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)
	if devicePlugin.Spec.ResourceManager {
		sa := v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gpu-manager-sa",
				Namespace: c.ns,
			},
		}

		return &sa
	}

	return nil
}

func (c *controller) NewClusterRoleBinding(rawObj client.Object) *rbacv1.ClusterRoleBinding {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)
	if devicePlugin.Spec.ResourceManager {
		rb := rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gpu-manager-rolebinding",
				Namespace: c.ns,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "gpu-manager-sa",
					Namespace: c.ns,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "inteldeviceplugins-gpu-manager-role",
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		return &rb
	}

	return nil
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)

	daemonSet := deployments.GPUPluginDaemonSet()
	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	daemonSet.ObjectMeta.Namespace = c.ns
	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	if devicePlugin.Spec.InitImage == "" {
		daemonSet.Spec.Template.Spec.InitContainers = nil
		daemonSet.Spec.Template.Spec.Volumes = removeVolume(daemonSet.Spec.Template.Spec.Volumes, "nfd-source-hooks")
	} else {
		setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec.InitImage)
	}

	// add service account if resource manager is enabled
	if devicePlugin.Spec.ResourceManager {
		daemonSet.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		addVolumeIfMissing(&daemonSet.Spec.Template.Spec, "podresources", "/var/lib/kubelet/pod-resources", v1.HostPathDirectory)
		addVolumeMountIfMissing(&daemonSet.Spec.Template.Spec, "podresources", "/var/lib/kubelet/pod-resources")
	}

	return daemonSet
}

func addVolumeMountIfMissing(spec *v1.PodSpec, name, mountPath string) {
	for _, mount := range spec.Containers[0].VolumeMounts {
		if mount.Name == name {
			return
		}
	}

	spec.Containers[0].VolumeMounts = append(spec.Containers[0].VolumeMounts, v1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	})
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
			Name:            "intel-gpu-initcontainer",
			SecurityContext: &v1.SecurityContext{
				SELinuxOptions: &v1.SELinuxOptions{
					Type: "container_device_plugin_init_t",
				},
				ReadOnlyRootFilesystem: &yes,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					MountPath: "/etc/kubernetes/node-feature-discovery/source.d/",
					Name:      "nfd-source-hooks",
				},
			},
		}}
	addVolumeIfMissing(spec, "nfd-source-hooks", "/etc/kubernetes/node-feature-discovery/source.d/", v1.HostPathDirectoryOrCreate)
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

func removeVolumeMount(volumeMounts []v1.VolumeMount, name string) []v1.VolumeMount {
	newVolumeMounts := []v1.VolumeMount{}

	for _, volume := range volumeMounts {
		if volume.Name != name {
			newVolumeMounts = append(newVolumeMounts, volume)
		}
	}

	return newVolumeMounts
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.GpuDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if dp.Spec.InitImage == "" {
		if ds.Spec.Template.Spec.InitContainers != nil {
			ds.Spec.Template.Spec.InitContainers = nil
			ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "nfd-source-hooks")
			updated = true
		}
	} else {
		setInitContainer(&ds.Spec.Template.Spec, dp.Spec.InitImage)
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

	newServiceAccountName := "default"
	if dp.Spec.ResourceManager {
		newServiceAccountName = serviceAccountName
	}

	if ds.Spec.Template.Spec.ServiceAccountName != newServiceAccountName {
		ds.Spec.Template.Spec.ServiceAccountName = newServiceAccountName
		if dp.Spec.ResourceManager {
			addVolumeIfMissing(&ds.Spec.Template.Spec, "podresources", "/var/lib/kubelet/pod-resources", v1.HostPathDirectory)
			addVolumeMountIfMissing(&ds.Spec.Template.Spec, "podresources", "/var/lib/kubelet/pod-resources")
		} else {
			ds.Spec.Template.Spec.Volumes = removeVolume(ds.Spec.Template.Spec.Volumes, "podresources")
			ds.Spec.Template.Spec.Containers[0].VolumeMounts = removeVolumeMount(ds.Spec.Template.Spec.Containers[0].VolumeMounts, "podresources")
		}

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

	if gdp.Spec.ResourceManager {
		args = append(args, "-resource-manager")
	}

	if gdp.Spec.PreferredAllocationPolicy != "" {
		args = append(args, "-allocation-policy", gdp.Spec.PreferredAllocationPolicy)
	} else {
		args = append(args, "-allocation-policy", "none")
	}

	return args
}
