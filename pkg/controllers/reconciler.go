// Copyright 2020-2022 Intel Corporation. All Rights Reserved.
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

// Package controllers contains code common for the device plugin controllers.
package controllers

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ImageMinVersion = versionutil.MustParseSemantic("0.35.0")
)

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,resourceNames=d1c7b6d5.intel.com,verbs=get;update

// DevicePluginController provides functionality for manipulating actual device plugin CRD objects.
type DevicePluginController interface {
	CreateEmptyObject() (devicePlugin client.Object)
	NewDaemonSet(devicePlugin client.Object) *apps.DaemonSet
	UpdateDaemonSet(client.Object, *apps.DaemonSet) (updated bool)
	UpdateStatus(client.Object, *apps.DaemonSet, []string) (updated bool, err error)
	Upgrade(ctx context.Context, obj client.Object) (upgrade bool)
}

type ControllerOptions struct {
	Namespace           string
	ImagePullSecretName string
	WithWebhook         bool
}

type reconciler struct {
	controller DevicePluginController
	client.Client
	scheme     *runtime.Scheme
	pluginKind string
	ownerKey   string
}

// Combine base and suffix with a dash.
func SuffixedName(base, suffix string) string {
	return base + "-" + suffix
}

func HasTolerationsChanged(before, after []v1.Toleration) bool {
	if before == nil && after == nil {
		return false
	} else if before == nil && after != nil {
		return true
	} else if before != nil && after == nil {
		return true
	}

	return !reflect.DeepEqual(before, after)
}

// fetchObjects returns the required objects for Reconcile.
func (r *reconciler) fetchObjects(ctx context.Context, req ctrl.Request, log logr.Logger) (
	*apps.DaemonSetList, error) {
	// Fetch the plugin's DaemonSet.
	var childDaemonSets apps.DaemonSetList
	if err := r.List(ctx, &childDaemonSets, client.MatchingFields{r.ownerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child DaemonSets")
		return nil, err
	}

	return &childDaemonSets, nil
}

func UpgradeImages(ctx context.Context, image *string, initimage *string) (upgrade bool) {
	for _, s := range []*string{image, initimage} {
		if s == nil {
			continue
		}
		// e.g. intel-dsa-plugin@sha256:hash -> [intel-dsa-plugin@sha256, hash]
		if parts := strings.SplitN(*s, ":", 2); len(parts) == 2 && len(parts[0]) > 0 {
			// e.g. [intel-dsa-plugin@sha256, hash] -> [intel-dsa-plugin, hash]
			name, version := strings.TrimSuffix(parts[0], "@sha256"), parts[1]

			// e.g. intel-dsa-plugin -> INTEL_DSA_PLUGIN_SHA
			// and get the value of the env var INTEL_DSA_PLUGIN_SHA
			envVarValue := os.Getenv(strings.ReplaceAll(strings.ToUpper(filepath.Base(name)), "-", "_") + "_SHA")

			if envVarValue != "" && *s != envVarValue {
				log.FromContext(ctx).Info("env var for the image: " + name + " is already set; user input of the image is ignored")

				*s = envVarValue
				upgrade = true

				continue
			}

			if ver, err := versionutil.ParseSemantic(version); err == nil && ver.LessThan(ImageMinVersion) {
				*s = name + ":" + ImageMinVersion.String()
				upgrade = true
			}
		}
	}

	return upgrade
}

// upgradeDevicePluginImages calls controller's Upgrade function which mostly calls reconcilers' UpgradeImages.
func upgradeDevicePluginImages(ctx context.Context, r *reconciler, devicePlugin client.Object) {
	if r.controller.Upgrade(ctx, devicePlugin) {
		if err := r.Update(ctx, devicePlugin); err != nil {
			log := log.FromContext(ctx)
			log.Error(err, "unable to update devicePlugin")
		}
	}
}

// Reconcile reconciles a device plugin object.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	childDaemonSets, err2 := r.fetchObjects(ctx, req, log)
	if err2 != nil {
		return ctrl.Result{}, err2
	}

	devicePlugin := r.controller.CreateEmptyObject()

	if err := r.Get(ctx, req.NamespacedName, devicePlugin); err != nil {
		return ctrl.Result{}, err
	}

	// Upgrade device plugin object's image, initImage etc.
	upgradeDevicePluginImages(ctx, r, devicePlugin)

	// Create a daemon set for the plugin if it doesn't exist.
	if len(childDaemonSets.Items) == 0 {
		return r.createDaemonSet(ctx, devicePlugin, log)
	}

	ds := &childDaemonSets.Items[0]

	ds0 := ds.DeepCopy()

	// Synchronize the DaemonSet with its owner.
	if r.controller.UpdateDaemonSet(devicePlugin, ds) {
		log.Info("daemonset difference", "diff", diff.Diff(ds0.Spec.Template.Spec, ds.Spec.Template.Spec))

		if err := r.Update(ctx, ds); err != nil {
			log.Error(err, "unable to update DaemonSet", "DaemonSet", ds)
			return ctrl.Result{}, err
		}
	}

	// Fetch the pods controlled by the controller's DaemonSet to list nodes
	var pods v1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(ds.Namespace), client.MatchingFields{r.ownerKey: ds.Name}); err != nil {
		log.Error(err, "unable to list child Pods of the controlled daemon set")
		return ctrl.Result{}, err
	}

	nodeNames := make([]string, len(pods.Items))
	for i, pod := range pods.Items {
		nodeNames[i] = pod.Spec.NodeName
	}

	// Update status
	statusUpdated, err := r.controller.UpdateStatus(devicePlugin, &childDaemonSets.Items[0], nodeNames)
	if err != nil {
		return ctrl.Result{}, err
	}

	if statusUpdated {
		if err := r.Status().Update(ctx, devicePlugin); apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, "unable to update device plugin status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func indexDaemonSets(ctx context.Context, mgr ctrl.Manager, apiGVString, pluginKind, ownerKey string) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &apps.DaemonSet{}, ownerKey,
		func(rawObj client.Object) []string {
			// grab the DaemonSet object, extract the owner...
			ds := rawObj.(*apps.DaemonSet)
			owner := metav1.GetControllerOf(ds)

			if owner == nil {
				return nil
			}

			// make sure it's a device plugin
			if owner.APIVersion != apiGVString || owner.Kind != pluginKind {
				return nil
			}

			// and if so, return it.
			return []string{owner.Name}
		})
}

func indexPods(ctx context.Context, mgr ctrl.Manager, _, _, ownerKey string) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &v1.Pod{}, ownerKey,
		func(rawObj client.Object) []string {
			// grab the Pod object, extract the owner...
			pod := rawObj.(*v1.Pod)
			owner := metav1.GetControllerOf(pod)

			if owner == nil {
				return nil
			}

			// make sure it's a DaemonSet
			if owner.APIVersion != apps.SchemeGroupVersion.String() || owner.Kind != "DaemonSet" {
				return nil
			}

			// and if so, return it.
			return []string{owner.Name}
		})
}

// SetupWithManager sets up a reconciler for a given device plugin controller.
func SetupWithManager(mgr ctrl.Manager, controller DevicePluginController, apiGVString, pluginKind, ownerKey string) error {
	r := &reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		ownerKey:   ownerKey,
		controller: controller,
		pluginKind: pluginKind,
	}

	ctx := context.Background()

	// Index DaemonSets with their owner (e.g. QatDevicePlugin).
	if err := indexDaemonSets(ctx, mgr, apiGVString, pluginKind, ownerKey); err != nil {
		return err
	}

	// Index Pods with their owner (DaemonSet).
	if err := indexPods(ctx, mgr, apiGVString, pluginKind, ownerKey); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(r.controller.CreateEmptyObject()).
		Owns(&apps.DaemonSet{}).
		Complete(r)
}

func (r *reconciler) createDaemonSet(ctx context.Context, dp client.Object, log logr.Logger) (ctrl.Result, error) {
	ds := r.controller.NewDaemonSet(dp)

	if err := ctrl.SetControllerReference(dp.(metav1.Object), ds, r.scheme); err != nil {
		log.Error(err, "unable to set controller reference")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, ds); err != nil {
		log.Error(err, "unable to create DaemonSet")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
