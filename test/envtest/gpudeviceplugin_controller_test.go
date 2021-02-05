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

package envtest

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
)

var _ = Describe("GpuDevicePlugin Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("Basic CRUD operations", func() {
		It("should handle GpuDevicePlugin objects correctly", func() {
			spec := devicepluginv1.GpuDevicePluginSpec{
				Image: "testimage",
			}

			key := types.NamespacedName{
				Name: "gpudeviceplugin-test",
			}

			toCreate := &devicepluginv1.GpuDevicePlugin{
				ObjectMeta: metav1.ObjectMeta{
					Name: key.Name,
				},
				Spec: spec,
			}

			By("creating GpuDevicePlugin successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetched := &devicepluginv1.GpuDevicePlugin{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetched)
				return len(fetched.Status.ControlledDaemonSet.UID) > 0
			}, timeout, interval).Should(BeTrue())

			By("updating image name successfully")
			updatedImage := "updated-testimage"
			fetched.Spec.Image = updatedImage

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			fetchedUpdated := &devicepluginv1.GpuDevicePlugin{}
			Eventually(func() string {
				_ = k8sClient.Get(context.Background(), key, fetchedUpdated)
				return fetchedUpdated.Spec.Image
			}, timeout, interval).Should(Equal(updatedImage))

			By("deleting GpuDevicePlugin successfully")
			Eventually(func() error {
				f := &devicepluginv1.GpuDevicePlugin{}
				_ = k8sClient.Get(context.Background(), key, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			Eventually(func() error {
				f := &devicepluginv1.GpuDevicePlugin{}
				return k8sClient.Get(context.Background(), key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
