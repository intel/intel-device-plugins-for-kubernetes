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

// Package fpga contains FPGA specific reconciliation logic.
package fpga

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

const appLabel = "intel-fpga-plugin"

// newDaemonSetExpected creates plugin daemonset
// it's copied from the original controller code (before the usage of go:embed).
func (c *controller) newDaemonSetExpected(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.FpgaDevicePlugin)
	yes := true
	no := false
	directoryOrCreate := v1.HostPathDirectoryOrCreate

	return &apps.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.ns,
			Name:      "intel-fpga-plugin",
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
								ReadOnlyRootFilesystem:   &yes,
								AllowPrivilegeEscalation: &no,
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
								ReadOnlyRootFilesystem:   &yes,
								AllowPrivilegeEscalation: &no,
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
					NodeSelector: map[string]string{"kubernetes.io/arch": "amd64"},
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

// Test that FPGA daemonset created by using go:embed is
// equal to the expected daemonset.
func TestNewDaemonSetFPGA(t *testing.T) {
	c := &controller{}

	plugin := &devicepluginv1.FpgaDevicePlugin{
		Spec: devicepluginv1.FpgaDevicePluginSpec{
			InitImage: "intel/intel-fpga-initcontainer:0.24.0",
		},
	}

	expected := c.newDaemonSetExpected(plugin)
	actual := c.NewDaemonSet(plugin)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected and actuall daemonsets differ: %+s", diff.ObjectGoPrintDiff(expected, actual))
	}
}
