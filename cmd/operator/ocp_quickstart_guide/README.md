# Intel® Device Plugins Operator for Red Hat OpenShift Container Platform

## Table of Contents
* [Introduction](#introduction)
* [Minimum Hardware Requirements](#minimum-hardware-requirements)
    * [Intel SGX Enabled Server](#intel-sgx-enabled-server)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
    * [Install Operator using OpenShift Web Console](#install-operator-using-openshift-web-console)
    * [Verify Operator installation](#verify-operator-installation)
* [Deploying Intel Device Plugins](#deploying-intel-device-plugins)
    * [Intel SGX Device Plugin](#intel-sgx-device-plugin)

## Introduction
The Intel Device Plugins Operator for OpenShift Container Platform is a collection of device plugins advertising Intel specific hardware resources to the kubelet. It provides a single point of control for Intel® Software Guard Extensions (Intel® SGX), Intel GPUs, Intel® QuickAccess Technology (Intel® QAT), Intel® Data Streaming Accelerator (Intel® DSA), and Intel® In-Memory Analytics Accelerator (Intel® IAA) devices to cluster administrators. The [`v0.24.0`](https://github.com/intel/intel-device-plugins-for-kubernetes/releases/tag/v0.24.0) release of the operator only supports Intel SGX and Intel QAT device plugins. GPU, Intel DSA, Intel  IAA, and other device plugins will be supported in future releases.

## Minimum Hardware Requirements
### Intel SGX Enabled Server
- Third Generation Intel® Xeon® Scalable Platform, code-named “Ice Lake” or later
- Configure BIOS using below details
    ![SGX Server BIOS](images/SGX-BIOS.PNG)
    [**Note:** The BIOS configuration shown above is just for the reference. Please contact your BIOS vendor for details]

## Installation
### Prerequisites
- Make sure Red Hat OpenShift Cluster is ready to use and the developing machine is RHEL and `oc` command is installed and configured properly. Please note that the following operation is verified on Red Hat OpenShift Cluster 4.11 and working machine RHEL-8.6
- Install the `oc` command to your development machine
- Follow the [link](https://docs.openshift.com/container-platform/4.11/hardware_enablement/psap-node-feature-discovery-operator.html) to install **NFD operator** (if it's not already installed).   
    **Note:** Please only install the NFD operator and use steps below to create the NodeFeatureDiscovery instance.  
    - Create the NodeFeatureDiscovery instance  
    ```
    $ oc apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/v0.24.0/deployments/nfd/overlays/node-feature-discovery/node-feature-discovery-openshift.yaml
    ```
    - Create the NodeFeatureRule instance  
    ``` 
    $ oc apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/v0.24.0/deployments/nfd/overlays/node-feature-rules/node-feature-rules-openshift.yaml
    ```
- Deploy SELinux Policy for OCP 4.10 -   
    The SGX device plugin and Init container run as a label `container_device_plugin_t` and `container_device_plugin_init_t` respectively. This requires a custom SELinux policy to be deployed before the SGX plugin can be run. To deploy this policy, run 
    ```
    $ oc apply -f https://raw.githubusercontent.com/intel/user-container-selinux/main/policy-deployment.yaml
    ```  

### Install Operator using OpenShift Web Console
1.  In OpenShift web console navigate to **Operator** -> **OperatorHub**
2.  Search for **Intel Device Plugins Operator ->** Click **Install**  
<img src="images/operator.PNG" width="300" height="200">

### Verify Operator installation
1.  Go to **Operator** -> **Installed Operators**
2.  Verify the status of operator as **Succeeded**
3.  Click **Intel Device Plugins Operator** to view the details  
    ![Verify Operator](images/verify-operator.PNG)


## Deploying Intel Device Plugins

### Intel SGX Device Plugin
Follow the steps below to deploy Intel SGX Device Plugin Custom Resource
1.	Go to **Operator** -> **Installed Operators**
2.  Open **Intel Device Plugins Operator**
3.  Navigate to tab **Intel Software Guard Extensions Device Plugin**
4.  Click **Create SgxDevicePlugin ->** set correct parameters -> Click **Create** 
    OR for any customizations, please select `YAML view` and edit details. Once done, click **Create**
5.  Verify CR by checking the status of DaemonSet **`intel-sgx-plugin`**
6.  Now `SgxDevicePlugin` is ready to deploy any workloads
