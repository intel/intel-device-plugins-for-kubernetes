// Copyright 2022 Intel Corporation. All Rights Reserved.
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

package controllers

import (
	"context"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"

	"errors"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPlugin struct {
	client.Object
}

func (m *mockPlugin) GetNamespace() string {
	return "default"
}

func (m *mockPlugin) GetObjectKind() schema.ObjectKind {
	return &metav1.TypeMeta{
		Kind:       "MockPlugin",
		APIVersion: "v1",
	}
}

func (m *mockPlugin) GetName() string {
	return "mock"
}

func (m *mockPlugin) GetUID() types.UID {
	return "mock-uid"
}

type mockController struct {
	statusErr error
	updated   bool
	upgrade   bool
}

func (m *mockController) CreateEmptyObject() client.Object {
	return &mockPlugin{}
}

func (m *mockController) NewDaemonSet(rawObj client.Object) *apps.DaemonSet {
	return &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock-ds",
			Namespace: "default",
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "mock"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "mock"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "mock",
							Image: "intel/intel-mock-plugin:latest",
						},
					},
				},
			},
		},
	}
}

func (m *mockController) UpdateDaemonSet(rawObj client.Object, ds *apps.DaemonSet) (updated bool) {
	return m.updated
}

func (m *mockController) UpdateStatus(rawObj client.Object, ds *apps.DaemonSet, messages []string) (updated bool, err error) {
	if m.statusErr != nil {
		return false, m.statusErr
	}

	return true, nil
}

func (m *mockController) Upgrade(ctx context.Context, obj client.Object) bool {
	return m.upgrade
}

type fakeStatusWriter struct{}

func (f *fakeStatusWriter) Create(ctx context.Context, obj client.Object, obj2 client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}
func (f *fakeStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}
func (f *fakeStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}
func (f *fakeStatusWriter) Status() client.StatusWriter {
	return f
}

type fakeClient struct {
	client.StatusWriter
	client.Client
	getErr       error
	listErr      error
	updateErr    error
	createErr    error
	statusErr    error
	ds           []*apps.DaemonSet
	pods         []*v1.Pod
	createCalled bool
}

func (f *fakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if f.getErr != nil {
		return f.getErr
	}

	return nil
}
func (f *fakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.listErr != nil {
		return f.listErr
	}
	switch l := list.(type) {
	case *apps.DaemonSetList:
		l.Items = []apps.DaemonSet{}

		for _, ds := range f.ds {
			l.Items = append(l.Items, *ds)
		}
	case *v1.PodList:
		l.Items = []v1.Pod{}

		for _, pod := range f.pods {
			l.Items = append(l.Items, *pod)
		}
	}
	return nil
}
func (f *fakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return f.updateErr
}
func (f *fakeClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	f.createCalled = true
	return f.createErr
}
func (f *fakeClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}
func (f *fakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}
func (f *fakeClient) UpdateStatus(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return f.statusErr
}
func (f *fakeClient) Status() client.StatusWriter {
	return f.StatusWriter
}
func (f *fakeClient) Scheme() *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "deviceplugin.intel.com",
		Version: "v1",
		Kind:    "MockPlugin",
	}, &mockPlugin{})

	return s
}

func fillDaemonSets() []*apps.DaemonSet {
	return []*apps.DaemonSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock-ds",
				Namespace: "default",
			},
			Spec: apps.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "mock"},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "mock"},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "mock",
								Image: "intel/intel-mock-plugin:latest",
							},
						},
					},
				},
			},
		},
	}
}

func fillPods() []*v1.Pod {
	return []*v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock-pod",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
						Name:       "mock-ds",
						UID:        "mock-uid",
					},
				},
			},
			Spec: v1.PodSpec{
				NodeName: "node1",
				Containers: []v1.Container{
					{
						Name:  "mock",
						Image: "intel/intel-mock-plugin:latest",
					},
				},
			},
		},
	}
}

func TestReconciler_Reconcile_CreateDaemonSet(t *testing.T) {
	controller := &mockController{}
	c := &fakeClient{
		StatusWriter: &fakeStatusWriter{},
		ds:           []*apps.DaemonSet{},
	}
	r := &reconciler{
		controller: controller,
		Client:     c,
		scheme:     c.Scheme(),
		pluginKind: "MockPlugin",
		ownerKey:   "owner",
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "mock", Namespace: "default"}}

	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if res != (ctrl.Result{}) {
		t.Errorf("expected empty result, got %v", res)
	}
	if c.createCalled == false {
		t.Error("expected create to be called, but it was not")
	}
}

func TestReconciler_Reconcile_UpdateDaemonSetAndStatus(t *testing.T) {
	controller := &mockController{
		updated: true,
		upgrade: false,
	}
	c := &fakeClient{
		StatusWriter: &fakeStatusWriter{},
		ds:           fillDaemonSets(),
		pods:         fillPods(),
	}
	r := &reconciler{
		controller: controller,
		Client:     c,
		scheme:     c.Scheme(),
		pluginKind: "MockPlugin",
		ownerKey:   "owner",
	}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Name: "mock", Namespace: "default"}}

	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if res != (ctrl.Result{}) {
		t.Errorf("expected empty result, got %v", res)
	}
}

type getError struct {
	error
}
type listError struct {
	error
}
type updatestatusError struct {
	error
}

func TestReconciler_Reconcile_GetError(t *testing.T) {
	controller := &mockController{}
	c := &fakeClient{
		getErr:       getError{},
		StatusWriter: &fakeStatusWriter{},
	}
	r := &reconciler{
		controller: controller,
		Client:     c,
		scheme:     c.Scheme(),
		pluginKind: "MockPlugin",
		ownerKey:   "owner",
	}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Name: "mock", Namespace: "default"}}

	_, err := r.Reconcile(context.Background(), req)
	if err == nil || errors.Is(err, c.getErr) == false {
		t.Errorf("expected get error, got %v", err)
	}
}

func TestReconciler_Reconcile_ListError(t *testing.T) {
	controller := &mockController{}
	c := &fakeClient{
		listErr:      listError{},
		StatusWriter: &fakeStatusWriter{},
	}
	r := &reconciler{
		controller: controller,
		Client:     c,
		scheme:     c.Scheme(),
		pluginKind: "MockPlugin",
		ownerKey:   "owner",
	}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Name: "mock", Namespace: "default"}}

	_, err := r.Reconcile(context.Background(), req)
	if err == nil || errors.Is(err, c.listErr) == false {
		t.Errorf("expected list error, got %v", err)
	}
}

func TestReconciler_Reconcile_UpdateStatusError(t *testing.T) {
	controller := &mockController{
		statusErr: updatestatusError{},
	}
	c := &fakeClient{
		StatusWriter: &fakeStatusWriter{},
		ds:           fillDaemonSets(),
		pods:         fillPods(),
	}
	r := &reconciler{
		controller: controller,
		Client:     c,
		scheme:     c.Scheme(),
		pluginKind: "MockPlugin",
		ownerKey:   "owner",
	}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Name: "mock", Namespace: "default"}}

	_, err := r.Reconcile(context.Background(), req)
	if err == nil || errors.Is(err, controller.statusErr) == false {
		t.Errorf("expected status update error, got %v", err)
	}
}

func TestUpgrade(test *testing.T) {
	image := "intel/intel-dsa-plugin"
	initimage := "intel/intel-idxd-config-initcontainer"
	version := ":" + ImageMinVersion.String()
	prevVersion := ":" + ImageMinVersion.WithMinor(ImageMinVersion.Minor()-1).String()
	tests := []struct {
		envVars           map[string]string
		image             string
		initimage         string
		expectedImage     string
		expectedInitimage string
		upgrade           bool
	}{
		{
			image:             image + prevVersion,
			expectedImage:     image + version,
			initimage:         initimage + prevVersion,
			expectedInitimage: initimage + version,
			upgrade:           true,
		},
		{
			image:             image + version,
			expectedImage:     image + version,
			initimage:         initimage + version,
			expectedInitimage: initimage + version,
			upgrade:           false,
		},
		{
			image:             image + ":devel",
			expectedImage:     image + ":devel",
			initimage:         initimage + ":devel",
			expectedInitimage: initimage + ":devel",
			upgrade:           false,
		},
		{
			image:             image,
			expectedImage:     image,
			initimage:         initimage,
			expectedInitimage: initimage,
			upgrade:           false,
		},
		{
			envVars: map[string]string{
				"INTEL_DSA_PLUGIN_SHA":                "intel/intel-dsa-plugin@sha256:000000000000000000000000000000000000000000000000000000000000000b",
				"INTEL_IDXD_CONFIG_INITCONTAINER_SHA": "intel/intel-idxd-config-initcontainer@sha256:000000000000000000000000000000000000000000000000000000000000000b",
			},
			image:             image + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedImage:     image + "@sha256:000000000000000000000000000000000000000000000000000000000000000b",
			initimage:         initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedInitimage: initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000b",
			upgrade:           true,
		},
		{
			envVars: map[string]string{
				"INTEL_DSA_PLUGIN_SHA":                "intel/intel-dsa-plugin@sha256:000000000000000000000000000000000000000000000000000000000000000a",
				"INTEL_IDXD_CONFIG_INITCONTAINER_SHA": "intel/intel-idxd-config-initcontainer@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			},
			image:             image + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedImage:     image + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			initimage:         initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			expectedInitimage: initimage + "@sha256:000000000000000000000000000000000000000000000000000000000000000a",
			upgrade:           false,
		},
	}

	for i := range tests {
		t := tests[i]

		for key, value := range t.envVars {
			os.Setenv(key, value)
		}

		upgrade := UpgradeImages(context.Background(), &t.image, &t.initimage)

		if !(upgrade == t.upgrade && t.image == t.expectedImage && t.initimage == t.expectedInitimage) {
			test.Errorf("expectedUpgrade: %v, received: %v", t.upgrade, upgrade)
			test.Errorf("expectedImage: %s, received: %s", t.expectedImage, t.image)
			test.Errorf("expectedInitimage: %s, received: %s", t.expectedInitimage, t.initimage)
		}

		for key := range t.envVars {
			os.Unsetenv(key)
		}
	}
}

func TestSuffixedName(test *testing.T) {
	result := SuffixedName("name", "suffix")

	if result != "name-suffix" {
		test.Errorf("invalid suffixed name received: %v", result)
	}
}

func TestHasTolerationsChanged(test *testing.T) {
	tests := []struct {
		desc    string
		pre     []v1.Toleration
		post    []v1.Toleration
		changed bool
	}{
		{
			desc:    "no tolerations",
			pre:     nil,
			post:    nil,
			changed: false,
		},
		{
			desc: "from tolerations to nothing",
			pre: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			post:    nil,
			changed: true,
		},
		{
			desc: "from nothing to tolerations",
			pre:  nil,
			post: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			changed: true,
		},
		{
			desc: "no changes",
			pre: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			post: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			changed: false,
		},
		{
			desc: "tolerations changed",
			pre: []v1.Toleration{
				{Key: "foo", Value: "bar", Operator: "Equal", Effect: "NoSchedule"},
			},
			post: []v1.Toleration{
				{Key: "foo2", Value: "bar2", Operator: "Equal", Effect: "NoSchedule"},
			},
			changed: true,
		},
	}

	for i := range tests {
		t := tests[i]

		changed := HasTolerationsChanged(t.pre, t.post)

		if changed != t.changed {
			test.Errorf("test: %s: expected: %v, received: %v", t.desc, t.changed, changed)
		}
	}
}
