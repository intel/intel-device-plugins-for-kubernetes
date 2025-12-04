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

	"github.com/google/go-cmp/cmp"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	maxUnavailable := intstr.FromInt(1)
	maxSurge := intstr.FromInt(0)

	ds := &apps.DaemonSet{
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
								SELinuxOptions: &v1.SELinuxOptions{
									Type: "container_device_plugin_t",
								},
								ReadOnlyRootFilesystem:   &yes,
								AllowPrivilegeEscalation: &no,
								Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
								SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
							},
							TerminationMessagePath: "/tmp/termination-log",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("160m"),
									v1.ResourceMemory: resource.MustParse("60Mi"),
								},
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("80m"),
									v1.ResourceMemory: resource.MustParse("30Mi"),
								},
							},
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
								{
									MountPath: "/var/run/cdi",
									Name:      "cdidir",
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
							Name: "cdidir",
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
		ds.Spec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
			{Name: c.args.ImagePullSecretName},
		}
	}

	return ds
}

// Test that FPGA daemonset created by using go:embed is
// equal to the expected daemonset.
func TestNewDaemonSetFPGA(t *testing.T) {
	c := &controller{}

	plugin := &devicepluginv1.FpgaDevicePlugin{
		Spec: devicepluginv1.FpgaDevicePluginSpec{
			InitImage: "intel/intel-fpga-initcontainer:0.34.1",
		},
	}
	plugin.Name = "testing"

	expected := c.newDaemonSetExpected(plugin)
	actual := c.NewDaemonSet(plugin)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected and actuall daemonsets differ: %+s", cmp.Diff(expected, actual))
	}

	c.args.ImagePullSecretName = "mysecret"

	expected = c.newDaemonSetExpected(plugin)
	actual = c.NewDaemonSet(plugin)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected and actual daemonsets with secret differ: %+s", cmp.Diff(expected, actual))
	}
}
