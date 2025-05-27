// Copyright 2025 Intel Corporation. All Rights Reserved.
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

// Package npu contains NPU specific reconciliation logic.
package npu

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
	ownerKey = ".metadata.controller.npu"
)

var defaultNodeSelector = deployments.NPUPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=npudeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=npudeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=npudeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for NpuDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, args controllers.ControllerOptions) error {
	c := &controller{scheme: mgr.GetScheme(), args: args}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "NpuDevicePlugin", ownerKey); err != nil {
		return err
	}

	if args.WithWebhook {
		return (&devicepluginv1.NpuDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	scheme *runtime.Scheme
	args   controllers.ControllerOptions
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.NpuDevicePlugin{}
}

func (c *controller) Upgrade(ctx context.Context, obj client.Object) bool {
	dp := obj.(*devicepluginv1.NpuDevicePlugin)
	return controllers.UpgradeImages(ctx, &dp.Spec.Image, nil)
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.NpuDevicePlugin)

	daemonSet := deployments.NPUPluginDaemonSet()
	daemonSet.Name = controllers.SuffixedName(daemonSet.Name, devicePlugin.Name)

	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}

	if devicePlugin.Spec.Tolerations != nil {
		daemonSet.Spec.Template.Spec.Tolerations = devicePlugin.Spec.Tolerations
	}

	if len(c.args.ImagePullSecretName) > 0 {
		daemonSet.Spec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
			{Name: c.args.ImagePullSecretName},
		}
	}

	daemonSet.ObjectMeta.Namespace = c.args.Namespace
	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	return daemonSet
}

func processNodeSelector(ds *apps.DaemonSet, dp *devicepluginv1.NpuDevicePlugin) bool {
	if len(dp.Spec.NodeSelector) > 0 {
		if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, dp.Spec.NodeSelector) {
			ds.Spec.Template.Spec.NodeSelector = dp.Spec.NodeSelector

			return true
		}
	} else if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, defaultNodeSelector) {
		ds.Spec.Template.Spec.NodeSelector = defaultNodeSelector

		return true
	}

	return false
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.NpuDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if processNodeSelector(ds, dp) {
		updated = true
	}

	newargs := getPodArgs(dp)
	oldArgString := strings.Join(ds.Spec.Template.Spec.Containers[0].Args, " ")

	if oldArgString != strings.Join(newargs, " ") {
		ds.Spec.Template.Spec.Containers[0].Args = newargs
		updated = true
	}

	if controllers.HasTolerationsChanged(ds.Spec.Template.Spec.Tolerations, dp.Spec.Tolerations) {
		ds.Spec.Template.Spec.Tolerations = dp.Spec.Tolerations
		updated = true
	}

	return updated
}

func (c *controller) UpdateStatus(rawObj client.Object, ds *apps.DaemonSet, nodeNames []string) (updated bool, err error) {
	dp := rawObj.(*devicepluginv1.NpuDevicePlugin)

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

func getPodArgs(gdp *devicepluginv1.NpuDevicePlugin) []string {
	args := make([]string, 0, 4)
	args = append(args, "-v", strconv.Itoa(gdp.Spec.LogLevel))

	if gdp.Spec.SharedDevNum > 0 {
		args = append(args, "-shared-dev-num", strconv.Itoa(gdp.Spec.SharedDevNum))
	} else {
		args = append(args, "-shared-dev-num", "1")
	}

	return args
}
