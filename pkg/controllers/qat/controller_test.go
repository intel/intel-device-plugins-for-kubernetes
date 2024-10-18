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

// Package qat contains QAT specific reconciliation logic.
package qat

import (
	"reflect"
	"testing"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
)

const appLabel = "intel-qat-plugin"

// newDaemonSetExpected creates plugin daemonset
// it's copied from the original controller code (before the usage of go:embed).
func (c *controller) newDaemonSetExpected(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.QatDevicePlugin)
	yes := true
	no := false
	maxUnavailable := intstr.FromInt(1)
	maxSurge := intstr.FromInt(0)

	daemonSet := apps.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.ns,
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
					AutomountServiceAccountToken: &no,
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
								SELinuxOptions: &v1.SELinuxOptions{
									Type: "container_device_plugin_t",
								},
								ReadOnlyRootFilesystem:   &yes,
								AllowPrivilegeEscalation: &no,
							},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("140m"),
									v1.ResourceMemory: resource.MustParse("30Mi"),
								},
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("70m"),
									v1.ResourceMemory: resource.MustParse("15Mi"),
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "devdir",
									MountPath: "/dev/vfio",
									ReadOnly:  true,
								},
								{
									Name:      "debugfsdir",
									MountPath: "/sys/kernel/debug",
									ReadOnly:  true,
								},
								{
									Name:      "pcidir",
									MountPath: "/sys/bus/pci",
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
							Name: "devdir",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/dev/vfio",
								},
							},
						},
						{
							Name: "debugfsdir",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys/kernel/debug",
								},
							},
						},
						{
							Name: "pcidir",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys/bus/pci",
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
		setInitContainer(&daemonSet.Spec.Template.Spec, devicePlugin.Spec)
	}

	return &daemonSet
}

// Test that QAT daemonset created by using go:embed is
// equal to the expected daemonset.
func TestNewDaemonSetQAT(t *testing.T) {
	c := &controller{}

	plugin := &devicepluginv1.QatDevicePlugin{}
	plugin.Name = "testing"
	plugin.Spec.InitImage = "intel/intel-qat-initcontainer:" + controllers.ImageMinVersion.String()

	expected := c.newDaemonSetExpected(plugin)
	actual := c.NewDaemonSet(plugin)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected and actuall daemonsets differ: %+s", diff.ObjectGoPrintDiff(expected, actual))
	}
}
