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

package e2e

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
)

const (
	// keeps downloaded content which doesn't change between test runs
	cacheDir          = "/tmp/intel-device-plugins-for-kubernetes-test-cache"
	kindBinaryPath    = cacheDir + "/kind"
	kindURL           = "https://github.com/kubernetes-sigs/kind/releases/download/v0.6.1/kind-linux-amd64"
	kindHash          = "386ef80ef8e1baeeca1fb8a8200736ff631958f5aaf9115383abfba973c76f5b"
	kubectlBinaryPath = cacheDir + "/kubectl"
	kubectlURL        = "https://storage.googleapis.com/kubernetes-release/release/v1.17.0/bin/linux/amd64/kubectl"
	kubectlHash       = "6e0aaaffe5507a44ec6b1b8a0fb585285813b78cc045f8804e70a6aac9d1cb4c"
	clusterName       = "intel-plugins-testing"
	kubeConfigPath    = cacheDir + "/kube.config"
	k8sImage          = "kindest/node:v1.17.0"
	webhookImageName  = "docker.io/intel/intel-fpga-admissionwebhook:devel"
	webhookImagePath  = cacheDir + "/intel-fpga-admissionwebhook-devel.tgz"
	samplePod         = `
apiVersion: v1
kind: Pod
metadata:
  name: sample-pod
  labels:
    app: sample
spec:
  containers:
  - name: sample-container
    image: busybox
    command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 3600']
    resources:
      requests:
        fpga.intel.com/arria10.dcp1.0-nlb0: 1
      limits:
        fpga.intel.com/arria10.dcp1.0-nlb0: 1

`
)

func setUp(t *testing.T) {
	info, err := os.Stat(cacheDir)
	if err != nil || info == nil || !info.IsDir() {
		os.RemoveAll(cacheDir)
		os.MkdirAll(cacheDir, 0755)
		t.Log("Downloading and caching required executables...")

		for _, tuple := range [][]string{{kubectlBinaryPath, kubectlURL, kubectlHash}, {kindBinaryPath, kindURL, kindHash}} {
			if err := fetchExecutable(tuple[0], tuple[1]); err != nil {
				t.Fatalf("unable to fetch executable: %+v", err)
			}
			if err := checkHash(tuple[0], tuple[2]); err != nil {
				t.Fatalf("unable to verify downloaded blob: %+v", err)
			}
		}
		t.Log("Done.")
	}
	os.RemoveAll(webhookImagePath)
	runCommand(t, "podman", "save", webhookImageName, "-o", webhookImagePath)
	runCommand(t, kindBinaryPath, "create", "cluster", "--name", clusterName, "--kubeconfig", kubeConfigPath, "--image", k8sImage)
	t.Log("Waiting for all nodes to be ready...")
	runCommand(t, kubectlBinaryPath, "wait", "--for=condition=Ready", "--all", "-l", "kubernetes.io/os=linux", "--timeout=120s", "node")
	t.Log("All nodes are ready. Loading webhook image...")
	// Load webhook image to the cluster
	runCommand(t, kindBinaryPath, "load", "image-archive", "--name", clusterName, webhookImagePath)
}

func tearDown(t *testing.T) {
	runCommand(t, kindBinaryPath, "delete", "cluster", "--name", clusterName)
}

func TestPreprogrammed(t *testing.T) {
	defer tearDown(t)
	setUp(t)

	webhookDeployPath, err := locateWebhookDeployerScript()
	if err != nil {
		t.Fatalf("unable to locate webhook-deploy.sh")
	}

	runCommand(t, webhookDeployPath, "--kubectl", kubectlBinaryPath)
	checkPodMutation(t, "fpga.intel.com/af-d8424dc4a4a3c413f89e433683f9040b")
}

func TestOrchestrated(t *testing.T) {
	defer tearDown(t)
	setUp(t)

	webhookDeployPath, err := locateWebhookDeployerScript()
	if err != nil {
		t.Fatalf("unable to locate webhook-deploy.sh")
	}

	runCommand(t, webhookDeployPath, "--kubectl", kubectlBinaryPath, "--mode", "orchestrated")
	checkPodMutation(t, "fpga.intel.com/region-ce48969398f05f33946d560708be108a")
}

func checkPodMutation(t *testing.T, expectedMutation string) {
	podYamlFileName, err := getTempPodYaml()
	if err != nil {
		t.Fatalf("%+v", err)
	}
	defer os.RemoveAll(podYamlFileName)

	runCommand(t, kubectlBinaryPath, "wait", "--for=condition=Available", "-A", "-l", "app=intel-fpga-webhook", "--timeout=30s", "deployment")
	runCommand(t, kubectlBinaryPath, "apply", "-f", podYamlFileName)

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		cmd := exec.Command(kubectlBinaryPath, "get", "pod", "sample-pod", "-o", "jsonpath='{.spec.containers[0].resources.limits}'")
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfigPath)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("unable to check sample pod: %+v", err)
		}
		t.Log(string(output))
		if bytes.Contains(output, []byte(expectedMutation)) {
			return
		}
	}

	t.Error("sample pod hasn't been mutated")
}

func getTempPodYaml() (string, error) {
	podYamlFile, err := ioutil.TempFile(cacheDir, "sample-pod")
	if err != nil {
		return "", errors.Wrap(err, "unable to create Pod config file")
	}

	if _, err := podYamlFile.Write([]byte(samplePod)); err != nil {
		return "", errors.Wrap(err, "unable to write Pod config file")
	}

	if err := podYamlFile.Close(); err != nil {
		return "", errors.Wrap(err, "unable to close Pod config file")
	}

	return podYamlFile.Name(), nil
}

func runCommand(t *testing.T, name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run %q: %+v", name+" "+strings.Join(arg, " "), err)
	}
}

func locateWebhookDeployerScript() (string, error) {
	root := os.Getenv("PLUGINS_REPO_DIR")
	if root != "" {
		path := filepath.Join(root, "scripts/webhook-deploy.sh")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return path, nil
		}
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	path := filepath.Join(currentDir, "scripts/webhook-deploy.sh")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, nil
	}
	path = filepath.Join(currentDir, "../../scripts/webhook-deploy.sh")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, err
	}

	return "", errors.New("no script found, try to define PLUGINS_REPO_DIR pointing to the root of the repository")
}

func fetchExecutable(path, url string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0770)
	if err != nil {
		return errors.Wrapf(err, "unable to open file %q", path)
	}
	defer f.Close()
	if err := downloadFromURL(url, f); err != nil {
		return errors.Wrapf(err, "unable to download %q", url)
	}
	return nil
}

func downloadFromURL(url string, f *os.File) error {
	client := http.Client{
		Timeout: time.Duration(60 * time.Second),
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer f.Sync()
	_, err = io.Copy(f, resp.Body)
	return err
}

func checkHash(path, hash string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "unable to open %q for reading", path)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return errors.Wrapf(err, "unable to calculate hash for %q", path)
	}

	if hash != fmt.Sprintf("%x", h.Sum(nil)) {
		return errors.Errorf("hash mismatch for %q. Expected %s, but got %x", path, hash, h.Sum(nil))
	}

	return nil
}
