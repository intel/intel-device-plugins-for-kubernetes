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

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
)

const appLabel = "intel-gpu-plugin"

// newDaemonSetExpected creates plugin daemonset
// it's copied from the original controller code (before the usage of go:embed).
func (c *controller) newDaemonSetExpected(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.GpuDevicePlugin)

	yes := true
	no := false
	daemonSet := apps.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.ns,
			Name:      "intel-gpu-plugin",
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
								ReadOnlyRootFilesystem:   &yes,
								AllowPrivilegeEscalation: &no,
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

	// add the optional init container
	if devicePlugin.Spec.InitImage != "" {
		setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec.InitImage)
	}

	// add service account if resource manager is enabled
	if devicePlugin.Spec.ResourceManager {
		daemonSet.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		addVolumeIfMissing(&daemonSet.Spec.Template.Spec, "podresources", "/var/lib/kubelet/pod-resources", v1.HostPathDirectory)
		addVolumeMountIfMissing(&daemonSet.Spec.Template.Spec, "podresources", "/var/lib/kubelet/pod-resources")
	}

	return &daemonSet
}

// Test that GPU daemonsets created by using go:embed
// are equal to the expected daemonsets.
func TestNewDamonSetGPU(t *testing.T) {
	tcases := []struct {
		name            string
		resourceManager bool
		isInitImage     bool
	}{
		{
			"plugin without resource manager and without initcontainer",
			false,
			false,
		},
		{
			"plugin without resource manager and with initcontainer",
			false,
			true,
		},
		{
			"plugin with resource manager and without initcontainer",
			true,
			false,
		},
		{
			"plugin with resource manager and with initcontainer",
			true,
			true,
		},
	}

	c := &controller{}

	for _, tc := range tcases {
		plugin := &devicepluginv1.GpuDevicePlugin{}
		if tc.resourceManager {
			plugin.Spec.ResourceManager = true
		}

		if tc.isInitImage {
			plugin.Spec.InitImage = "intel/intel-gpu-initcontainer:0.24.0"
		}

		t.Run(tc.name, func(t *testing.T) {
			expected := c.newDaemonSetExpected(plugin)
			actual := c.NewDaemonSet(plugin)

			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("expected and actuall daemonsets differ: %+s", diff.ObjectGoPrintDiff(expected, actual))
			}
		})
	}
}
