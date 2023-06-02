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
	"strings"
	"sync"

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
	bKeeper         = &bookKeeper{}
	ImageMinVersion = versionutil.MustParseSemantic("0.27.0")
)

func init() {
	bKeeper.pluginCounter = make(map[string]int)
}

//nolint:govet
type bookKeeper struct {
	sync.Mutex
	pluginCounter map[string]int
}

func (b *bookKeeper) set(pluginKind string, count int) {
	b.Lock()
	defer b.Unlock()

	b.pluginCounter[pluginKind] = count
}

func (b *bookKeeper) count(pluginKind string) int {
	b.Lock()
	defer b.Unlock()

	return b.pluginCounter[pluginKind]
}

// GetDevicePluginCount returns number of device plugin CRs registered.
func GetDevicePluginCount(pluginKind string) int {
	return bKeeper.count(pluginKind)
}

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;list
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,resourceNames=d1c7b6d5.intel.com,verbs=get;update

// ServiceAccountFactory provides functions for creating a service account and related objects
// Note that the rbac Role can be generated from kubebuilder:rbac comment (some examples above),
// which is the reason why this interface does not yet have a NewRole function.
type ServiceAccountFactory interface {
	NewServiceAccount(rawObj client.Object) *v1.ServiceAccount
	NewClusterRoleBinding(rawObj client.Object) *rbacv1.ClusterRoleBinding
}

// DefaultServiceAccountFactory is an empty ServiceAccountFactory. "default" will be used for the service account then.
type DefaultServiceAccountFactory struct{}

func (d *DefaultServiceAccountFactory) NewServiceAccount(rawObj client.Object) *v1.ServiceAccount {
	return nil
}
func (d *DefaultServiceAccountFactory) NewClusterRoleBinding(rawObj client.Object) *rbacv1.ClusterRoleBinding {
	return nil
}

// DevicePluginController provides functionality for manipulating actual device plugin CRD objects.
type DevicePluginController interface {
	ServiceAccountFactory
	CreateEmptyObject() (devicePlugin client.Object)
	GetTotalObjectCount(ctx context.Context, client client.Client) (count int, err error)
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

// fetchObjects returns the required objects for Reconcile.
func (r *reconciler) fetchObjects(ctx context.Context, req ctrl.Request, log logr.Logger) (
	*apps.DaemonSetList, *v1.ServiceAccountList, *rbacv1.ClusterRoleBindingList, error) {
	// Fetch the plugin's DaemonSet.
	var childDaemonSets apps.DaemonSetList
	if err := r.List(ctx, &childDaemonSets, client.MatchingFields{r.ownerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child DaemonSets")
		return nil, nil, nil, err
	}

	// Fetch the plugin's ServiceAccount.
	var childServiceAccounts v1.ServiceAccountList
	if err := r.List(ctx, &childServiceAccounts, client.MatchingFields{r.ownerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child ServiceAccounts")
		return nil, nil, nil, err
	}

	// Fetch the plugin's RoleBinding.
	var childClusterRoleBindings rbacv1.ClusterRoleBindingList
	if err := r.List(ctx, &childClusterRoleBindings, client.MatchingFields{r.ownerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child RoleBindings")
		return nil, nil, nil, err
	}

	return &childDaemonSets, &childServiceAccounts, &childClusterRoleBindings, nil
}

// createObjects creates required objects for Reconcile.
func (r *reconciler) createObjects(ctx context.Context,
	log logr.Logger,
	childServiceAccounts *v1.ServiceAccountList,
	childClusterRoleBindings *rbacv1.ClusterRoleBindingList,
	devicePlugin client.Object) (result ctrl.Result, err error) {
	// Create service account for the plugin if it doesn't exist
	if len(childServiceAccounts.Items) == 0 {
		result, err = r.createServiceAccount(ctx, devicePlugin, log)
		if err != nil {
			return result, err
		}
	}

	// Create role binding for the plugin if it doesn't exist
	if len(childClusterRoleBindings.Items) == 0 {
		result, err = r.createClusterRoleBinding(ctx, devicePlugin, log)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func UpgradeImages(image *string, initimage *string) (upgrade bool) {
	for _, s := range []*string{image, initimage} {
		if s == nil {
			continue
		}

		if parts := strings.SplitN(*s, ":", 2); len(parts) == 2 && len(parts[0]) > 0 {
			name, version := parts[0], parts[1]
			if ver, err := versionutil.ParseSemantic(version); err == nil && ver.LessThan(ImageMinVersion) {
				*s = name + ":" + ImageMinVersion.String()
				upgrade = true
			}
		}
	}

	return upgrade
}

func upgrade(ctx context.Context, r *reconciler, devicePlugin client.Object) {
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

	if err := r.updateBookKeeper(ctx); err != nil {
		log.Error(err, "unable to total count of device plugins")
		return ctrl.Result{}, err
	}

	childDaemonSets, childServiceAccounts, childClusterRoleBindings, err2 := r.fetchObjects(ctx, req, log)
	if err2 != nil {
		return ctrl.Result{}, err2
	}

	devicePlugin := r.controller.CreateEmptyObject()
	if err := r.Get(ctx, req.NamespacedName, devicePlugin); err != nil {
		return r.maybeDeleteDaemonSets(ctx, err, childDaemonSets.Items, log)
	}

	if result, err := r.createObjects(ctx, log, childServiceAccounts, childClusterRoleBindings, devicePlugin); err != nil {
		return result, err
	}

	upgrade(ctx, r, devicePlugin)

	// Create a daemon set for the plugin if it doesn't exist.
	if len(childDaemonSets.Items) == 0 {
		return r.createDaemonSet(ctx, devicePlugin, log)
	}

	ds := &childDaemonSets.Items[0]

	ds0 := ds.DeepCopy()

	// Synchronize the DaemonSet with its owner.
	if r.controller.UpdateDaemonSet(devicePlugin, ds) {
		log.Info("", cmp.Diff(ds0.Spec.Template.Spec, ds.Spec.Template.Spec, diff.IgnoreUnset()))

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
	r.maybeDeleteRedundantClusterRoleBindings(ctx, devicePlugin, childClusterRoleBindings.Items, log)
	r.maybeDeleteRedundantServiceAccounts(ctx, devicePlugin, childServiceAccounts.Items, log)

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

func indexServiceAccounts(ctx context.Context, mgr ctrl.Manager, apiGVString, pluginKind, ownerKey string) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &v1.ServiceAccount{}, ownerKey,
		func(rawObj client.Object) []string {
			// grab the ServiceAccounts object, extract the owner...
			sa := rawObj.(*v1.ServiceAccount)
			owner := metav1.GetControllerOf(sa)
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

func indexRoleBindings(ctx context.Context, mgr ctrl.Manager, apiGVString, pluginKind, ownerKey string) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.ClusterRoleBinding{}, ownerKey,
		func(rawObj client.Object) []string {
			// grab the ServiceAccounts object, extract the owner...
			rb := rawObj.(*rbacv1.ClusterRoleBinding)
			owner := metav1.GetControllerOf(rb)
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

	// Index ServiceAccounts with their owner (e.g. QatDevicePlugin).
	if err := indexServiceAccounts(ctx, mgr, apiGVString, pluginKind, ownerKey); err != nil {
		return err
	}

	// Index RoleBindings with their owner (e.g. QatDevicePlugin).
	if err := indexRoleBindings(ctx, mgr, apiGVString, pluginKind, ownerKey); err != nil {
		return err
	}

	// Index Pods with their owner (DaemonSet).
	if err := indexPods(ctx, mgr, apiGVString, pluginKind, ownerKey); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(r.controller.CreateEmptyObject()).
		Owns(&apps.DaemonSet{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&v1.ServiceAccount{}).
		Complete(r)
}

func (r *reconciler) updateBookKeeper(ctx context.Context) error {
	count, err := r.controller.GetTotalObjectCount(ctx, r)
	if err != nil {
		return err
	}

	bKeeper.set(r.pluginKind, count)

	return nil
}

func (r *reconciler) createClusterRoleBinding(ctx context.Context, dp client.Object, log logr.Logger) (ctrl.Result, error) {
	rb := r.controller.NewClusterRoleBinding(dp)
	if rb == nil {
		// most controllers don't need role bindings
		return ctrl.Result{}, nil
	}

	if err := ctrl.SetControllerReference(dp.(metav1.Object), rb, r.scheme); err != nil {
		log.Error(err, "unable to set controller reference")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, rb); err != nil {
		log.Error(err, "unable to create ClusterRoleBinding")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) createServiceAccount(ctx context.Context, dp client.Object, log logr.Logger) (ctrl.Result, error) {
	sa := r.controller.NewServiceAccount(dp)
	if sa == nil {
		// most controllers don't need service accounts
		return ctrl.Result{}, nil
	}

	if err := ctrl.SetControllerReference(dp.(metav1.Object), sa, r.scheme); err != nil {
		log.Error(err, "unable to set controller reference")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, sa); err != nil {
		log.Error(err, "unable to create ServiceAccount")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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

func (r *reconciler) maybeDeleteRedundantServiceAccounts(ctx context.Context, dp client.Object, sas []v1.ServiceAccount, log logr.Logger) {
	sa := r.controller.NewServiceAccount(dp)
	if sa == nil {
		for _, sa := range sas {
			saCopy := sa
			if err := r.Delete(ctx, &saCopy, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete redundant ServiceAccount", "ServiceAccount", sa)
			} else {
				log.V(1).Info("deleted redundant ServiceAccount", "ServiceAccount", sa)
			}
		}
	}
}

func (r *reconciler) maybeDeleteRedundantClusterRoleBindings(ctx context.Context, dp client.Object, rbs []rbacv1.ClusterRoleBinding, log logr.Logger) {
	rb := r.controller.NewClusterRoleBinding(dp)
	if rb == nil {
		for _, rb := range rbs {
			rbCopy := rb
			if err := r.Delete(ctx, &rbCopy, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete redundant ClusterRoleBinding", "ClusterRoleBinding", rb)
			} else {
				log.V(1).Info("deleted redundant ClusterRoleBinding", "ClusterRoleBinding", rb)
			}
		}
	}
}
