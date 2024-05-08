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

package fpgacontroller

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type mockClient struct {
	getError error
}

func (c *mockClient) Create(context.Context, client.Object, ...client.CreateOption) error {
	return nil
}

func (c *mockClient) Delete(context.Context, client.Object, ...client.DeleteOption) error {
	return nil
}

func (c *mockClient) DeleteAllOf(context.Context, client.Object, ...client.DeleteAllOfOption) error {
	return nil
}

func (c *mockClient) Get(context.Context, types.NamespacedName, client.Object, ...client.GetOption) error {
	return c.getError
}

func (c *mockClient) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return nil
}

func (c *mockClient) Patch(context.Context, client.Object, client.Patch, ...client.PatchOption) error {
	return nil
}

func (c *mockClient) Status() client.SubResourceWriter {
	return nil
}

func (c *mockClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}

func (c *mockClient) Update(context.Context, client.Object, ...client.UpdateOption) error {
	return nil
}

func (c *mockClient) Scheme() *runtime.Scheme {
	return nil
}

func (c *mockClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (c *mockClient) GroupVersionKindFor(runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (c *mockClient) IsObjectNamespaced(runtime.Object) (bool, error) {
	return false, nil
}

type mockManager struct {
	scheme *runtime.Scheme
	log    logr.Logger
}

func (m *mockManager) Add(manager.Runnable) error {
	return nil
}

func (m *mockManager) AddHealthzCheck(string, healthz.Checker) error {
	return nil
}

func (m *mockManager) AddMetricsServerExtraHandler(string, http.Handler) error {
	return nil
}

func (m *mockManager) AddReadyzCheck(string, healthz.Checker) error {
	return nil
}

func (m *mockManager) Elected() <-chan struct{} {
	return nil
}

func (m *mockManager) GetAPIReader() client.Reader {
	return nil
}

func (m *mockManager) GetCache() cache.Cache {
	return nil
}

func (m *mockManager) GetClient() client.Client {
	return nil
}

func (m *mockManager) GetConfig() *rest.Config {
	return nil
}

func (m *mockManager) GetEventRecorderFor(string) record.EventRecorder {
	return nil
}

func (m *mockManager) GetFieldIndexer() client.FieldIndexer {
	return nil
}

func (m *mockManager) GetRESTMapper() meta.RESTMapper {
	return nil
}

func (m *mockManager) GetScheme() *runtime.Scheme {
	return m.scheme
}

func (m *mockManager) GetWebhookServer() webhook.Server {
	return nil
}

func (m *mockManager) GetLogger() logr.Logger {
	return m.log
}

func (m *mockManager) GetControllerOptions() config.Controller {
	return config.Controller{}
}

func (m *mockManager) SetFields(interface{}) error {
	return nil
}

func (m *mockManager) Start(context.Context) error {
	return nil
}

func (m *mockManager) GetHTTPClient() *http.Client {
	return nil
}
