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

// Package utils contails utilities useful in the context of E2E tests.
package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gcustom"
	"github.com/onsi/gomega/types"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

// podLogs returns the log of the container, or the error message if the
// log cannot be fetched.
func podLogs(ctx context.Context, c clientset.Interface, namespace, podName, containerName string) string {
	log, err := e2epod.GetPodLogs(ctx, c, namespace, podName, containerName)
	if err != nil {
		return fmt.Sprintf("unable to get log from pod: %v", err)
	}

	return fmt.Sprintf("log output of the container %s in the pod %s:%s", containerName, podName, log)
}

// HaveSucceeded matches a Pod whose phase is Succeeded. When used with
// gomega.Eventually, polling stops early if the Pod has failed or can never
// terminate because of its restart policy.
func HaveSucceeded() types.GomegaMatcher {
	return gcustom.MakeMatcher(func(pod *v1.Pod) (bool, error) {
		if pod.DeletionTimestamp == nil && pod.Spec.RestartPolicy == v1.RestartPolicyAlways {
			return false, gomega.StopTrying("pod will never terminate with a succeeded state since its restart policy is Always")
		}

		switch pod.Status.Phase {
		case v1.PodSucceeded:
			return true, nil
		case v1.PodFailed:
			return false, gomega.StopTrying("pod failed")
		default:
			return false, nil
		}
	}).WithTemplate("Expected Pod {{.To}} succeed\nGot instead:\n{{.FormattedActual}}")
}

// WaitForPodSuccess waits up to timeout for the Pod to succeed. If it fails,
// never terminates or does not finish in time, the test fails and the log of
// containerName is attached to the failure message.
func WaitForPodSuccess(ctx context.Context, c clientset.Interface, namespace, podName, containerName string, timeout time.Duration) {
	ginkgo.GinkgoHelper()

	pod := framework.NamespacedName{Namespace: namespace, Name: podName}

	err := framework.Gomega().Eventually(ctx, e2epod.Get(c, pod)).WithTimeout(timeout).Should(HaveSucceeded())
	if err != nil {
		framework.ExpectNoError(err, "%s", podLogs(ctx, c, namespace, podName, containerName))
	}
}

// AllocatableResource returns a function that sums the allocatable quantity
// of res across all nodes. It is meant to be polled with gomega.Eventually:
//
//	gomega.Eventually(ctx, utils.AllocatableResource(c, res)).WithTimeout(t).Should(gomega.BeNumerically(">", 0))
func AllocatableResource(c clientset.Interface, res v1.ResourceName) func(ctx context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		nodelist, err := c.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return 0, err
		}

		var sum int64

		for _, item := range nodelist.Items {
			if q, ok := item.Status.Allocatable[res]; ok {
				sum += q.Value()
			}
		}

		framework.Logf("Found %d of allocatable %q", sum, res)

		return sum, nil
	}
}

// ApplyPluginAndWait applies a plugin kustomization and waits for its Pod to
// become ready.
func ApplyPluginAndWait(ctx context.Context, f *framework.Framework, kustomizationPath, appLabel string, timeout time.Duration) []v1.Pod {
	ginkgo.GinkgoHelper()

	e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", filepath.Dir(kustomizationPath))

	ginkgo.By("waiting for plugin availability")
	podList, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
		labels.Set{"app": appLabel}.AsSelector(), 1 /* one replica */, timeout)
	if err != nil {
		e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
		e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "unable to wait for all pods to be running and ready")

	return podList.Items
}

// DeletePluginAndWait deletes a plugin kustomization and waits for its Pod to
// disappear.
func DeletePluginAndWait(ctx context.Context, f *framework.Framework, kustomizationPath, podName string, timeout time.Duration) {
	ginkgo.GinkgoHelper()

	e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "delete", "-k", filepath.Dir(kustomizationPath))
	gomega.Expect(e2epod.WaitForPodNotFoundInNamespace(
		ctx, f.ClientSet, podName, f.Namespace.Name, timeout,
	)).To(gomega.Succeed(), "failed to terminate pod")
}

// MustLocateRepoFile returns the absolute path of a file inside this repository
// and fails the test if the file does not exist. The repository root comes from
// the e2e framework's --repo-root flag (framework.TestContext.RepoRoot).
func MustLocateRepoFile(repopath string) string {
	path, err := filepath.Abs(filepath.Join(framework.TestContext.RepoRoot, repopath))
	if err != nil {
		framework.Failf("unable to resolve %q: %v", repopath, err)
	}

	if _, err := os.Stat(path); err != nil {
		framework.Failf("unable to locate %q: %v (check the --repo-root flag)", repopath, err)
	}

	return path
}

func copyFiles(srcDir, dstDir string) error {
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if d.IsDir() || err != nil {
			return nil
		}

		//nolint:gosec // The source is trusted test data from the repository.
		n, err := os.ReadFile(path)
		if err != nil && err != io.EOF || len(n) == 0 {
			return err
		}

		fn := filepath.Join(dstDir, filepath.Base(path))

		//nolint:gosec // The destination is constrained to the overlay directory and a base filename.
		if err := os.WriteFile(fn, n, 0600); err != nil {
			return err
		}

		return nil
	})

	return err
}

// CreateKustomizationOverlay copies the base overlay, and changes the namespace
// and relative paths to resources. The deletion of the files is left for the caller.
func CreateKustomizationOverlay(namespace, kustomizeYamlFileDir, overlayDir string) error {
	relPath, err := filepath.Rel(overlayDir, kustomizeYamlFileDir)
	if err != nil {
		return err
	}

	// Copy all files under the kustomize path under the temp overlay path.
	err = copyFiles(kustomizeYamlFileDir, overlayDir)
	if err != nil {
		return err
	}

	kustomizationFile := filepath.Join(overlayDir, "kustomization.yaml")

	bytes, err := os.ReadFile(kustomizationFile)
	if err != nil {
		return err
	}

	content := make(map[string]any)

	err = yaml.Unmarshal(bytes, content)
	if err != nil {
		return err
	}

	content["namespace"] = namespace

	resInterface := content["resources"].([]any)
	resources := make([]string, len(resInterface))

	for i, v := range resInterface {
		resources[i] = v.(string)
	}

	// Add relative path for directories. Leave local (.yaml) files as they are.
	for i, res := range resources {
		if !strings.HasSuffix(res, ".yaml") {
			resources[i] = relPath + "/" + res
		}
	}

	content["resources"] = resources

	bytes, err = yaml.Marshal(content)
	if err != nil {
		return err
	}

	if err := os.WriteFile(kustomizationFile, bytes, 0600); err != nil {
		return err
	}

	return nil
}

// DeployWebhook deploys an admission webhook to a framework-specific namespace.
func DeployWebhook(ctx context.Context, f *framework.Framework, kustomizationPath string) v1.Pod {
	if _, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, "cert-manager",
		labels.Set{"app.kubernetes.io/name": "cert-manager"}.AsSelector(), 1 /* one replica */, 10*time.Second); err != nil {
		framework.Failf("unable to detect running cert-manager: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "webhooke2etest-"+f.Namespace.Name)
	if err != nil {
		framework.Failf("unable to create temp directory: %v", err)
	}

	defer os.RemoveAll(tmpDir)

	// The overlay files are deleted by the deferred RemoveAll call above.
	err = CreateKustomizationOverlay(f.Namespace.Name, filepath.Dir(kustomizationPath), tmpDir)
	if err != nil {
		framework.Failf("unable to kustomization overlay: %v", err)
	}

	e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", tmpDir)

	podList, err := e2epod.WaitForPodsWithLabelRunningReady(ctx, f.ClientSet, f.Namespace.Name,
		labels.Set{"control-plane": "controller-manager"}.AsSelector(), 1 /* one replica */, 60*time.Second)
	if err != nil {
		e2edebug.DumpAllNamespaceInfo(ctx, f.ClientSet, f.Namespace.Name)
		e2ekubectl.LogFailedContainers(ctx, f.ClientSet, f.Namespace.Name, framework.Logf)
		framework.Failf("unable to wait for all pods to be running and ready: %v", err)
	}

	// Wait for the webhook to initialize
	time.Sleep(2 * time.Second)

	return podList.Items[0]
}

// TestContainersRunAsNonRoot checks that all containers within the Pods run
// with non-root UID/GID.
func TestContainersRunAsNonRoot(pods []v1.Pod) error {
	for _, p := range pods {
		for _, c := range append(p.Spec.InitContainers, p.Spec.Containers...) {
			if c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
				return fmt.Errorf("%s (container: %s): RunAsNonRoot is not true", p.Name, c.Name)
			}

			if c.SecurityContext.RunAsGroup == nil || *c.SecurityContext.RunAsGroup == 0 {
				return fmt.Errorf("%s (container: %s): RunAsGroup is root (0)", p.Name, c.Name)
			}

			if c.SecurityContext.RunAsUser == nil || *c.SecurityContext.RunAsUser == 0 {
				return fmt.Errorf("%s (container: %s): RunAsUser is root (0)", p.Name, c.Name)
			}
		}
	}

	return nil
}

func printVolumeMounts(vm []v1.VolumeMount) {
	for _, v := range vm {
		if !v.ReadOnly {
			framework.Logf("Available RW volume mounts: %v", v)
		}
	}
}

// TestPodsFileSystemInfo checks that all containers within the Pods run
// with ReadOnlyRootFileSystem. It also prints RW volume mounts.
func TestPodsFileSystemInfo(pods []v1.Pod) error {
	for _, p := range pods {
		for _, c := range append(p.Spec.InitContainers, p.Spec.Containers...) {
			if c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
				return fmt.Errorf("%s (container: %s): Writable root filesystem", p.Name, c.Name)
			}

			printVolumeMounts(c.VolumeMounts)
		}
	}

	return nil
}

func TestWebhookServerTLS(ctx context.Context, f *framework.Framework, serviceName string) error {
	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testssl-tester",
			Namespace: f.Namespace.Name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Args: []string{
						"--openssl=/usr/bin/openssl",
						"--mapping",
						"iana",
						"-s",
						"-f",
						"-p",
						"-P",
						"-U",
						serviceName},
					Name:            "testssl-container",
					Image:           "drwetter/testssl.sh:3.0",
					ImagePullPolicy: "IfNotPresent",
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	_, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podSpec, metav1.CreateOptions{})
	framework.ExpectNoError(err, "pod Create API error")

	waitErr := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, "testssl-tester", f.Namespace.Name, 180*time.Second)

	output, err := e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, "testssl-tester", "testssl-container")
	if err != nil {
		return fmt.Errorf("failed to get output for testssl.sh run: %w", err)
	}

	framework.Logf("testssl.sh output:\n %s", output)

	if waitErr != nil {
		return fmt.Errorf("testssl.sh run did not succeed: %w", waitErr)
	}

	return nil
}

func Kubectl(ns string, cmd string, opt string, file string) {
	path := MustLocateRepoFile(file)

	if opt == "-k" {
		path = filepath.Dir(path)
	}

	msg := e2ekubectl.RunKubectlOrDie(ns, cmd, opt, path)
	framework.Logf("%s", msg)
}

func FindNodeAndResourceCapacity(f *framework.Framework, ctx context.Context, resourceName string) (string, int64) {
	nodelist, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		framework.Failf("failed to list Nodes: %v", err)
	}

	// we have at least one node with resource capacity
	for _, item := range nodelist.Items {
		if q, ok := item.Status.Allocatable[v1.ResourceName(resourceName)]; ok && q.Value() > 0 {
			return item.Name, q.Value()
		}
	}

	return "", 0
}
