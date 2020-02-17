package main

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func getModeOverrideFromCluster(nodeName, kubeConfig, master, mode string) (string, error) {
	var config *rest.Config
	var err error

	if len(nodeName) == 0 {
		return mode, fmt.Errorf("node name is not set")
	}

	if len(kubeConfig) == 0 {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags(master, kubeConfig)
	}
	if err != nil {
		return mode, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return mode, err
	}

	node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return mode, err
	}

	if nodeMode, ok := node.ObjectMeta.Annotations["fpga.intel.com/device-plugin-mode"]; ok {
		return nodeMode, nil
	}

	return mode, nil
}
