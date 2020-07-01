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

// Package fpgacontroller implements reconiciling controller for
// AcceleratorFunction and FpgaRegion objects used in the FPGA admission
// webhook.
package fpgacontroller

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fpgav2 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v2"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpgacontroller/patcher"
)

// +kubebuilder:rbac:groups=fpga.intel.com,resources=acceleratorfunctions;fpgaregions,verbs=get;list;watch

// AcceleratorFunctionReconciler reconciles AcceleratorFunction objects.
type AcceleratorFunctionReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	PatcherManager *patcher.PatcherManager
}

// Reconcile reconciles updates for AcceleratorFunction objects.
func (r *AcceleratorFunctionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("af", req.NamespacedName)

	p := r.PatcherManager.GetPatcher(req.NamespacedName.Namespace)
	var af fpgav2.AcceleratorFunction
	if err := r.Get(ctx, req.NamespacedName, &af); err != nil {
		if apierrors.IsNotFound(err) {
			p.RemoveAf(req.NamespacedName.Name)
			log.V(4).Info("removed from patcher")
			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to fetch AcceleratorFunction object")
		return ctrl.Result{}, err
	}

	log.V(4).Info("received", "AcceleratorFunction", af)
	return ctrl.Result{}, p.AddAf(&af)
}

// SetupWithManager sets up the controller.
func (r *AcceleratorFunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fpgav2.AcceleratorFunction{}).
		Complete(r)
}

// FpgaRegionReconciler reconciles AcceleratorFunction objects.
type FpgaRegionReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	PatcherManager *patcher.PatcherManager
}

// Reconcile reconciles updates for FpgaRegion objects.
func (r *FpgaRegionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("af", req.NamespacedName)

	p := r.PatcherManager.GetPatcher(req.NamespacedName.Namespace)
	var region fpgav2.FpgaRegion
	if err := r.Get(ctx, req.NamespacedName, &region); err != nil {
		if apierrors.IsNotFound(err) {
			p.RemoveRegion(req.NamespacedName.Name)
			log.V(4).Info("removed from patcher")
			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to fetch FpgaRegion object")
		return ctrl.Result{}, err
	}

	log.V(4).Info("received", "FpgaRegion", region)
	p.AddRegion(&region)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller.
func (r *FpgaRegionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fpgav2.FpgaRegion{}).
		Complete(r)
}
