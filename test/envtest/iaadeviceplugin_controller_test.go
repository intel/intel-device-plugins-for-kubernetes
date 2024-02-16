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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	devicepluginv1 "github.com/intel/intel-device-plugins-for-kubernetes/pkg/apis/deviceplugin/v1"
)

var _ = Describe("IaaDevicePlugin Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("Basic CRUD operations", func() {
		It("should handle IaaDevicePlugin objects correctly", func() {
			spec := devicepluginv1.IaaDevicePluginSpec{
				Image:     "testimage",
				InitImage: "testinitimage",
				NodeSelector: map[string]string{"iaa-nodeselector": "true",
					"kubernetes.io/arch": "amd64"},
			}

			key := types.NamespacedName{
				Name:      "iaadeviceplugin-test",
				Namespace: ns,
			}

			toCreate := &devicepluginv1.IaaDevicePlugin{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: ns,
				},
				Spec: spec,
			}

			expectedDsName := "intel-iaa-plugin-iaadeviceplugin-test"

			By("creating IaaDevicePlugin successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetched := &devicepluginv1.IaaDevicePlugin{}
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

			By("updating IaaDevicePlugin successfully")
			updatedImage := "updated-iaa-testimage"
			updatedInitImage := "updated-iaa-testinitimage"
			updatedProvisioningConfig := "updated-iaa-provisioningconfig"
			fetched.Spec.Image = updatedImage
			updatedLogLevel := 2
			updatedSharedDevNum := 42
			updatedNodeSelector := map[string]string{"updated-iaa-nodeselector": "true",
				"kubernetes.io/arch": "amd64"}

			fetched.Spec.Image = updatedImage
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.ProvisioningConfig = updatedProvisioningConfig
			fetched.Spec.LogLevel = updatedLogLevel
			fetched.Spec.SharedDevNum = updatedSharedDevNum
			fetched.Spec.NodeSelector = updatedNodeSelector

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			fetchedUpdated := &devicepluginv1.IaaDevicePlugin{}
			Eventually(func() devicepluginv1.IaaDevicePluginSpec {
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
				"-shared-dev-num",
				strconv.Itoa(updatedSharedDevNum),
			}
			mode := int32(420)
			expectedVolume := v1.Volume{
				Name: "intel-iaa-config-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{Name: updatedProvisioningConfig},
						DefaultMode:          &mode,
					},
				},
			}

			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(ConsistOf(expectArgs))
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal(updatedImage))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(Equal(updatedInitImage))

			Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(expectedVolume))

			Expect(ds.Spec.Template.Spec.NodeSelector).Should(Equal(updatedNodeSelector))

			By("updating IaaDevicePlugin with different values successfully")
			updatedInitImage = ""
			updatedNodeSelector = map[string]string{}
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.NodeSelector = updatedNodeSelector

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			time.Sleep(interval)

			By("checking DaemonSet is updated with different values successfully")
			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			Expect(err).To(BeNil())
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(0))
			Expect(ds.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(expectedVolume))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(And(HaveLen(1), HaveKeyWithValue("kubernetes.io/arch", "amd64")))

			By("updating IaaDevicePlugin with tolerations")

			tolerations := []v1.Toleration{
				{Key: "foo", Operator: "Equal", Value: "bar", Effect: "NoSchedule"},
			}

			fetched.Spec.Tolerations = tolerations
			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			time.Sleep(interval)

			By("checking DaemonSet is updated with tolerations")
			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			Expect(err).To(BeNil())
			Expect(ds.Spec.Template.Spec.Tolerations).To(Equal(tolerations))

			By("deleting IaaDevicePlugin successfully")
			Eventually(func() error {
				f := &devicepluginv1.IaaDevicePlugin{}
				_ = k8sClient.Get(context.Background(), key, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			Eventually(func() error {
				f := &devicepluginv1.IaaDevicePlugin{}
				return k8sClient.Get(context.Background(), key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	It("upgrades", func() {
		dp := &devicepluginv1.IaaDevicePlugin{}

		var image string

		testUpgrade("iaa", dp, &image, nil)

		Expect(dp.Spec.Image == image).To(BeTrue())
	})

	var _ = AfterEach(func() {
		CleanupAfter("iaa", &devicepluginv1.IaaDevicePlugin{})
	})
})
