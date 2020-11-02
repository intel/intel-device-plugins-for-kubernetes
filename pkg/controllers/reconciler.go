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

// Package controllers contains code common for the device plugin controllers.
package controllers

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	bKeeper = &bookKeeper{}
)

func init() {
	bKeeper.pluginCounter = make(map[string]int)
}

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
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,resourceNames=d1c7b6d5.intel.com,verbs=get;update

// DevicePluginController provides functionality for manipulating actual device plugin CRD objects.
type DevicePluginController interface {
	CreateEmptyObject() (devicePlugin client.Object)
	GetTotalObjectCount(ctx context.Context, client client.Client) (count int, err error)
	NewDaemonSet(devicePlugin client.Object) *apps.DaemonSet
	UpdateDaemonSet(client.Object, *apps.DaemonSet) (updated bool)
	UpdateStatus(client.Object, *apps.DaemonSet, []string) (updated bool, err error)
}

type reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	pluginKind string
	ownerKey   string
	controller DevicePluginController
}

// Reconcile reconciles a device plugin object.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues(r.pluginKind, req.NamespacedName)

	if err := r.updateBookKeeper(ctx); err != nil {
		log.Error(err, "unable to total count of device plugins")
		return ctrl.Result{}, err
	}

	// Fetch the plugin's DaemonSet.
	var childDaemonSets apps.DaemonSetList
	if err := r.List(ctx, &childDaemonSets, client.InNamespace(req.Namespace), client.MatchingFields{r.ownerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child DaemonSets")
		return ctrl.Result{}, err
	}

	devicePlugin := r.controller.CreateEmptyObject()
	if err := r.Get(ctx, req.NamespacedName, devicePlugin); err != nil {
		return r.maybeDeleteDaemonSets(ctx, err, childDaemonSets.Items, log)
	}

	// Create a daemon set for the plugin if it doesn't exist.
	if len(childDaemonSets.Items) == 0 {
		return r.createDaemonSet(ctx, devicePlugin, log)
	}

	ds := &childDaemonSets.Items[0]

	// Synchronize the DaemonSet with its owner.
	if r.controller.UpdateDaemonSet(devicePlugin, ds) {
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

	// Drop redundant daemon sets if any.
	r.maybeDeleteRedundantDaemonSets(ctx, childDaemonSets.Items, log)

	return ctrl.Result{}, nil
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
	if err := mgr.GetFieldIndexer().IndexField(ctx, &apps.DaemonSet{}, ownerKey,
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
		}); err != nil {
		return err
	}

	// Index Pods with their owner (DaemonSet).
	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1.Pod{}, ownerKey,
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
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(r.controller.CreateEmptyObject()).
		Owns(&apps.DaemonSet{}).
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
