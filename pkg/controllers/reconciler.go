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
	"github.com/google/go-cmp/cmp"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	ImageMinVersion = versionutil.MustParseSemantic("0.31.0")
)

const (
	sharedObjectsNone = iota
	sharedObjectsMayUse
	sharedObjectsUsed
)

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;list
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,resourceNames=d1c7b6d5.intel.com,verbs=get;update

// SharedObjectsFactory provides functions for creating service account and cluster rule binding objects.
// Note that the rbac Role can be generated from kubebuilder:rbac comment (some examples above),
// which is the reason why this interface does not yet have a NewRole function.
type SharedObjectsFactory interface {
	// Indicates if plugin will ever require shared objects. Not all plugins do.
	PluginMayRequireSharedObjects() bool
	// Indicates if plugin currently require shared objects.
	PluginRequiresSharedObjects(ctx context.Context, client client.Client) bool
	NewSharedServiceAccount() *v1.ServiceAccount
	NewSharedClusterRoleBinding() *rbacv1.ClusterRoleBinding
}

// DefaultServiceAccountFactory is an empty ServiceAccountFactory. "default" will be used for the service account then.
type DefaultServiceAccountFactory struct{}

func (d *DefaultServiceAccountFactory) NewSharedServiceAccount() *v1.ServiceAccount {
	return nil
}
func (d *DefaultServiceAccountFactory) NewSharedClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return nil
}
func (d *DefaultServiceAccountFactory) PluginMayRequireSharedObjects() bool {
	return false
}
func (d *DefaultServiceAccountFactory) PluginRequiresSharedObjects(ctx context.Context, client client.Client) bool {
	return false
}

// DevicePluginController provides functionality for manipulating actual device plugin CRD objects.
type DevicePluginController interface {
	SharedObjectsFactory
	CreateEmptyObject() (devicePlugin client.Object)
	NewDaemonSet(devicePlugin client.Object) *apps.DaemonSet
	UpdateDaemonSet(client.Object, *apps.DaemonSet) (updated bool)
	UpdateStatus(client.Object, *apps.DaemonSet, []string) (updated bool, err error)
	Upgrade(ctx context.Context, obj client.Object) (upgrade bool)
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

// createSharedObjects creates required objects for Reconcile.
func (r *reconciler) createSharedObjects(ctx context.Context, log logr.Logger) (result ctrl.Result, err error) {
	// Since ServiceAccount and ClusterRoleBinding are can be shared by many,
	// it's not owned by the CR. 'SetControllerReference' in the create daemonset function.
	sa := r.controller.NewSharedServiceAccount()

	if err := r.Create(ctx, sa); client.IgnoreAlreadyExists(err) != nil {
		log.Error(err, "unable to create shared ServiceAccount")
		return result, err
	}

	rb := r.controller.NewSharedClusterRoleBinding()

	if err := r.Create(ctx, rb); client.IgnoreAlreadyExists(err) != nil {
		log.Error(err, "unable to create shared ClusterRoleBinding")
		return ctrl.Result{}, err
	}

	return result, nil
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

// determinateSharedObjectReqs Determinates if the installed plugins require shared objects.
// The result is one of three: no, may use and uses currently.
func (r *reconciler) determinateSharedObjectReqs(ctx context.Context, req ctrl.Request) int {
	ret := sharedObjectsNone

	if !r.controller.PluginMayRequireSharedObjects() {
		return ret
	}

	ret = sharedObjectsMayUse

	// Decide from the untyped objects the need to have shared objects.
	if r.controller.PluginRequiresSharedObjects(ctx, r.Client) {
		ret = sharedObjectsUsed
	}

	return ret
}

// Reconcile reconciles a device plugin object.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	childDaemonSets, err2 := r.fetchObjects(ctx, req, log)
	if err2 != nil {
		return ctrl.Result{}, err2
	}

	sharedObjectsNeed := r.determinateSharedObjectReqs(ctx, req)
	devicePlugin := r.controller.CreateEmptyObject()

	if err := r.Get(ctx, req.NamespacedName, devicePlugin); err != nil {
		// Delete shared objects if they are not needed anymore.
		r.maybeDeleteSharedObjects(ctx, sharedObjectsNeed, log)

		return r.maybeDeleteDaemonSets(ctx, err, childDaemonSets.Items, log)
	}

	if sharedObjectsNeed == sharedObjectsUsed {
		if result, err := r.createSharedObjects(ctx, log); err != nil {
			return result, err
		}
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
		log.Info("daemonset difference", "diff", cmp.Diff(ds0.Spec.Template.Spec, ds.Spec.Template.Spec, diff.IgnoreUnset()))

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

	// Drop redundant daemon sets, role bindings and service accounts, if any.
	r.maybeDeleteRedundantDaemonSets(ctx, childDaemonSets.Items, log)
	// Delete shared objects if they are not needed anymore.
	r.maybeDeleteSharedObjects(ctx, sharedObjectsNeed, log)

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

func indexPods(ctx context.Context, mgr ctrl.Manager, apiGVString, pluginKind, ownerKey string) error {
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

func (r *reconciler) maybeDeleteDaemonSets(ctx context.Context, err error, daemonSets []apps.DaemonSet, log logr.Logger) (ctrl.Result, error) {
	if apierrors.IsNotFound(err) {
		for i := range daemonSets {
			if err = r.Delete(ctx, &daemonSets[i], client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete DaemonSet", "DaemonSet", daemonSets[i])
				return ctrl.Result{}, err
			}
		}

		log.V(1).Info("deleted DaemonSets owned by deleted custom device plugin object")

		return ctrl.Result{}, nil
	}

	log.Error(err, "unable to fetch custom device plugin object")

	return ctrl.Result{}, err
}

func (r *reconciler) maybeDeleteRedundantDaemonSets(ctx context.Context, dsets []apps.DaemonSet, log logr.Logger) {
	count := len(dsets)
	if count > 1 {
		log.V(0).Info("there are redundant DaemonSets", "redundantDS", count-1)

		redundantSets := dsets[1:]
		for i := range redundantSets {
			if err := r.Delete(ctx, &redundantSets[i], client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete redundant DaemonSet", "DaemonSet", redundantSets[i])
			} else {
				log.V(1).Info("deleted redundant DaemonSet", "DaemonSet", redundantSets[i])
			}
		}
	}
}

func (r *reconciler) maybeDeleteSharedObjects(ctx context.Context, sharedObjectsNeed int, log logr.Logger) {
	// Delete shared objects only if plugin may use but is not currently using any.
	if sharedObjectsNeed != sharedObjectsMayUse {
		return
	}

	sa := r.controller.NewSharedServiceAccount()

	if err := r.Delete(ctx, sa, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
		log.Error(err, "unable to delete redundant shared ServiceAccount", "ServiceAccount", sa)
	}

	crb := r.controller.NewSharedClusterRoleBinding()

	if err := r.Delete(ctx, crb, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
		log.Error(err, "unable to delete redundant shared ClusterRoleBinding", "ClusterRoleBinding", crb)
	}
}
