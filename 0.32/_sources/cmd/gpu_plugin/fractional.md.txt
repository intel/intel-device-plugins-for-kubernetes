# GPU plugin with GPU Aware Scheduling

This is an experimental feature.

Installing the GPU plugin with [GPU Aware Scheduling](https://github.com/intel/platform-aware-scheduling/tree/master/gpu-aware-scheduling) (GAS) enables containers to request partial (fractional) GPU resources. For example, a Pod's container can request GPU's millicores or memory and use only a fraction of the GPU. The remaining resources could be leveraged by another container.

> *NOTE*: For this use case to work properly, all GPUs in a given node should provide equal amount of resources
i.e. heterogenous GPU nodes are not supported.

> *NOTE*:  Resource values are used only for scheduling workloads to nodes, not for limiting their GPU usage on the nodes. Container requesting 50% of the GPU's resources is not restricted by the kernel driver or firmware from using more than 50% of the resources. A container requesting 1% of the GPU could use 100% of it.

## Install GPU Aware Scheduling

GAS' installation is described in its [README](https://github.com/intel/platform-aware-scheduling/tree/master/gpu-aware-scheduling#usage-with-nfd-and-the-gpu-plugin).

## Install GPU plugin with fractional resources

### With yaml deployments

The GPU Plugin DaemonSet needs additional RBAC-permissions and access to the kubelet podresources
gRPC service to function. All the required changes are gathered in the `fractional_resources`
overlay. Install GPU plugin by running:

```bash
# Start NFD - if your cluster doesn't have NFD installed yet
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>'

# Create NodeFeatureRules for detecting GPUs on nodes
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>'

# Create GPU plugin daemonset
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin/overlays/fractional_resources?ref=<RELEASE_VERSION>'
```

> **NOTE:** The yaml deployment above does not support deployment to non-default namespace. The ClusterRoleBinding object has a hardcoded namespace and does not respect the target namespace. If you would like to deploy to a custom namespace, you will need to either modify the [yaml file](../../deployments/gpu_plugin/overlays/fractional_resources/gpu-manager-rolebinding.yaml) or deploy using the Operator.

### With Device Plugin Operator

Install the Device Plugin Operator according to the [install](../operator/README.md#installation) instructions. When applying the [GPU plugin Custom Resource](../../deployments/operator/samples/deviceplugin_v1_gpudeviceplugin.yaml) (CR), set `resourceManager` option to `true`. The Operator will install all the required RBAC objects and service accounts.

```
spec:
  resourceManager: true
```

## Details about fractional resources

Use of fractional GPU resources requires that the cluster has node extended resources with the name prefix `gpu.intel.com/`. Those are automatically created by GPU plugin with the help of the NFD. When fractional resources are enabled, the plugin lets GAS do card selection decisions based on resource availability and the amount of extended resources requested in the [pod spec](https://github.com/intel/platform-aware-scheduling/blob/master/gpu-aware-scheduling/docs/usage.md#pods).

GAS then annotates the pod objects with unique increasing numeric timestamps in the annotation `gas-ts` and container card selections in `gas-container-cards` annotation. The latter has container separator '`|`' and card separator '`,`'. Example for a pod with two containers and both containers getting two cards: `gas-container-cards:card0,card1|card2,card3`.

Enabling the fractional resource support in the plugin without running GAS in the cluster will only slow down GPU-deployments, so do not enable this feature unnecessarily.

## Tile level access and Level Zero workloads

Level Zero library supports targeting different tiles on a GPU. If the host is equipped with multi-tile GPU devices, and the container requests both `gpu.intel.com/i915` and `gpu.intel.com/tiles` resources, GPU plugin (with GAS) adds an [affinity mask](https://spec.oneapi.io/level-zero/latest/core/PROG.html#affinity-mask) to the container. By default the mask is in "FLAT" [device hierarchy](https://spec.oneapi.io/level-zero/latest/core/PROG.html#device-hierarchy) format. With the affinity mask, two Level Zero workloads can share a two tile GPU so that workloads use one tile each.

If a multi-tile workload is intended to work in "COMPOSITE" hierarchy mode, the container spec environment should include hierarchy mode variable (ZE_FLAT_DEVICE_HIERARCHY) with "COMPOSITE" value. GPU plugin will then adapt the affinity mask from the default "FLAT" to "COMPOSITE" format.

If the GPU is a single tile device, GPU plugin does not set the affinity mask. Only exposing GPU devices is enough in that case.

### Details about tile resources

GAS makes the GPU and tile selection based on the Pod's resource specification. The selection is passed to GPU plugin via the Pod's annotation.

Tiles targeted for containers are specified to Pod via `gas-container-tiles` annotation where the the annotation value describes a set of card and tile combinations. For example in a two container pod, the annotation could be `gas-container-tiles:card0:gt0+gt1|card1:gt1,card2:gt0`. Similarly to `gas-container-cards`, the container details are split via `|`. In the example above, the first container gets tiles 0 and 1 from card 0, and the second container gets tile 1 from card 1 and tile 0 from card 2.
