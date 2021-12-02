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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/intel/intel-device-plugins-for-kubernetes/deployments"
	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/controllers"
	"github.com/pkg/errors"
)

const ownerKey = ".metadata.controller.fpga"

var defaultNodeSelector = deployments.FPGAPluginDaemonSet().Spec.Template.Spec.NodeSelector

// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=fpgadeviceplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=fpgadeviceplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deviceplugin.intel.com,resources=fpgadeviceplugins/finalizers,verbs=update

// SetupReconciler creates a new reconciler for FpgaDevicePlugin objects.
func SetupReconciler(mgr ctrl.Manager, namespace string, withWebhook bool) error {
	c := &controller{scheme: mgr.GetScheme(), ns: namespace}
	if err := controllers.SetupWithManager(mgr, c, devicepluginv1.GroupVersion.String(), "FpgaDevicePlugin", ownerKey); err != nil {
		return err
	}

	if withWebhook {
		return (&devicepluginv1.FpgaDevicePlugin{}).SetupWebhookWithManager(mgr)
	}

	return nil
}

type controller struct {
	controllers.DefaultServiceAccountFactory
	scheme *runtime.Scheme
	ns     string
}

func (c *controller) CreateEmptyObject() client.Object {
	return &devicepluginv1.FpgaDevicePlugin{}
}

func (c *controller) GetTotalObjectCount(ctx context.Context, clnt client.Client) (int, error) {
	var list devicepluginv1.FpgaDevicePluginList
	if err := clnt.List(ctx, &list); err != nil {
		return 0, err
	}

	return len(list.Items), nil
}

func (c *controller) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	devicePlugin := rawObj.(*devicepluginv1.FpgaDevicePlugin)

	daemonSet := deployments.FPGAPluginDaemonSet()
	if len(devicePlugin.Spec.NodeSelector) > 0 {
		daemonSet.Spec.Template.Spec.NodeSelector = devicePlugin.Spec.NodeSelector
	}
	daemonSet.ObjectMeta.Namespace = c.ns
	daemonSet.Spec.Template.Spec.Containers[0].Args = getPodArgs(devicePlugin)
	daemonSet.Spec.Template.Spec.Containers[0].Image = devicePlugin.Spec.Image

	return daemonSet
}

func (c *controller) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	dp := rawObj.(*devicepluginv1.FpgaDevicePlugin)

	if ds.Spec.Template.Spec.Containers[0].Image != dp.Spec.Image {
		ds.Spec.Template.Spec.Containers[0].Image = dp.Spec.Image
		updated = true
	}

	if ds.Spec.Template.Spec.InitContainers[0].Image != dp.Spec.InitImage {
		ds.Spec.Template.Spec.InitContainers[0].Image = dp.Spec.InitImage
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

	return updated
}

func (c *controller) UpdateStatus(rawObj client.Object, ds *apps.DaemonSet, nodeNames []string) (updated bool, err error) {
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
