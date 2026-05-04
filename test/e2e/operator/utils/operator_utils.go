// Copyright 2026 Intel Corporation. All Rights Reserved.
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

// Package operutils provides helper functions for operator-specific e2e tests.
package operutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// pluginVersion is the default image tag used when building image references.
// These variables allow defining custom container paths: <1:image registry>/<2:image path>/container:<3:plugin version>
// e.g. my-registry.com/project-x/intel-deviceplugin-operator:1.2.3
//         ^^^^1^^^^    ^^^^2^^^^                             ^^3^^
// Project namespace is the cluster namespace where the operator is deployed
// For OCP, project namespace and image path have to be same to allow pulling from the internal registry.

var pluginVersion = "0.35.0"
var projectNamespace = "inteldeviceplugins-system"
var imagePath = "inteldeviceplugins-system"
var imageRegistry = "image-registry.openshift-image-registry.svc:5000"

func init() {
	// Allow overriding the default plugin version with the PLUGIN_VERSION
	if version := os.Getenv("PLUGIN_VERSION"); version != "" {
		pluginVersion = version
	}
	if ns := os.Getenv("PROJECT_NAMESPACE"); ns != "" {
		projectNamespace = ns
	}
	if registry := os.Getenv("IMAGE_REGISTRY"); registry != "" {
		imageRegistry = registry
	}
	if imgPath := os.Getenv("IMAGE_PATH"); imgPath != "" {
		imagePath = imgPath
	}
}

func PluginVersion() string {
	return pluginVersion
}

func ProjectNamespace() string {
	return projectNamespace
}

func ImagePath() string {
	return imagePath
}

func ImageRegistry() string {
	return imageRegistry
}

// PluginImageName returns the fully-qualified container image reference for
// the given image name.
// When IMAGE_PATH is set, the reference points to the internal OCP
// registry so that pods inside the cluster can pull without external TLS
// concerns.  Otherwise, it points to docker.io/intel.
func PluginImageName(name string) string {
	return fmt.Sprintf("%s/%s/%s:%s", ImageRegistry(), ImagePath(), name, PluginVersion())
}

// CreateKustomizationOverlay writes a minimal kustomization.yaml into
// tempDir that wraps the provided ocpOverlayDir.  When IMAGE_PATH is
// set, image transformers redirect all upstream docker.io/intel/ images to the
// internal OCP registry so that pods pull from within the cluster.
//
// The caller is responsible for removing tempDir after use.
func CreateKustomizationOverlay(ocpOverlayDir, tempDir, version, namespace string) error {
	absOverlay, err := filepath.Abs(ocpOverlayDir)
	if err != nil {
		return err
	}

	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		return err
	}

	// Kustomize forbids absolute paths in resources; use a relative path from
	// tempDir to the overlay directory instead.
	relOverlay, err := filepath.Rel(absTempDir, absOverlay)
	if err != nil {
		return err
	}

	content := map[string]any{
		"resources": []string{relOverlay},
		"namespace": namespace,
		"images":    buildImageOverrides(ImageRegistry()+"/"+ImagePath(), version),
	}

	bytes, err := yaml.Marshal(content)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tempDir, "kustomization.yaml"), bytes, 0o600)
}

func buildImageOverrides(registry, version string) []map[string]string {
	containers := []string{
		"intel-deviceplugin-operator",
		"crypto-perf",
		"dsa-dpdk-dmadevtest",
	}

	overrides := make([]map[string]string, 0, len(containers)*2)
	for _, container := range containers {
		overrides = append(overrides, []map[string]string{
			{
				"name":    "intel/" + container,
				"newName": registry + "/" + container,
				"newTag":  version,
			},
			{
				"name":    "docker.io/intel/" + container,
				"newName": registry + "/" + container,
				"newTag":  version,
			},
		}...)
	}

	return overrides
}

func IsRunningOnOCP(ctx context.Context, c clientset.Interface) bool {
	_, err := c.CoreV1().Namespaces().Get(ctx, "openshift-apiserver", metav1.GetOptions{})
	return err == nil
}

func CreateWorkloadKustomizationFromDir(originalDir, version string) (string, error) {
	tempDir, err := os.MkdirTemp("", "workloaddir-kustomization")
	if err != nil {
		return "", err
	}

	// Copy originalDir into tempDir
	err = filepath.Walk(originalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		fbytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		destPath := filepath.Join(tempDir, filepath.Base(path))
		writeErr := os.WriteFile(destPath, fbytes, 0600)
		if writeErr != nil {
			return writeErr
		}

		return nil
	})

	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	kustomizationObj := map[string]any{}

	kustomizationFile := filepath.Join(tempDir, "kustomization.yaml")
	kBytes, err := os.ReadFile(kustomizationFile)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}
	if err = yaml.Unmarshal(kBytes, &kustomizationObj); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	kustomizationObj["images"] = buildImageOverrides(ImageRegistry()+"/"+ImagePath(), version)

	newKBytes, err := yaml.Marshal(kustomizationObj)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	if err = os.WriteFile(kustomizationFile, newKBytes, 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return tempDir, nil
}

func CreateWorkloadKustomizationFromFile(originalFile, version string) (string, error) {
	tempDir, err := os.MkdirTemp("", "workload-kustomization")
	if err != nil {
		return "", err
	}

	// Copy original file from originalFile into tempDir
	originalBytes, err := os.ReadFile(originalFile)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	destFile := filepath.Join(tempDir, filepath.Base(originalFile))
	if err = os.WriteFile(destFile, originalBytes, 0o600); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	content := map[string]any{
		"resources": []string{destFile},
		"images":    buildImageOverrides(ImageRegistry()+"/"+ImagePath(), version),
	}

	bytes, err := yaml.Marshal(content)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	if err = os.WriteFile(filepath.Join(tempDir, "kustomization.yaml"), bytes, 0o600); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return tempDir, nil
}
