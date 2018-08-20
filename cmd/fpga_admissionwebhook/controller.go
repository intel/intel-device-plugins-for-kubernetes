// Copyright 2018 Intel Corporation. All Rights Reserved.
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

package main

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/intel/intel-device-plugins-for-kubernetes/pkg/client/clientset/versioned"
	informers "github.com/intel/intel-device-plugins-for-kubernetes/pkg/client/informers/externalversions"
	listers "github.com/intel/intel-device-plugins-for-kubernetes/pkg/client/listers/fpga.intel.com/v1"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

const (
	resyncPeriod = 0 * time.Second
)

type fpgaObjectKey struct {
	name string
	kind string
}

type controller struct {
	patcher         *patcher
	informerFactory informers.SharedInformerFactory
	afsSynced       cache.InformerSynced
	regionsSynced   cache.InformerSynced
	afLister        listers.AcceleratorFunctionLister
	regionLister    listers.FpgaRegionLister
	queue           workqueue.RateLimitingInterface
	stopCh          chan struct{}
}

func newController(patcher *patcher, config *rest.Config) (*controller, error) {
	clientset, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST clientset")
	}

	informerFactory := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	stopCh := make(chan struct{})

	afInformer := informerFactory.Fpga().V1().AcceleratorFunctions()
	regionInformer := informerFactory.Fpga().V1().FpgaRegions()

	controller := &controller{
		patcher:         patcher,
		informerFactory: informerFactory,
		queue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		afsSynced:       afInformer.Informer().HasSynced,
		regionsSynced:   regionInformer.Informer().HasSynced,
		afLister:        afInformer.Lister(),
		regionLister:    regionInformer.Lister(),
		stopCh:          stopCh,
	}
	afInformer.Informer().AddEventHandler(createEventHandler("af", controller.queue))
	regionInformer.Informer().AddEventHandler(createEventHandler("region", controller.queue))

	return controller, nil
}

// run sets up the event handlers for AcceleratorFunction and FpgaRegion types,
// as well as syncing informer caches and starts workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *controller) run(threadiness int) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	fmt.Println("Starting controller")

	go c.informerFactory.Start(c.stopCh)

	if ok := cache.WaitForCacheSync(c.stopCh, c.afsSynced); !ok {
		return errors.New("failed to wait for AF caches to sync")
	}

	if ok := cache.WaitForCacheSync(c.stopCh, c.regionsSynced); !ok {
		return errors.New("failed to wait for Region caches to sync")
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(func() {
			for c.processNextWorkItem() {
			}
		}, time.Second, c.stopCh)
	}
	fmt.Println("Started controller workers")
	<-c.stopCh

	return nil
}

// processNextWorkItem reads a single work item off the workqueue and
// attempts to process it, by calling the sync handlers.
func (c *controller) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		var key fpgaObjectKey
		var ok bool

		// We call Done here so the workqueue knows we have finished
		// processing this item.
		defer c.queue.Done(obj)

		if key, ok = obj.(fpgaObjectKey); !ok {
			c.queue.Forget(obj)
			return errors.Errorf("expected fpgaObjectKey in workqueue but got %#v", obj)
		}

		switch key.kind {
		case "af":
			if err := c.syncAfHandler(key.name); err != nil {
				return errors.Wrapf(err, "error syncing '%s'", key.name)
			}
		case "region":
			if err := c.syncRegionHandler(key.name); err != nil {
				return errors.Wrapf(err, "error syncing '%s'", key.name)
			}
		default:
			c.queue.Forget(obj)
			return errors.Errorf("Unknown kind of object key: %s", key.kind)
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		debug.Printf("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *controller) syncAfHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the AcceleratorFunction resource with this namespace/name
	af, err := c.afLister.AcceleratorFunctions(namespace).Get(name)
	if err != nil {
		// The AcceleratorFunction resource may no longer exist, in which case we stop
		// processing.
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("accelerated function '%s' in work queue no longer exists", key))
			debug.Printf("AF '%s' no longer exists", key)
			c.patcher.removeAf(name)
			return nil
		}

		return err
	}

	debug.Print("Received", af)
	c.patcher.addAf(af)
	return nil
}

func (c *controller) syncRegionHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the FpgaRegion resource with this namespace/name
	region, err := c.regionLister.FpgaRegions(namespace).Get(name)
	if err != nil {
		// The FpgaRegion resource may no longer exist, in which case we stop
		// processing.
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("FPGA region '%s' in work queue no longer exists", key))
			debug.Printf("Region '%s' no longer exists", key)
			c.patcher.removeRegion(name)
			return nil
		}

		return err
	}

	debug.Print("Received", region)
	c.patcher.addRegion(region)
	return nil
}

func createEventHandler(kind string, queue workqueue.RateLimitingInterface) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(fpgaObjectKey{
					name: key,
					kind: kind,
				})
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(fpgaObjectKey{
					name: key,
					kind: kind,
				})
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(fpgaObjectKey{
					name: key,
					kind: kind,
				})
			}
		},
	}
}
