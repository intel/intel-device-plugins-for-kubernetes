# Intel Device Plugins for Red Hat OpenShift Container Platform

This document describes how to provision Intel hardware features on a Red Hat
OpenShift Container Platform (RHOCP) cluster using Intel Device Plugins. It
covers hardware and BIOS configuration, node labeling with Node Feature
Discovery, and installing the Intel Device Plugins Operator to expose QAT, SGX,
DSA, and GPU resources to workloads.

> **Note:** Version numbers used in examples throughout this document are for
> reference only. Always check the latest versions of RHOCP, operators, and
> container images before deploying.

## Table of Contents

- [Prerequisites](#prerequisites)
- [BIOS Configuration](#bios-configuration)
- [Machine Configuration](#machine-configuration)
  - [Hugepages](#hugepages)
  - [CPU Manager](#cpu-manager)
- [Node Feature Discovery](#node-feature-discovery)
  - [NodeFeatureRules for Intel Devices](#nodefeaturerules-for-intel-devices)
  - [Verification](#nfd-verification)
  - [Labels Table](#labels-table)
- [Intel Device Plugins Operator](#intel-device-plugins-operator)
  - [Installation via Web Console](#installation-via-web-console)
  - [Installation via CLI](#installation-via-cli)
  - [Verify Installation](#verify-operator-installation)
- [Creating Device Plugin Custom Resources](#creating-device-plugin-custom-resources)
  - [Intel QAT Device Plugin](#intel-qat-device-plugin)
  - [Intel SGX Device Plugin](#intel-sgx-device-plugin)
  - [Intel DSA Device Plugin](#intel-dsa-device-plugin)
  - [Intel GPU Device Plugin](#intel-gpu-device-plugin)
- [Resources Provided by Intel Device Plugins](#resources-provided-by-intel-device-plugins)

## Prerequisites

- A provisioned RHOCP cluster (bare-metal multi-node is recommended). See
  [Red Hat's installation documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/installation_overview/ocp-installation-overview)
  for cluster provisioning instructions.
- `oc` CLI tool installed and configured with cluster-admin privileges.
- Intel hardware present in worker nodes (see [BIOS Configuration](#bios-configuration)
  for per-feature requirements).

## BIOS Configuration

Before provisioning Intel hardware features, ensure the BIOS is configured
correctly on each worker node. Refer to your server vendor's BIOS documentation
for specific instructions. The following are general references:

| Feature | BIOS Configuration Reference |
|---|---|
| Intel® SGX | [Intel SGX BIOS setup](https://www.intel.com/content/www/us/en/support/articles/000087972/server-products/single-node-servers.html) |
| Intel® Data Center GPU Flex Series | [GPU Flex Series setup guide](https://www.intel.com/content/www/us/en/content-details/774119/virtualization-guide-for-intel-data-center-gpu-flex-series.html) |
| Intel® Data Center GPU Max Series | [GPU Max Series BIOS setup](https://dgpu-docs.intel.com/system-user-guides/DNP-Max-1100-userguide/DNP-Max-1100-userguide.html#bios-setup) |
| Intel® QAT | [QAT BIOS setup](https://github.com/intel/qatlib/blob/main/INSTALL) |
| Intel® DSA | [DSA setup guide (PDF)](https://cdrdv2-public.intel.com/759709/353216-data-streaming-accelerator-user-guide-003.pdf) |

## Machine Configuration

The Machine Config Operator (MCO) manages operating system configuration on
RHOCP worker nodes. For Intel QAT and Intel DSA, you must enable the
`intel_iommu` kernel parameter with `sm_on` and load the `vfio-pci` module at
boot.

Apply the following `MachineConfig` to enable IOMMU and VFIO for QAT and DSA:

> **Note:** Applying this MachineConfig will trigger a reboot of the matching
> worker nodes.

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  labels:
    machineconfiguration.openshift.io/role: worker
  name: 100-intel-iommu-on
spec:
  kernelArguments:
    # The vfio-pci.ids below are examples; add any supported QAT VF or DSA PF PCI IDs for your hardware
    - intel_iommu=on,sm_on modules_load=vfio-pci vfio-pci.ids=8086:4941,8086:4943
```

Save the above to a file (e.g. `100-intel-iommu-on.yaml`) and apply:

```bash
oc apply -f 100-intel-iommu-on.yaml
```

### Hugepages

If your workloads require hugepages (e.g. DPDK-based QAT applications), apply
the following `MachineConfig` to allocate 2 Mi hugepages at boot:

> **Note:** Applying this MachineConfig will trigger a reboot of the matching
> worker nodes.

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  name: 99-worker-hugepages-2mi
  labels:
    machineconfiguration.openshift.io/role: worker
spec:
  kernelArguments:
    - "default_hugepagesz=2M"
    - "hugepagesz=2M"
    - "hugepages=1024"
```

Save the above to a file (e.g. `99-worker-hugepages-2mi.yaml`) and apply:

```bash
oc apply -f 99-worker-hugepages-2mi.yaml
```

### CPU Manager

For workloads that require exclusive CPU pinning, enable the static CPU Manager
policy via a `KubeletConfig`:

```yaml
apiVersion: machineconfiguration.openshift.io/v1
kind: KubeletConfig
metadata:
  name: cpumanager-enabled
spec:
  machineConfigPoolSelector:
    matchLabels:
      custom-kubelet: cpumanager-enabled
  kubeletConfig:
     cpuManagerPolicy: static
     cpuManagerReconcilePeriod: 5s
```

Save the above to a file (e.g. `cpumanager-kubeletconfig.yaml`) and apply:

```bash
oc apply -f cpumanager-kubeletconfig.yaml
```

Then label the `MachineConfigPool` to activate the config:

```bash
oc label machineconfigpool worker custom-kubelet=cpumanager-enabled
```

### Verification

After the worker nodes have rebooted, verify the configuration by navigating to
the node terminal on the web console (**Compute** → **Nodes** → select a node →
**Terminal**), or by using `oc debug node/<node-name>`:

```bash
# Check kernel command line
cat /proc/cmdline
# Ensure "intel_iommu=on,sm_on" is present

# Check that vfio_pci driver is loaded
chroot /host
lsmod | grep vfio_pci
```

## Node Feature Discovery

[Node Feature Discovery (NFD)](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/specialized_hardware_and_driver_enablement/psap-node-feature-discovery-operator)
detects hardware features and system configuration on cluster nodes and exposes
them as node labels. These labels are used by the Intel Device Plugins Operator
to schedule device plugin pods on appropriate nodes.

### Installing the NFD Operator

Follow the Red Hat documentation to install the NFD Operator:

- [Install from the CLI](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/specialized_hardware_and_driver_enablement/psap-node-feature-discovery-operator#install-operator-cli_psap-node-feature-discovery-operator)
- [Install from the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/specialized_hardware_and_driver_enablement/psap-node-feature-discovery-operator#install-operator-web-console_psap-node-feature-discovery-operator)

After installing the operator, create a `NodeFeatureDiscovery` CR instance
following the
[Red Hat NFD documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/specialized_hardware_and_driver_enablement/psap-node-feature-discovery-operator#creating-nfd-cr-cli_psap-node-feature-discovery-operator).

### NodeFeatureRules for Intel Devices

Once NFD is running, apply the following `NodeFeatureRule` to label nodes with
Intel hardware features (GPU, QAT, SGX, DSA):

```yaml
apiVersion: nfd.openshift.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: intel-dp-devices
  namespace: openshift-nfd
spec:
  rules:
    - name: "intel.gpu"
      labels:
        "intel.feature.node.kubernetes.io/gpu": "true"
      matchFeatures:
        - feature: pci.device
          matchExpressions:
            vendor: {op: In, value: ["8086"]}
            class: {op: In, value: ["0300", "0380"]}

    - name: "intel.qat"
      labels:
        "intel.feature.node.kubernetes.io/qat": "true"
      matchFeatures:
        - feature: pci.device
          matchExpressions:
            vendor: {op: In, value: ["8086"]}
            device: {op: In, value: ["4940", "4942", "4944"]}
            class: {op: In, value: ["0b40"]}
        - feature: kernel.loadedmodule
          matchExpressions:
            intel_qat: {op: Exists}  # intel_qat module must be loaded

    - name: "intel.sgx"
      labels:
        "intel.feature.node.kubernetes.io/sgx": "true"
      extendedResources:
        sgx.intel.com/epc: "@cpu.security.sgx.epc"
      matchFeatures:
        - feature: cpu.cpuid
          matchExpressions:
            SGX: {op: Exists}
            SGXLC: {op: Exists}
        - feature: cpu.security
          matchExpressions:
            sgx.enabled: {op: IsTrue}
        - feature: kernel.config
          matchExpressions:
            X86_SGX: {op: Exists}

    - name: "intel.dsa"
      labels:
        "intel.feature.node.kubernetes.io/dsa": "true"
      matchFeatures:
        - feature: pci.device
          matchExpressions:
            vendor: {op: In, value: ["8086"]}
            device: {op: In, value: ["0b25"]}
            class: {op: In, value: ["0880"]}
        - feature: kernel.loadedmodule
          matchExpressions:
            idxd: {op: Exists}  # idxd module must be loaded
```

Save the above to a file (e.g. `node-feature-rules.yaml`) and apply:

```bash
oc apply -f node-feature-rules.yaml
```

> **Note:** The QAT rule requires the `intel_qat` kernel module and the DSA rule
> requires the `idxd` kernel module to be loaded on the nodes. These modules are
> typically loaded automatically when the corresponding hardware is present and
> properly configured.

### NFD Verification

Use the following commands to verify that nodes are labeled correctly:

```bash
oc get nodes
oc describe node <node-name> | grep intel.feature.node.kubernetes.io
```

Example output:

```
intel.feature.node.kubernetes.io/gpu=true
intel.feature.node.kubernetes.io/qat=true
intel.feature.node.kubernetes.io/sgx=true
intel.feature.node.kubernetes.io/dsa=true
```

### Labels Table

| Label | Intel Hardware Feature |
|---|---|
| `intel.feature.node.kubernetes.io/gpu=true` | Intel® Data Center GPU Flex Series or Intel® Data Center GPU Max Series |
| `intel.feature.node.kubernetes.io/sgx=true` | Intel® SGX |
| `intel.feature.node.kubernetes.io/qat=true` | Intel® QAT |
| `intel.feature.node.kubernetes.io/dsa=true` | Intel® DSA |

## Intel Device Plugins Operator

The [Intel Device Plugins Operator](https://catalog.redhat.com/software/container-stacks/detail/61e9f2d7b9cdd99018fc5736)
manages the deployment and lifecycle of device plugins that advertise Intel
hardware resources to the RHOCP cluster. The operator and device plugin
container images are certified and published on the
[Red Hat Ecosystem Catalog](https://catalog.redhat.com/software/container-stacks/detail/61e9f2d7b9cdd99018fc5736).

### Installation via Web Console

1. In the OpenShift web console, navigate to **Operator** → **OperatorHub**.
2. Search for **Intel Device Plugins Operator**.
3. Click **Install**.

Verify by navigating to **Operator** → **Installed Operators** and confirming
the status is **Succeeded**.

### Installation via CLI

Apply the following to create an operator subscription:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  labels:
    operators.coreos.com/intel-device-plugins-operator.openshift-operators: ""
  name: intel-device-plugins-operator
  namespace: openshift-operators
spec:
  channel: stable
  installPlanApproval: Automatic
  name: intel-device-plugins-operator
  source: certified-operators
  sourceNamespace: openshift-marketplace
```

Save the above to a file (e.g. `install-operator.yaml`) and apply:

```bash
oc apply -f install-operator.yaml
```

### Verify Operator Installation

```bash
oc get pod -n openshift-operators | grep inteldeviceplugins-controller-manager
```

Expected output (example):

```
inteldeviceplugins-controller-manager-6b8c76c867-hftqm   2/2     Running   0   17m
```

## Creating Device Plugin Custom Resources

After the operator is installed, create Custom Resources (CRs) for each device
plugin you need. Each CR can be created either via the OpenShift web console or
via `oc` CLI.

> **Note:** The container image references shown below are examples. Always
> check the [Red Hat Ecosystem Catalog](https://catalog.redhat.com/software/container-stacks/detail/61e9f2d7b9cdd99018fc5736)
> for the latest certified image digests.

### Intel QAT Device Plugin

#### Prerequisites

- [Machine Configuration](#machine-configuration) applied (IOMMU and VFIO enabled).
- NFD labels indicating QAT presence on worker nodes.

#### Create via Web Console

1. Go to **Operator** → **Installed Operators**.
2. Open **Intel Device Plugins Operator**.
3. Navigate to the **Intel QuickAssist Technology Device Plugin** tab.
4. Click **Create QatDevicePlugin** → set parameters → click **Create**.

#### Create via CLI

```yaml
apiVersion: deviceplugin.intel.com/v1
kind: QatDevicePlugin
metadata:
  name: qatdeviceplugin-sample
spec:
  image: registry.connect.redhat.com/intel/intel-qat-plugin@sha256:ccb19d26f6afdb79cf0b2b70dab73b45f2bb9d5d3d94482486bec4beab6bfba3  # check Red Hat Ecosystem Catalog for latest digest
  initImage: registry.connect.redhat.com/intel/intel-qat-initcontainer@sha256:33c39996800676660f72952c994775c05de63e102f4b67afdd6a0e3f5a1bab5f  # check Red Hat Ecosystem Catalog for latest digest
  dpdkDriver: vfio-pci
  kernelVfDrivers:
    - 4xxxvf
    - 420xxvf
  maxNumDevices: 128
  logLevel: 4
  nodeSelector:
    intel.feature.node.kubernetes.io/qat: "true"
```

Save and apply:

```bash
oc apply -f qat-device-plugin.yaml
```

#### Verify

```bash
oc get QatDevicePlugin
```

Example output:

```
NAME                     DESIRED   READY   NODE SELECTOR                                       AGE
qatdeviceplugin-sample   1         1       {"intel.feature.node.kubernetes.io/qat":"true"}      3h
```

Verify QAT resources on a node:

```bash
oc describe node <node-name> | grep qat.intel.com
```

Example output:

```
qat.intel.com/cy: 32
qat.intel.com/dc: 32
```

> **Note:** By default the device plugin registers half the resources for
> `qat.intel.com/cy` (crypto) and half for `qat.intel.com/dc` (compression).

#### QAT Resource Configuration

The QAT device plugin supports the following configuration flags:

| Flag | Value | Description |
|---|---|---|
| `-dpdk-driver` | `vfio-pci` | VFIO driver for QAT VFIO devices |
| `-kernel-vf-drivers` | `4xxxvf` | Support for 4xxx QAT devices |
| `-max-num-devices` | `128` | Maximum VF devices to enable |
| `-provisioning-config` | ConfigMap name | Custom resource configuration |

To customize QAT resource services, create a ConfigMap:

```bash
oc create configmap --namespace=openshift-operators \
  --from-literal "qat.conf=ServicesEnabled=<option>" <configmap-name>
```

Where `<option>` is one of:
- `dc` — compression services (`qat.intel.com/dc`)
- `sym;asym` — crypto services (`qat.intel.com/cy`)
- `sym` — symmetric crypto (`qat.intel.com/sym`)
- `asym` — asymmetric crypto (`qat.intel.com/asym`)
- `sym;dc` — symmetric crypto and compression (`qat.intel.com/sym-dc`)
- `asym;dc` — asymmetric crypto and compression (`qat.intel.com/asym-dc`)

Then reference the ConfigMap name in the `provisioningConfig` field of the
`QatDevicePlugin` CR.

For more details about QAT device plugin configuration, see
[Modes and Configuration Options](cmd/qat_plugin/README.md#modes-and-configuration-options).

### Intel SGX Device Plugin

#### Prerequisites

- NFD labels indicating SGX presence on worker nodes.
- SGX enabled in BIOS (see [BIOS Configuration](#bios-configuration)).

#### Create via Web Console

1. Go to **Operator** → **Installed Operators**.
2. Open **Intel Device Plugins Operator**.
3. Navigate to the **Intel Software Guard Extensions Device Plugin** tab.
4. Click **Create SgxDevicePlugin** → set parameters → click **Create**.

#### Create via CLI

```yaml
apiVersion: deviceplugin.intel.com/v1
kind: SgxDevicePlugin
metadata:
  name: sgxdeviceplugin-sample
spec:
  image: registry.connect.redhat.com/intel/intel-sgx-plugin@sha256:4ac8769c4f0a82b3ea04cf1532f15e9935c71fe390ff5a9dc3ee57f970a65f0b  # check Red Hat Ecosystem Catalog for latest digest
  enclaveLimit: 110
  provisionLimit: 110
  logLevel: 4
  nodeSelector:
    intel.feature.node.kubernetes.io/sgx: "true"
```

Save and apply:

```bash
oc apply -f sgx-device-plugin.yaml
```

#### Verify

```bash
oc get SgxDevicePlugin
```

Example output:

```
NAME                     DESIRED   READY   NODE SELECTOR                                       AGE
sgxdeviceplugin-sample   1         1       {"intel.feature.node.kubernetes.io/sgx":"true"}      2m
```

### Intel DSA Device Plugin

#### Prerequisites

- [Machine Configuration](#machine-configuration) applied (IOMMU and VFIO enabled).
- NFD labels indicating DSA presence on worker nodes.

#### Create via Web Console

1. Go to **Operator** → **Installed Operators**.
2. Open **Intel Device Plugins Operator**.
3. Navigate to the **Intel DSA Device Plugin** tab.
4. Click **Create DSADevicePlugin** → set parameters → click **Create**.

#### Create via CLI

```yaml
apiVersion: deviceplugin.intel.com/v1
kind: DsaDevicePlugin
metadata:
  name: dsadeviceplugin-sample
spec:
  image: registry.connect.redhat.com/intel/intel-dsa-plugin@sha256:64ec224b9382f711cf834722a85497965bb20e4fbf54e619ac296b46be6e1964  # check Red Hat Ecosystem Catalog for latest digest
  initImage: registry.connect.redhat.com/intel/intel-idxd-config-initcontainer@sha256:c573ff46096f78d025d736bb3eedb131e9fc3aa2271d2dd6096a4c4911ee8a1f  # check Red Hat Ecosystem Catalog for latest digest
  logLevel: 4
  nodeSelector:
    intel.feature.node.kubernetes.io/dsa: "true"
  sharedDevNum: 10
```

Save and apply:

```bash
oc apply -f dsa-device-plugin.yaml
```

#### Verify

```bash
oc get DsaDevicePlugin
```

Example output:

```
NAME                     DESIRED   READY   NODE SELECTOR                                       AGE
dsadeviceplugin-sample   3         3       {"intel.feature.node.kubernetes.io/dsa":"true"}      98m
```

Verify DSA resources on a node:

```bash
oc describe node <node-name> | grep dsa.intel.com
```

Example output:

```
dsa.intel.com/wq-user-shared:     160
dsa.intel.com/wq-user-dedicated:  0
```

#### DSA Resource Configuration

The DSA plugin uses a
[default configuration](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/dsa.conf)
that creates dedicated work queues for each DSA device. To customize the
configuration:

1. Create a ConfigMap with your DSA configuration:

   ```bash
   oc create configmap --namespace=openshift-operators \
     intel-dsa-config --from-file=dsa.conf
   ```

2. Reference the ConfigMap name in the `provisioningConfig` field of the
   `DsaDevicePlugin` CR.

### Intel GPU Device Plugin

> **Note:** Although the Intel GPU device plugin is part of the supported device plugins set, the i915 out-of-tree KMD driver is no longer provided and the new Xe KMD driver is not yet available. It is not possible to utilize the GPU plugin in RHCOP without a functioning GPU KMD.

#### Prerequisites

- NFD labels indicating GPU presence on worker nodes.

#### Create via Web Console

1. Go to **Operator** → **Installed Operators**.
2. Open **Intel Device Plugins Operator**.
3. Navigate to the **Intel GPU Device Plugin** tab.
4. Click **Create GpuDevicePlugin** → set parameters → click **Create**.

#### Create via CLI

```yaml
apiVersion: deviceplugin.intel.com/v1
kind: GpuDevicePlugin
metadata:
  name: gpudeviceplugin-sample
spec:
  image: registry.connect.redhat.com/intel/intel-gpu-plugin@sha256:51f5db99b7ec78407cb8f00e20d6f455e62edbb3a4abe3352dfaa6c037ba598c  # check Red Hat Ecosystem Catalog for latest digest
  preferredAllocationPolicy: none
  sharedDevNum: 1
  logLevel: 4
  nodeSelector:
    intel.feature.node.kubernetes.io/gpu: "true"
```

Save and apply:

```bash
oc apply -f gpu-device-plugin.yaml
```

#### Verify

```bash
oc get GpuDevicePlugin
```

Example output:

```
NAME                     DESIRED   READY   NODE SELECTOR                                       AGE
gpudeviceplugin-sample   1         1       {"intel.feature.node.kubernetes.io/gpu":"true"}      3m
```

#### Using GPU Resources

When claiming `i915` resources in your workload, set both limits and requests:

```yaml
spec:
  containers:
    - name: gpu-workload
      resources:
        limits:
          gpu.intel.com/i915: 1
        requests:
          gpu.intel.com/i915: 1
```

## Resources Provided by Intel Device Plugins

The table below summarizes the Kubernetes resources exposed by each device
plugin. Workloads request these resources in their pod specs.

| Feature | Resource | Description |
|---|---|---|
| Intel® SGX | `sgx.intel.com/epc` | SGX Enclave Page Cache memory |
| Intel® Data Center GPU | `gpu.intel.com/i915` | Intel GPU device |
| Intel® QAT | `qat.intel.com/cy` | QAT VFIO VF for cryptography |
| Intel® QAT | `qat.intel.com/dc` | QAT VFIO VF for compression |
| Intel® DSA | `dsa.intel.com/wq-user-shared` | DSA shared work queue |
| Intel® DSA | `dsa.intel.com/wq-user-dedicated` | DSA dedicated work queue |
