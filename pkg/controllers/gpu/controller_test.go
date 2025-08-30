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

// Package gpu contains GPU specific reconciliation logic.
package gpu

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const appLabel = "intel-gpu-plugin"

// newDaemonSetExpected creates plugin daemonset
// it's copied from the original controller code (before the usage of go:embed).
func (c *controller) newDaemonSetExpected(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)

	yes := true
	no := false
	directoryOrCreate := v1.HostPathDirectoryOrCreate
	maxUnavailable := intstr.FromInt(1)
	maxSurge := intstr.FromInt(0)

	daemonSet := apps.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.args.Namespace,
			Name:      appLabel + "-" + devicePlugin.Name,
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
			UpdateStrategy: apps.DaemonSetUpdateStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &apps.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
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
								{
									Name: "HOST_IP",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "status.hostIP",
										},
									},
								},
							},
							Args:            getPodArgs(devicePlugin),
							Image:           devicePlugin.Spec.Image,
							ImagePullPolicy: "IfNotPresent",
							SecurityContext: &v1.SecurityContext{
								SELinuxOptions: &v1.SELinuxOptions{
									Type: "container_device_plugin_t",
								},
								ReadOnlyRootFilesystem:   &yes,
								AllowPrivilegeEscalation: &no,
								Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
								SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
							},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("90Mi"),
								},
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("40m"),
									v1.ResourceMemory: resource.MustParse("45Mi"),
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "devfs",
									MountPath: "/dev/dri",
									ReadOnly:  true,
								},
								{
									Name:      "sysfsdrm",
									MountPath: "/sys/class/drm",
									ReadOnly:  true,
								},
								{
									Name:      "kubeletsockets",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
								{
									Name:      "cdipath",
									MountPath: "/var/run/cdi",
								},
							},
						},
					},
					NodeSelector: map[string]string{"kubernetes.io/arch": "amd64"},
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
							Name: "sysfsdrm",
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
						{
							Name: "cdipath",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/run/cdi",
									Type: &directoryOrCreate,
								},
							},
						},
					},
				},
			},
		},
	}

	if len(c.args.ImagePullSecretName) > 0 {
		daemonSet.Spec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
			{Name: c.args.ImagePullSecretName},
		}
	}

	return &daemonSet
}

func (c *controller) updateDaemonSetExpected(rawObj client.Object, ds *apps.DaemonSet) {
	dp := rawObj.(*devicepluginv1.GpuDevicePlugin)

	ds.Spec.Template.Spec.Containers[0].Args = getPodArgs(dp)
}

// Test that GPU daemonsets created by using go:embed
// are equal to the expected daemonsets.
func TestNewDamonSetGPU(t *testing.T) {
	tcases := []struct {
		name string
	}{
		{
			"plugin as is",
		},
	}

	c := &controller{}

	for _, tc := range tcases {
		plugin := &devicepluginv1.GpuDevicePlugin{}

		plugin.Name = "new-gpu-cr-testing"

		t.Run(tc.name, func(t *testing.T) {
			expected := c.newDaemonSetExpected(plugin)
			actual := c.NewDaemonSet(plugin)

			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("expected and actuall daemonsets differ: %+s", cmp.Diff(expected, actual))
			}
		})
	}
}

func TestNewDamonSetGPUWithSecret(t *testing.T) {
	c := &controller{
		args: controllers.ControllerOptions{
			ImagePullSecretName: "mysecret",
		},
	}

	plugin := &devicepluginv1.GpuDevicePlugin{}
	plugin.Name = "new-gpu-cr-testing"

	expected := c.newDaemonSetExpected(plugin)
	actual := c.NewDaemonSet(plugin)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected and actual daemonsets with secret differ: %+s", cmp.Diff(expected, actual))
	}
}

func TestUpdateDamonSetGPU(t *testing.T) {
	tcases := []struct {
		name        string
		sharedCount int
	}{
		{
			"shared dev num as 5",
			5,
		},
	}

	c := &controller{}

	for _, tc := range tcases {
		before := &devicepluginv1.GpuDevicePlugin{}
		before.Name = "update-gpu-cr-testing"

		before.Spec.SharedDevNum = 1

		after := &devicepluginv1.GpuDevicePlugin{}
		after.Name = "update-gpu-cr-testing"

		after.Spec.SharedDevNum = tc.sharedCount

		t.Run(tc.name, func(t *testing.T) {
			expected := c.newDaemonSetExpected(before)
			actual := c.NewDaemonSet(before)

			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("expected and actual daemonsets differ: %+s", cmp.Diff(expected, actual))
			}

			updated := c.UpdateDaemonSet(after, actual)
			if updated == false {
				t.Error("daemonset didn't update while it should have")
			}
			c.updateDaemonSetExpected(after, expected)

			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("updated expected and actual daemonsets differ: %+s", cmp.Diff(expected, actual))
			}
		})
	}
}
