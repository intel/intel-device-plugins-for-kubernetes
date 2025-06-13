# Labels

GPU labels originate from two main sources: NFD rules and GPU plugin (& NFD hook).

## NFD rules

NFD rule is a method to instruct NFD to add certain label(s) to node based on the devices detected on it. There is a generic rule to identify all Intel GPUs. It will add labels for each PCI device type. For example, a Tigerlake iGPU (PCI Id 0x9a49) will show up as:

```
gpu.intel.com/device-id.0300-9a49.count=1
gpu.intel.com/device-id.0300-9a49.present=true
```

For data center GPUs, there are more specific rules which will create additional labels for GPU family, product and device count. For example, Flex 170:
```
gpu.intel.com/device.count=1
gpu.intel.com/family=Flex_Series
gpu.intel.com/product=Flex_170
```

For MAX 1550:
```
gpu.intel.com/device.count=2
gpu.intel.com/family=Max_Series
gpu.intel.com/product=Max_1550
```

Current covered platforms/devices are: Flex 140, Flex 170, Max 1100 and Max 1550.

To identify other GPUs, see the graphics processor table [here](https://dgpu-docs.intel.com/devices/hardware-table.html#graphics-processor-table).

## GPU Plugin and NFD hook

In GPU plugin, these labels are only applied when [Resource Management](README.md#fractional-resources-details) is enabled. With the NFD hook, labels are created regardless of how GPU plugin is configured.

Numeric labels are converted into extended resources for the node (with NFD) and other labels are used directly by [GPU Aware Scheduling (GAS)](https://github.com/intel/platform-aware-scheduling/tree/master/gpu-aware-scheduling). Extended resources should only be used with GAS as Kubernetes scheduler doesn't properly handle resource allocations with multiple GPUs.

### Default labels

Following labels are created by default.

name | type | description|
-----|------|------|
|`gpu.intel.com/millicores`| number | node GPU count * 1000.
|`gpu.intel.com/memory.max`| number | sum of detected [GPU memory amounts](#gpu-memory) in bytes OR environment variable value * GPU count
|`gpu.intel.com/cards`| string | list of card names separated by '`.`'. The names match host `card*`-folders under `/sys/class/drm/`. Deprecated, use `gpu-numbers`.
|`gpu.intel.com/gpu-numbers`| string | list of numbers separated by '`.`'. The numbers correspond to device file numbers for the primary nodes of given GPUs in kernel DRI subsystem, listed as `/dev/dri/card<num>` in devfs, and `/sys/class/drm/card<num>` in sysfs.
|`gpu.intel.com/tiles`| number | sum of all detected GPU tiles in the system.
|`gpu.intel.com/numa-gpu-map`| string | list of numa node to gpu mappings.

If the value of the `gpu-numbers` label would not fit into the 63 character length limit, you will also get labels `gpu-numbers2`,
`gpu-numbers3`... until all the gpu numbers have been labeled.

The tile count `gpu.intel.com/tiles` describes the total amount of tiles on the system. System is expected to be homogeneous, and thus the number of tiles per GPU can be calculated by dividing the tile count with GPU count.

The `numa-gpu-map` label is a list of numa to gpu mapping items separated by `_`. Each list item has a numa node id combined with a list of gpu indices. e.g. 0-1.2.3 would mean: numa node 0 has gpus 1, 2 and 3. More complex example would be: 0-0.1_1-3.4 where numa node 0 would have gpus 0 and 1, and numa node 1 would have gpus 3 and 4. As with `gpu-numbers`, this label will be extended to multiple labels if the length of the value exceeds the max label length.

### PCI-groups (optional)

GPUs which share the same PCI paths under `/sys/devices/pci*` can be grouped into a label. GPU nums are separated by '`.`' and
groups are separated by '`_`'. The label is created only if environment variable named `GPU_PCI_GROUPING_LEVEL` has a value greater
than zero. GPUs are considered to belong to the same group, if as many identical folder names are found for the GPUs, as is the value
of the environment variable. Counting starts from the folder name which starts with `pci`.

For example, the SG1 card has 4 GPUs, which end up sharing pci-folder names under `/sys/devices`. With a `GPU_PCI_GROUPING_LEVEL`
of 3, a node with two such SG1 cards could produce a `pci-groups` label with a value of `0.1.2.3_4.5.6.7`.

name | type | description|
-----|------|------|
|`gpu.intel.com/pci-groups`| string | list of pci-groups separated by '`_`'. GPU numbers in the groups are separated by '`.`'. The numbers correspond to device file numbers for the primary nodes of given GPUs in kernel DRI subsystem, listed as `/dev/dri/card<num>` in devfs, and `/sys/class/drm/card<num>` in sysfs.

If the value of the `pci-groups` label would not fit into the 63 character length limit, you will also get labels `pci-groups2`,
`pci-groups3`... until all the PCI groups have been labeled.

### Limitations

For the above to work as intended, GPUs on the same node must be identical in their capabilities.

### GPU memory

GPU memory amount is read from sysfs `gt/gt*` files and turned into a label.
There are two supported environment variables named `GPU_MEMORY_OVERRIDE` and
`GPU_MEMORY_RESERVED`. Both are supposed to hold numeric byte amounts. For systems with
older kernel drivers or GPUs which do not support reading the GPU memory
amount, the `GPU_MEMORY_OVERRIDE` environment variable value is turned into a GPU
memory amount label instead of a read value. `GPU_MEMORY_RESERVED` value will be
scoped out from the GPU memory amount found from sysfs.
