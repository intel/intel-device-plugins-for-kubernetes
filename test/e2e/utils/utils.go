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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	poll = time.Second
)

// WaitForNodesWithResource waits for nodes to have positive allocatable resource.
func WaitForNodesWithResource(c clientset.Interface, res v1.ResourceName, timeout time.Duration) error {
	framework.Logf("Waiting up to %s for any positive allocatable resource %q", timeout, res)

	start := time.Now()

	err := wait.Poll(poll, timeout,
		func() (bool, error) {
			for t := time.Now(); time.Since(t) < timeout; time.Sleep(poll) {
				nodelist, err := c.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}

				resNum := 0
				for _, item := range nodelist.Items {
					if q, ok := item.Status.Allocatable[res]; ok {
						resNum = resNum + int(q.Value())
					}
				}
				framework.Logf("Found %d of %q. Elapsed: %s", resNum, res, time.Since(start))
				if resNum > 0 {
					return true, nil
				}
			}

			return false, errors.New("unable to list nodes")
		})

	return err
}

// WaitForPodFailure waits for a pod to fail.
// This function used to be a part of k8s e2e framework, but was deleted in
// https://github.com/kubernetes/kubernetes/pull/86732.
func WaitForPodFailure(f *framework.Framework, name string, timeout time.Duration) {
	gomega.Expect(e2epod.WaitForPodCondition(f.ClientSet, f.Namespace.Name, name, "success or failure", timeout,
		func(pod *v1.Pod) (bool, error) {
			switch pod.Status.Phase {
			case v1.PodFailed:
				return true, nil
			case v1.PodSucceeded:
				return true, errors.Errorf("pod %q successed with reason: %q, message: %q", name, pod.Status.Reason, pod.Status.Message)
			default:
				return false, nil
			}
		},
	)).To(gomega.Succeed(), "wait for pod %q to fail", name)
}

// LocateRepoFile locates a file inside this repository.
func LocateRepoFile(repopath string) (string, error) {
	root := os.Getenv("PLUGINS_REPO_DIR")
	if root != "" {
		path := filepath.Join(root, repopath)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return path, nil
		}
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	path := filepath.Join(currentDir, repopath)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, nil
	}

	path = filepath.Join(currentDir, "../../"+repopath)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, err
	}

	return "", errors.New("no file found, try to define PLUGINS_REPO_DIR pointing to the root of the repository")
}

// CreateKustomizationOverlay creates an overlay with overridden namespace.
func CreateKustomizationOverlay(namespace, base, overlay string) error {
	relPath := ""
	for range strings.Split(overlay[1:], "/") {
		relPath = relPath + "../"
	}

	relPath = relPath + base[1:]
	content := fmt.Sprintf("namespace: %s\nbases:\n  - %s", namespace, relPath)

	return os.WriteFile(overlay+"/kustomization.yaml", []byte(content), 0600)
}

// DeployWebhook deploys an admission webhook to a framework-specific namespace.
func DeployWebhook(f *framework.Framework, kustomizationPath string) v1.Pod {
	if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, "cert-manager",
		labels.Set{"app.kubernetes.io/name": "cert-manager"}.AsSelector(), 1 /* one replica */, 10*time.Second); err != nil {
		framework.Failf("unable to detect running cert-manager: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "webhooke2etest-"+f.Namespace.Name)
	if err != nil {
		framework.Failf("unable to create temp directory: %v", err)
	}

	defer os.RemoveAll(tmpDir)

	err = CreateKustomizationOverlay(f.Namespace.Name, filepath.Dir(kustomizationPath), tmpDir)
	if err != nil {
		framework.Failf("unable to kustomization overlay: %v", err)
	}

	e2ekubectl.RunKubectlOrDie(f.Namespace.Name, "apply", "-k", tmpDir)

	podList, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
		labels.Set{"control-plane": "controller-manager"}.AsSelector(), 1 /* one replica */, 60*time.Second)
	if err != nil {
		e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
		e2ekubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)
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
				return errors.Errorf("%s (container: %s): RunAsNonRoot is not true", p.Name, c.Name)
			}

			if c.SecurityContext.RunAsGroup == nil || *c.SecurityContext.RunAsGroup == 0 {
				return errors.Errorf("%s (container: %s): RunAsGroup is root (0)", p.Name, c.Name)
			}

			if c.SecurityContext.RunAsUser == nil || *c.SecurityContext.RunAsUser == 0 {
				return errors.Errorf("%s (container: %s): RunAsUser is root (0)", p.Name, c.Name)
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
				return errors.Errorf("%s (container: %s): Writable root filesystem", p.Name, c.Name)
			}

			printVolumeMounts(c.VolumeMounts)
		}
	}

	return nil
}

func TestInitcontainersRunIdempotent(f *framework.Framework, podList *v1.PodList, pluginName string) error {
	if err := e2epod.DeletePodWithWaitByName(f.ClientSet, podList.Items[0].Name, f.Namespace.Name); err != nil {
		e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
		return errors.Errorf("unable to wait for the plugin to be terminated: %v", err)
	}

	if _, err := e2epod.WaitForPodsWithLabelRunningReady(f.ClientSet, f.Namespace.Name,
		labels.Set{"app": pluginName}.AsSelector(), 1 /* one replica */, 100*time.Second); err != nil {
		e2edebug.DumpAllNamespaceInfo(f.ClientSet, f.Namespace.Name)
		e2ekubectl.LogFailedContainers(f.ClientSet, f.Namespace.Name, framework.Logf)

		return errors.Errorf("unable to wait for all pods to be running and ready: %v", err)
	}

	return nil
}

func TestWebhookServerTLS(f *framework.Framework, serviceName string) error {
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
					Image:           "drwetter/testssl.sh",
					ImagePullPolicy: "IfNotPresent",
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	_, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), podSpec, metav1.CreateOptions{})
	framework.ExpectNoError(err, "pod Create API error")

	waitErr := e2epod.WaitForPodSuccessInNamespaceTimeout(f.ClientSet, "testssl-tester", f.Namespace.Name, 180*time.Second)

	output, err := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, "testssl-tester", "testssl-container")
	if err != nil {
		return errors.Wrap(err, "failed to get output for testssl.sh run")
	}

	framework.Logf("testssl.sh output:\n %s", output)

	if waitErr != nil {
		return errors.Wrap(err, "testssl.sh run did not succeed")
	}

	return nil
}

func Kubectl(ns string, cmd string, opt string, file string) {
	path, err := LocateRepoFile(file)
	if err != nil {
		framework.Failf("unable to locate %q: %v", file, err)
	}

	if opt == "-k" {
		path = filepath.Dir(path)
	}

	msg := e2ekubectl.RunKubectlOrDie(ns, cmd, opt, path)
	framework.Logf("%s", msg)
}
