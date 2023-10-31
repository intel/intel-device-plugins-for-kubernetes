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

package envtest

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
)

var _ = Describe("SgxDevicePlugin Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("Basic CRUD operations", func() {
		It("should handle SgxDevicePlugin objects correctly", func() {
			spec := devicepluginv1.SgxDevicePluginSpec{
				Image:        "sgx-testimage",
				InitImage:    "sgx-testinitimage",
				NRIImage:     "sgx-testnriimage",
				NodeSelector: map[string]string{"sgx-nodeselector": "true"},
			}

			key := types.NamespacedName{
				Name: "sgxdeviceplugin-test",
			}

			toCreate := &devicepluginv1.SgxDevicePlugin{
				ObjectMeta: metav1.ObjectMeta{
					Name: key.Name,
				},
				Spec: spec,
			}

			expectedDsName := "intel-sgx-plugin-sgxdeviceplugin-test"

			By("creating SgxDevicePlugin successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetched := &devicepluginv1.SgxDevicePlugin{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetched)
				return len(fetched.Status.ControlledDaemonSet.UID) > 0
			}, timeout, interval).Should(BeTrue())

			By("checking DaemonSet is created successfully")
			ds := &apps.DaemonSet{}
			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			Expect(err).To(BeNil())
			Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(spec.Image))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(Equal(spec.InitImage))
			Expect(ds.Spec.Template.Spec.NodeSelector).To(Equal(spec.NodeSelector))
			Expect(ds.Spec.Template.Spec.Tolerations).To(HaveLen(0))

			By("updating SgxDevicePlugin successfully")
			updatedImage := "updated-sgx-testimage"
			updatedInitImage := "updated-sgx-testinitimage"
			updatedNRIImage := "updated-sgx-testnriimage"
			updatedLogLevel := 2
			updatedEnclaveLimit := 2
			updatedProvisionLimit := 2
			updatedNodeSelector := map[string]string{"updated-sgx-nodeselector": "true"}

			fetched.Spec.Image = updatedImage
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.NRIImage = updatedNRIImage
			fetched.Spec.LogLevel = updatedLogLevel
			fetched.Spec.EnclaveLimit = updatedEnclaveLimit
			fetched.Spec.ProvisionLimit = updatedProvisionLimit
			fetched.Spec.NodeSelector = updatedNodeSelector

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			fetchedUpdated := &devicepluginv1.SgxDevicePlugin{}
			Eventually(func() devicepluginv1.SgxDevicePluginSpec {
				_ = k8sClient.Get(context.Background(), key, fetchedUpdated)
				return fetchedUpdated.Spec
			}, timeout, interval).Should(Equal(fetched.Spec))
			time.Sleep(interval)

			By("checking DaemonSet is updated successfully")
			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			Expect(err).To(BeNil())

			expectArgs := []string{
				"-v",
				strconv.Itoa(updatedLogLevel),
				"-enclave-limit",
				strconv.Itoa(updatedEnclaveLimit),
				"-provision-limit",
				strconv.Itoa(updatedProvisionLimit),
			}

			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(ConsistOf(expectArgs))
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal(updatedImage))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(2))
			Expect(ds.Spec.Template.Spec.Containers[1].Image).Should(Equal(updatedNRIImage))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(Equal(updatedInitImage))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(Equal(updatedNodeSelector))

			By("updating SgxDevicePlugin with different values successfully")
			updatedInitImage = ""
			updatedNRIImage = ""
			updatedNodeSelector = map[string]string{}
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.NRIImage = updatedNRIImage
			fetched.Spec.NodeSelector = updatedNodeSelector

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			time.Sleep(interval)

			By("checking DaemonSet is updated with different values successfully")
			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			Expect(err).To(BeNil())
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(0))
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(And(HaveLen(1), HaveKeyWithValue("kubernetes.io/arch", "amd64")))

			By("updating SgxDevicePlugin with tolerations")

			tolerations := []corev1.Toleration{
				{Key: "foo", Operator: "Equal", Value: "bar", Effect: "NoSchedule"},
			}

			fetched.Spec.Tolerations = tolerations
			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			time.Sleep(interval)

			By("checking DaemonSet is updated with tolerations")
			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			Expect(err).To(BeNil())
			Expect(ds.Spec.Template.Spec.Tolerations).To(Equal(tolerations))

			By("deleting SgxDevicePlugin successfully")
			Eventually(func() error {
				f := &devicepluginv1.SgxDevicePlugin{}
				_ = k8sClient.Get(context.Background(), key, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			Eventually(func() error {
				f := &devicepluginv1.SgxDevicePlugin{}
				return k8sClient.Get(context.Background(), key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	It("upgrades", func() {
		dp := &devicepluginv1.SgxDevicePlugin{}

		var image, initimage string

		testUpgrade("sgx", dp, &image, &initimage)

		Expect(dp.Spec.Image == image).To(BeTrue())
		Expect(dp.Spec.InitImage == initimage).To(BeTrue())
	})

	var _ = AfterEach(func() {
		CleanupAfter("sgx", &devicepluginv1.SgxDevicePlugin{})
	})
})
