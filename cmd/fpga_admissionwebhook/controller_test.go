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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"

	v1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/fpga.intel.com/v1"
	listers "github.com/intel/intel-device-plugins-for-kubernetes/pkg/client/listers/fpga.intel.com/v1"
)

type fakeAfNamespaceLister struct {
	af  *v1.AcceleratorFunction
	err error
}

func (nl *fakeAfNamespaceLister) Get(name string) (*v1.AcceleratorFunction, error) {
	return nl.af, nl.err
}

func (nl *fakeAfNamespaceLister) List(selector labels.Selector) (ret []*v1.AcceleratorFunction, err error) {
	return nil, nil
}

type fakeAfLister struct {
	af  *v1.AcceleratorFunction
	err error
}

func (l *fakeAfLister) AcceleratorFunctions(namespace string) listers.AcceleratorFunctionNamespaceLister {
	return &fakeAfNamespaceLister{
		af:  l.af,
		err: l.err,
	}
}

func (l *fakeAfLister) List(selector labels.Selector) (ret []*v1.AcceleratorFunction, err error) {
	return nil, nil
}

func TestSyncAfHandler(t *testing.T) {
	tcases := []struct {
		name        string
		key         string
		afLister    *fakeAfLister
		expectedErr bool
	}{
		{
			name: "Wrong key format",
			key:  "wrong/key/format/of/object",
		},
		{
			name: "Known key",
			key:  "default/arria10-nlb0",
			afLister: &fakeAfLister{
				af: &v1.AcceleratorFunction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10-nlb0",
					},
					Spec: v1.AcceleratorFunctionSpec{
						AfuID: "d8424dc4a4a3c413f89e433683f9040b",
					},
				},
			},
		},
		{
			name: "Unknown key",
			key:  "default/unknown",
		},
		{
			name: "Unhandled error",
			key:  "default/unknown",
			afLister: &fakeAfLister{
				err: fmt.Errorf("Some fake error"),
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		p, err := newPatcher(preprogrammed)
		if err != nil {
			t.Fatalf("Test case '%s': %+v", tt.name, err)
		}
		c, err := newController(p, &rest.Config{})
		if err != nil {
			t.Fatalf("Test case '%s': %+v", tt.name, err)
		}
		if tt.afLister != nil {
			c.afLister = tt.afLister
		}
		err = c.syncAfHandler(tt.key)
		if err != nil && !tt.expectedErr {
			t.Errorf("Test case '%s': unexpected error: %+v", tt.name, err)
		}
		if err == nil && tt.expectedErr {
			t.Errorf("Test case '%s': expected error, but got success", tt.name)
		}
	}
}

type fakeRegionNamespaceLister struct {
	region *v1.FpgaRegion
	err    error
}

func (nl *fakeRegionNamespaceLister) Get(name string) (*v1.FpgaRegion, error) {
	return nl.region, nl.err
}

func (nl *fakeRegionNamespaceLister) List(selector labels.Selector) (ret []*v1.FpgaRegion, err error) {
	return nil, nil
}

type fakeRegionLister struct {
	region *v1.FpgaRegion
	err    error
}

func (l *fakeRegionLister) FpgaRegions(namespace string) listers.FpgaRegionNamespaceLister {
	return &fakeRegionNamespaceLister{
		region: l.region,
		err:    l.err,
	}
}

func (l *fakeRegionLister) List(selector labels.Selector) (ret []*v1.FpgaRegion, err error) {
	return nil, nil
}

func TestSyncRegionHandler(t *testing.T) {
	tcases := []struct {
		name         string
		key          string
		regionLister *fakeRegionLister
		expectedErr  bool
	}{
		{
			name: "Wrong key format",
			key:  "wrong/key/format/of/object",
		},
		{
			name: "Known key",
			key:  "default/arria10",
			regionLister: &fakeRegionLister{
				region: &v1.FpgaRegion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "arria10",
					},
					Spec: v1.FpgaRegionSpec{
						InterfaceID: "ce48969398f05f33946d560708be108a",
					},
				},
			},
		},
		{
			name: "Unknown key",
			key:  "default/unknown",
		},
		{
			name: "Unhandled error",
			key:  "default/unknown",
			regionLister: &fakeRegionLister{
				err: fmt.Errorf("Some fake error"),
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		p, err := newPatcher(preprogrammed)
		if err != nil {
			t.Fatalf("Test case '%s': %+v", tt.name, err)
		}
		c, err := newController(p, &rest.Config{})
		if err != nil {
			t.Fatalf("Test case '%s': %+v", tt.name, err)
		}
		if tt.regionLister != nil {
			c.regionLister = tt.regionLister
		}
		err = c.syncRegionHandler(tt.key)
		if err != nil && !tt.expectedErr {
			t.Errorf("Test case '%s': unexpected error: %+v", tt.name, err)
		}
		if err == nil && tt.expectedErr {
			t.Errorf("Test case '%s': expected error, but got success", tt.name)
		}
	}
}

type fakeQueue struct {
	obj      *fpgaObjectKey
	shutdown bool
}

func (q *fakeQueue) Add(item interface{}) {}

func (q *fakeQueue) AddAfter(item interface{}, duration time.Duration) {
}

func (q *fakeQueue) AddRateLimited(item interface{}) {}

func (q *fakeQueue) Done(item interface{}) {}

func (q *fakeQueue) Forget(item interface{}) {}

func (q *fakeQueue) Get() (item interface{}, shutdown bool) {
	if q.obj == nil {
		return nil, q.shutdown
	}

	return *q.obj, q.shutdown
}

func (q *fakeQueue) Len() int {
	return 0
}

func (q *fakeQueue) NumRequeues(item interface{}) int {
	return 0
}

func (q *fakeQueue) ShutDown() {}

func (q *fakeQueue) ShuttingDown() bool {
	return false
}

func TestProcessNextWorkItem(t *testing.T) {
	tcases := []struct {
		name             string
		obj              *fpgaObjectKey
		shutdown         bool
		afLister         *fakeAfLister
		regionLister     *fakeRegionLister
		expectedContinue bool
	}{
		{
			name:             "Receive invalid object",
			expectedContinue: true,
		},
		{
			name:     "Shutdown queue",
			shutdown: true,
		},
		{
			name: "Receive unknown kind of object",
			obj: &fpgaObjectKey{
				kind: "unknown",
			},
			expectedContinue: true,
		},
		{
			name: "Receive object with AF key",
			obj: &fpgaObjectKey{
				kind: "af",
			},
			expectedContinue: true,
		},
		{
			name: "Receive object with Region key",
			obj: &fpgaObjectKey{
				kind: "region",
			},
			expectedContinue: true,
		},
		{
			name: "Receive broken object with AF key",
			obj: &fpgaObjectKey{
				kind: "af",
			},
			afLister: &fakeAfLister{
				err: fmt.Errorf("some fake error"),
			},
			expectedContinue: true,
		},
		{
			name: "Receive broken object with Region key",
			obj: &fpgaObjectKey{
				kind: "region",
			},
			regionLister: &fakeRegionLister{
				err: fmt.Errorf("some fake error"),
			},
			expectedContinue: true,
		},
	}
	for _, tt := range tcases {
		p := &patcher{}
		c, err := newController(p, &rest.Config{})
		if err != nil {
			t.Fatalf("Test case '%s': %+v", tt.name, err)
		}
		c.queue = &fakeQueue{
			shutdown: tt.shutdown,
			obj:      tt.obj,
		}
		if tt.afLister != nil {
			c.afLister = tt.afLister
		}
		if tt.regionLister != nil {
			c.regionLister = tt.regionLister
		}
		toContinue := c.processNextWorkItem()
		if toContinue != tt.expectedContinue {
			t.Errorf("Test case '%s': expected toContinue flag %v, but got %v", tt.name, tt.expectedContinue, toContinue)
		}
	}
}

func TestCreateEventhandler(t *testing.T) {
	funcs := createEventHandler("testkind", &fakeQueue{})
	funcs.AddFunc(&v1.FpgaRegion{})
	funcs.UpdateFunc(nil, &v1.FpgaRegion{})
	funcs.DeleteFunc(&v1.FpgaRegion{})
}

func TestRun(t *testing.T) {
	tcases := []struct {
		name        string
		expectedErr bool
	}{
		{
			name:        "Fail to wait for AF caches to sync",
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		p := &patcher{}
		c, err := newController(p, &rest.Config{})
		if err != nil {
			t.Fatalf("Test case '%s': %+v", tt.name, err)
		}
		close(c.stopCh)
		err = c.run(0)
		if err != nil && !tt.expectedErr {
			t.Errorf("Test case '%s': unexpected error: %+v", tt.name, err)
		}
		if err == nil && tt.expectedErr {
			t.Errorf("Test case '%s': expected error, but got success", tt.name)
		}
	}
}

func TestNewController(t *testing.T) {
	tcases := []struct {
		name        string
		configHost  string
		expectedErr bool
	}{
		{
			name:        "wrong config",
			configHost:  "hostname/somepath",
			expectedErr: true,
		},
		{
			name: "successful creation",
		},
	}
	for _, tt := range tcases {
		config := &rest.Config{
			Host: tt.configHost,
		}
		p := &patcher{}
		c, err := newController(p, config)
		if err != nil && !tt.expectedErr {
			t.Errorf("Test case '%s': unexpected error: %+v", tt.name, err)
		}
		if err == nil && tt.expectedErr {
			t.Errorf("Test case '%s': expected error, but got success", tt.name)
		}
		if err == nil && c == nil {
			t.Errorf("Test case '%s': no controller created", tt.name)
		}
	}
}
