# Using Intel GPU plugin with KubeVirt

Intel GPU can be used to aid the use of GPUs in KubeVirt Pods. GPU plugin may assist KubeVirt use in two ways: automatically binding the GPU device to `vfio-pci` driver, and by registering vfio resource that can be used by KubeVirt.

## Intel GPU initcontainer

KubeVirt requires the devices to be bound to the `vfio-pci` driver so that they can be passed through to a KubeVirt VM Pod. The initcontainer runs before the plugin and unbinds the GPU devices from xe or i915 driver and rebinds them to vfio-pci.

The rebind is one-way, so it doesn't support moving the devices back from vfio-pci to xe or i915.

To use the initcontainer in the Operator GPU CR, set the initImage variable:
```
spec:
  initImage: intel/intel-gpu-initcontainer:x.y.z
```

## Intel GPU plugin

The GPU plugin has to run in a special `vfio` mode to support KubeVirt use case. This can be achieved by deploying the [kubevirt overlay](../../deployments/gpu_plugin/overlays/kubevirt/) or by requesting VFIOMode=true with the Device Plugin Operator.

Plugin has to run with `-run-mode=vfio` arguments and have `/sys/bus/pci` mounted to the container.

> **Note**: When plugin is in `vfio` mode, it doesn't register the normal `i915` or `xe` resources. Plugin also assumes the GPU devices are bound to the vfio-pci driver.

## KubeVirt configuration

To support Intel GPUs, the following changes are required in the KubeVirt CR:

```
spec:
  configuration:
    permittedHostDevices:
      pciHostDevices:
      - externalResourceProvider: true
        pciVendorSelector: 8086:*
        resourceName: gpu.intel.com/vfio
```

In the VM CR, one has to define this to pass-through one GPU:

```
spec:
  template:
     spec:
       domain:
         devices:
           gpus:
             - name: gpu0
               deviceName: gpu.intel.com/vfio
```

## Limiting access to hardware

If a node has multiple different GPUs, e.g. an integrated GPU, it may be good to ignore that device from being used in KubeVirt Pods. GPU plugin can be configured to only allow certain devices to be registered as resources. Both the init-container and the plugin support "-allow-ids" and "-deny-ids" arguments that can limit which GPUs the plugin will use.

For example, if only a B570 is intended to be used in KubeVirt, then one can use `-allow-ids=0xe20c` argument in the plugin, or `AllowIDs` in the plugin CR with the Device Plugin Operator.
