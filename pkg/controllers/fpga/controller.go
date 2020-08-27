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

// Package fpga contains FPGA specific reconciliation logic.
package fpga

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
	ownerKey = ".metadata.controller.fpga"
	appLabel = "intel-fpga-plugin"
)

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=fpgadeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=fpgadeviceplugins/status,verbs=get;update;patch

// SetupReconciler creates a new reconciler for FpgaDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager) error {
	c := &controller{scheme: mgr.GetScheme()}
	return controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "FpgaDevicePlugin", ownerKey)
}

type controller struct {
	scheme *runtime.Scheme
}

func (c *controller) CreateEmptyObject() runtime.Object {
	return &devicepluginv1.FpgaDevicePlugin{}
}

func (c *controller) GetTotalObjectCount(ctx context.Context, clnt client.Client) (int, error) {
	var list devicepluginv1.FpgaDevicePluginList
	if err := clnt.List(ctx, &list); err != nil {
		return 0, err
	}

	return len(list.Items), nil
}

func (c *controller) NewDaemonSet(rawObj runtime.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.FpgaDevicePlugin)
	yes := true
	directoryOrCreate := v1.HostPathDirectoryOrCreate
	return &apps.DaemonSet{
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
							Args: getPodArgs(devicePlugin),
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
							Image:           devicePlugin.Spec.Image,
							ImagePullPolicy: "IfNotPresent",
							Name:            appLabel,
							SecurityContext: &v1.SecurityContext{
								ReadOnlyRootFilesystem: &yes,
							},
							TerminationMessagePath: "/tmp/termination-log",
							VolumeMounts: []v1.VolumeMount{
								{
									MountPath: "/dev",
									Name:      "devfs",
									ReadOnly:  true,
								},
								{
									MountPath: "/sys/class",
									Name:      "sysfs",
									ReadOnly:  true,
								},
								{
									MountPath: "/var/lib/kubelet/device-plugins",
									Name:      "kubeletsockets",
								},
							},
						},
					},
					InitContainers: []v1.Container{
						{
							Image:           devicePlugin.Spec.InitImage,
							ImagePullPolicy: "IfNotPresent",
							Name:            "intel-fpga-initcontainer",
							SecurityContext: &v1.SecurityContext{
								ReadOnlyRootFilesystem: &yes,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									MountPath: "/opt/intel/fpga-sw",
									Name:      "intel-fpga-sw",
								},
								{
									MountPath: "/etc/containers/oci/hooks.d",
									Name:      "oci-hooks-config",
								},
							},
						},
					},
					NodeSelector: devicePlugin.Spec.NodeSelector,
					Volumes: []v1.Volume{
						{
							Name: "devfs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/dev",
								},
							},
						},
						{
							Name: "sysfs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys/class",
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
						{
							Name: "intel-fpga-sw",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/opt/intel/fpga-sw",
									Type: &directoryOrCreate,
								},
							},
						},
						{
							Name: "oci-hooks-config",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/containers/oci/hooks.d",
									Type: &directoryOrCreate,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (c *controller) UpdateDaemonSet(rawObj runtime.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.FpgaDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if ds.Spec.Template.Spec.InitContainers[0].Image != dp.Spec.InitImage {
		ds.Spec.Template.Spec.InitContainers[0].Image = dp.Spec.InitImage
		updated = true
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

func (c *controller) UpdateStatus(rawObj runtime.Object, ds *apps.DaemonSet, nodeNames []string) (updated bool, err error) {
	dp := rawObj.(*devicepluginv1.FpgaDevicePlugin)

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

func getPodArgs(dp *devicepluginv1.FpgaDevicePlugin) []string {
	args := make([]string, 0, 8)
	args = append(args, "-v", strconv.Itoa(dp.Spec.LogLevel))

	if dp.Spec.Mode != "" {
		args = append(args, "-mode", dp.Spec.Mode)
	} else {
		args = append(args, "-mode", "af")
	}

	return args
}
