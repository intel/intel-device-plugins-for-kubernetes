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

var _ = Describe("GpuDevicePlugin Controller", func() {

	const timeout = time.Second * 5
	const interval = time.Millisecond * 100

	Context("Basic CRUD operations", func() {
		It("should handle GpuDevicePlugin objects correctly", func() {
			spec := devicepluginv1.GpuDevicePluginSpec{
				Image:        "gpu-testimage",
				NodeSelector: map[string]string{"gpu-nodeselector": "true"},
				SharedDevNum: 5,
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

			expectedDsName := "intel-gpu-plugin-gpudeviceplugin-test"

			expectArgs := []string{
				"-v",
				"0",
				"-shared-dev-num",
				"5",
				"-allocation-policy",
				"none",
			}

			By("creating GpuDevicePlugin successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetched := &devicepluginv1.GpuDevicePlugin{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetched)
				return len(fetched.Status.ControlledDaemonSet.UID) > 0
			}, timeout, interval).Should(BeTrue())

			By("checking DaemonSet is created successfully")
			ds := &apps.DaemonSet{}

			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
			}, timeout, interval).Should(BeNil())

			err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)

			dsGen := ds.Generation

			Expect(err).To(BeNil())
			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(BeEquivalentTo(expectArgs))
			Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(spec.Image))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(0))
			Expect(ds.Spec.Template.Spec.NodeSelector).To(Equal(spec.NodeSelector))
			Expect(ds.Spec.Template.Spec.Tolerations).To(HaveLen(0))

			By("updating GpuDevicePlugin successfully")
			updatedImage := "updated-gpu-testimage"
			updatedInitImage := "updated-gpu-testinitimage"
			updatedLogLevel := 2
			updatedSharedDevNum := 1
			updatedNodeSelector := map[string]string{"updated-gpu-nodeselector": "true"}

			fetched.Spec.Image = updatedImage
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.LogLevel = updatedLogLevel
			fetched.Spec.SharedDevNum = updatedSharedDevNum
			fetched.Spec.NodeSelector = updatedNodeSelector
			fetched.Spec.VFIOMode = true

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())
			fetchedUpdated := &devicepluginv1.GpuDevicePlugin{}
			Eventually(func() devicepluginv1.GpuDevicePluginSpec {
				_ = k8sClient.Get(context.Background(), key, fetchedUpdated)
				return fetchedUpdated.Spec
			}, timeout, interval).Should(Equal(fetched.Spec))

			By("checking DaemonSet is updated successfully")
			// Wait for DS to update to the next generation
			Eventually(func() int64 {
				_ = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
				return ds.Generation
			}, timeout, interval).Should(BeNumerically(">", dsGen))

			expectArgs = []string{
				"-v",
				strconv.Itoa(updatedLogLevel),
				"-shared-dev-num",
				strconv.Itoa(updatedSharedDevNum),
				"-allocation-policy",
				"none",
				"-run-mode=vfio",
			}

			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(BeEquivalentTo(expectArgs))
			Expect(ds.Spec.Template.Spec.Containers[0].Image).Should(Equal(updatedImage))
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(Equal(updatedInitImage))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(Equal(updatedNodeSelector))

			dsGen = ds.Generation

			By("updating GpuDevicePlugin with different values successfully")
			updatedInitImage = ""
			updatedNodeSelector = map[string]string{}
			fetched.Spec.InitImage = updatedInitImage
			fetched.Spec.NodeSelector = updatedNodeSelector

			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())

			By("checking DaemonSet is updated with different values successfully")
			// Wait for DS to update to the next generation
			Eventually(func() int64 {
				_ = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
				return ds.Generation
			}, timeout, interval).Should(BeNumerically(">", dsGen))

			dsGen = ds.Generation

			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(0))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(HaveLen(1))
			Expect(ds.Spec.Template.Spec.NodeSelector).Should(And(HaveLen(1), HaveKeyWithValue("kubernetes.io/arch", "amd64")))

			By("updating GPUDevicePlugin with tolerations")

			tolerations := []corev1.Toleration{
				{Key: "foo", Operator: "Equal", Value: "bar", Effect: "NoSchedule"},
			}

			fetched.Spec.Tolerations = tolerations
			Expect(k8sClient.Update(context.Background(), fetched)).Should(Succeed())

			By("checking DaemonSet is updated with tolerations")
			// Wait for DS to update to the next generation
			Eventually(func() int64 {
				_ = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: expectedDsName}, ds)
				return ds.Generation
			}, timeout, interval).Should(BeNumerically(">", dsGen))

			Expect(ds.Spec.Template.Spec.Tolerations).Should(BeEquivalentTo(tolerations))

			// Not really needed, but if extra steps are added later...
			//nolint:ineffassign,staticcheck
			dsGen = ds.Generation

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

	It("upgrades", func() {
		dp := &devicepluginv1.GpuDevicePlugin{}

		var image string

		testUpgrade("gpu", dp, &image, nil)

		Expect(dp.Spec.Image == image).To(BeTrue())
	})

	var _ = AfterEach(func() {
		CleanupAfter("gpu", &devicepluginv1.GpuDevicePlugin{})
	})
})
