// Copyright 2021-2022 Intel Corporation. All Rights Reserved.
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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
)

var _ = Describe("DlbDevicePlugin Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("Basic CRUD operations", func() {
		It("should handle DlbDevicePlugin objects correctly", func() {
			spec := devicepluginv1.DlbDevicePluginSpec{
				Image:        "dlb-testimage",
				InitImage:    "dlb-testinitimage",
				NodeSelector: map[string]string{"feature.node.kubernetes.io/dlb": "true"},
			}

			key := types.NamespacedName{
				Name: "dlbdeviceplugin-test",
			}

			toCreate := &devicepluginv1.DlbDevicePlugin{
				ObjectMeta: metav1.ObjectMeta{
					Name: key.Name,
				},
				Spec: spec,
			}

			By("creating DlbDevicePlugin successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetched := &devicepluginv1.DlbDevicePlugin{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetched)
				return len(fetched.Status.ControlledDaemonSet.UID) > 0
			}, timeout, interval).Should(BeTrue())

			By("checking DaemonSet is created successfully")
			ds := &apps.DaemonSet{}
			_ = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: "intel-dlb-plugin"}, ds)
			Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(spec.Image))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(Equal(spec.InitImage))
			Expect(ds.Spec.Template.Spec.NodeSelector).To(Equal(spec.NodeSelector))

			By("updating DlbDevicePlugin successfully")
			updatedImage := "updated-dlb-testimage"
			updatedInitImage := "updated-dlb-testinitimage"
			updatedNodeSelector := map[string]string{"updated-dlb-nodeselector": "true"}
			updatedLogLevel := 3

			fetched.Spec.Image = updatedImage
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.NodeSelector = updatedNodeSelector
			fetched.Spec.LogLevel = updatedLogLevel

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			fetchedUpdated := &devicepluginv1.DlbDevicePlugin{}
			Eventually(func() devicepluginv1.DlbDevicePluginSpec {
				_ = k8sClient.Get(context.Background(), key, fetchedUpdated)
				return fetchedUpdated.Spec
			}, timeout, interval).Should(Equal(fetched.Spec))
			time.Sleep(interval)

			By("checking DaemonSet is updated successfully")
			_ = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: "intel-dlb-plugin"}, ds)

			expectArgs := []string{
				"-v",
				strconv.Itoa(updatedLogLevel),
			}
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal(updatedImage))
			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(ConsistOf(expectArgs))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(Equal(updatedInitImage))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(Equal(updatedNodeSelector))

			By("updating DlbDevicePlugin with different values successfully")
			updatedInitImage = ""
			updatedNodeSelector = map[string]string{}

			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.NodeSelector = updatedNodeSelector

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			time.Sleep(interval)

			By("checking DaemonSet is updated with different values successfully")
			_ = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: "intel-dlb-plugin"}, ds)
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(And(HaveLen(1), HaveKeyWithValue("kubernetes.io/arch", "amd64")))

			By("deleting DlbDevicePlugin successfully")
			Eventually(func() error {
				f := &devicepluginv1.DlbDevicePlugin{}
				_ = k8sClient.Get(context.Background(), key, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			Eventually(func() error {
				f := &devicepluginv1.DlbDevicePlugin{}
				return k8sClient.Get(context.Background(), key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	It("upgrades", func() {
		dp := &devicepluginv1.DlbDevicePlugin{}

		var image, initimage string

		testUpgrade("dlb", dp, &image, &initimage)

		Expect(dp.Spec.Image == image).To(BeTrue())
		Expect(dp.Spec.InitImage == initimage).To(BeTrue())
	})

	var _ = AfterEach(func() {
		CleanupAfter("dlb", &devicepluginv1.DlbDevicePlugin{})
	})
})
