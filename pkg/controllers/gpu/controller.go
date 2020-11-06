// Copyright 2020 Intel Corporation. All Rights Reserved.
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
	"github.com/pkg/errors"
)

const (
	ownerKey = ".metadata.controller.gpu"
	appLabel = "intel-gpu-plugin"
)

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=gpudeviceplugins/status,verbs=get;update;patch

// SetupReconciler creates a new reconciler for GpuDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager) error {
	c := &controller{scheme: mgr.GetScheme()}
	return controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "GpuDevicePlugin", ownerKey)
}

type controller struct {
	scheme *runtime.Scheme
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.GpuDevicePlugin{}
}

func (c *controller) GetTotalObjectCount(ctx context.Context, clnt client.Client) (int, error) {
	var list devicepluginv1.GpuDevicePluginList
	if err := clnt.List(ctx, &list); err != nil {
		return 0, err
	}

	return len(list.Items), nil
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)

	var nodeSelector map[string]string
	dpNodeSelectorSize := len(devicePlugin.Spec.NodeSelector)
	if dpNodeSelectorSize > 0 {
		nodeSelector = make(map[string]string, dpNodeSelectorSize+1)
		for k, v := range devicePlugin.Spec.NodeSelector {
			nodeSelector[k] = v
		}
		nodeSelector["kubernetes.io/arch"] = "amd64"
	} else {
		nodeSelector = map[string]string{"kubernetes.io/arch": "amd64"}
	}

	yes := true
	daemonSet := apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    devicePlugin.Namespace,
			GenerateName: devicePlugin.Name + "-",
			Labels: map[string]string{
				"app": appLabel,
			},
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appLabel,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": appLabel,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: appLabel,
							Env: []v1.EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							Args:            getPodArgs(devicePlugin),
							Image:           devicePlugin.Spec.Image,
							ImagePullPolicy: "IfNotPresent",
							SecurityContext: &v1.SecurityContext{
								ReadOnlyRootFilesystem: &yes,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "devfs",
									MountPath: "/dev/dri",
									ReadOnly:  true,
								},
								{
									Name:      "sysfs",
									MountPath: "/sys/class/drm",
									ReadOnly:  true,
								},
								{
									Name:      "kubeletsockets",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
					NodeSelector: nodeSelector,
					Volumes: []v1.Volume{
						{
							Name: "devfs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/dev/dri",
								},
							},
						},
						{
							Name: "sysfs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys/class/drm",
								},
							},
						},
						{
							Name: "kubeletsockets",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
					},
				},
			},
		},
	}
	// add the optional InitImage
	if devicePlugin.Spec.InitImage != "" {
		setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec.InitImage)
	}
	return &daemonSet
}

func setInitContainer(spec *v1.PodSpec, imageName string) {
	yes := true
	spec.InitContainers = []v1.Container{
		{
			Image:           imageName,
			ImagePullPolicy: "IfNotPresent",
			Name:            "intel-gpu-initcontainer",
			SecurityContext: &v1.SecurityContext{
				ReadOnlyRootFilesystem: &yes,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					MountPath: "/etc/kubernetes/node-feature-discovery/source.d/",
					Name:      "nfd-source-hooks",
				},
			},
		}}
	directoryOrCreate := v1.HostPathDirectoryOrCreate
	missing := true
	for _, vol := range spec.Volumes {
		if vol.Name == "nfd-source-hooks" {
			missing = false
			break
		}
	}
	if missing {
		spec.Volumes = append(spec.Volumes, v1.Volume{
			Name: "nfd-source-hooks",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/etc/kubernetes/node-feature-discovery/source.d/",
					Type: &directoryOrCreate,
				},
			},
		})
	}
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

	if dp.Spec.NodeSelector == nil {
		dp.Spec.NodeSelector = map[string]string{"kubernetes.io/arch": "amd64"}
	} else {
		dp.Spec.NodeSelector["kubernetes.io/arch"] = "amd64"
	}
	if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, dp.Spec.NodeSelector) {
		ds.Spec.Template.Spec.NodeSelector = dp.Spec.NodeSelector
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
	args := make([]string, 0, 4)
	args = append(args, "-v", strconv.Itoa(gdp.Spec.LogLevel))

	if gdp.Spec.SharedDevNum > 0 {
		args = append(args, "-shared-dev-num", strconv.Itoa(gdp.Spec.SharedDevNum))
	} else {
		args = append(args, "-shared-dev-num", "1")
	}

	if gdp.Spec.MonitoringDev {
		args = append(args, "-monitoring")
	}

	return args
}
