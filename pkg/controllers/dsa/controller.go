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

// Package dsa contains DSA specific reconciliation logic.
package dsa

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
	ownerKey = ".metadata.controller.dsa"
	appLabel = "intel-dsa-plugin"
)

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dsadeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=dsadeviceplugins/status,verbs=get;update;patch

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
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.DsaDevicePlugin{}
}

func (c *controller) GetTotalObjectCount(ctx context.Context, clnt client.Client) (int, error) {
	var list devicepluginv1.DsaDevicePluginList
	if err := clnt.List(ctx, &list); err != nil {
		return 0, err
	}

	return len(list.Items), nil
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.DsaDevicePlugin)

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
			Namespace:    c.ns,
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
									MountPath: "/dev/dsa",
									ReadOnly:  true,
								},
								{
									Name:      "chardevs",
									MountPath: "/dev/char",
									ReadOnly:  true,
								},
								{
									Name:      "sysfs",
									MountPath: "/sys/bus",
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
									Path: "/dev/dsa",
								},
							},
						},
						{
							Name: "chardevs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/dev/char",
								},
							},
						},
						{
							Name: "sysfs",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/sys/bus",
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
	return &daemonSet
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.DsaDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
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
