# Intel GPU NFD hook

This is the [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
binary hook implementation for the Intel GPUs. The intel-gpu-initcontainer which
is built among other images can be placed as part of the gpu-plugin deployment,
so that it copies this hook to the host system only in those hosts, in which also
gpu-plugin is deployed.

When NFD worker runs this hook, it will add a number of labels to the nodes,
which can be used for example to deploy services to nodes with specific GPU
types. Selected numeric labels can be turned into kubernetes extended resources
by the NFD, allowing for finer grained resource management for GPU-using PODs.

In the NFD deployment, the hook requires `/host-sys` -folder to have the host `/sys`-folder content mounted. Write access is not necessary.

## GPU memory

GPU memory amount is read from sysfs gt/gt* files and turned into a label.
There are two supported environment variables named `GPU_MEMORY_OVERRIDE` and
`GPU_MEMORY_RESERVED`. Both are supposed to hold numeric byte amounts. For systems with
older kernel drivers or GPUs which do not support reading the GPU memory
amount, the `GPU_MEMORY_OVERRIDE` environment variable value is turned into a GPU
memory amount label instead of a read value. `GPU_MEMORY_RESERVED` value will be
scoped out from the GPU memory amount found from sysfs.

## Default labels

Following labels are created by default. You may turn numeric labels into extended resources with NFD.

name | type | description|
-----|------|------|
|`gpu.intel.com/millicores`| number | node GPU count * 1000. Can be used as a finer grained shared execution fraction.
|`gpu.intel.com/memory.max`| number | sum of detected [GPU memory amounts](#GPU-memory) in bytes OR environment variable value * GPU count
|`gpu.intel.com/cards`| string | list of card names separated by '`.`'. The names match host `card*`-folders under `/sys/class/drm/`. Deprecated, use `gpu-numbers`.
|`gpu.intel.com/gpu-numbers`| string | list of numbers separated by '`.`'. The numbers correspond to device file numbers for the primary nodes of given GPUs in kernel DRI subsystem, listed as `/dev/dri/card<num>` in devfs, and `/sys/class/drm/card<num>` in sysfs.
|`gpu.intel.com/tiles`| number | sum of all detected GPU tiles in the system.

If the value of the `gpu-numbers` label would not fit into the 63 character length limit, you will also get labels `gpu-numbers2`,
`gpu-numbers3`... until all the gpu numbers have been labeled.

The tile count `gpu.intel.com/tiles` describes the total amount of tiles on the system. System is expected to be homogeneous, and thus the number of tiles per GPU can be calculated by dividing the tile count with GPU count.

## PCI-groups (optional)

GPUs which share the same pci paths under `/sys/devices/pci*` can be grouped into a label. GPU nums are separated by '`.`' and
groups are separated by '`_`'. The label is created only if environment variable named `GPU_PCI_GROUPING_LEVEL` has a value greater
than zero. GPUs are considered to belong to the same group, if as many identical folder names are found for the GPUs, as is the value
of the environment variable. Counting starts from the folder name which starts with `pci`.

For example, the SG1 card has 4 GPUs, which end up sharing pci-folder names under `/sys/devices`. With a `GPU_PCI_GROUPING_LEVEL`
of 3, a node with two such SG1 cards could produce a `pci-groups` label with a value of `0.1.2.3_4.5.6.7`.

name | type | description|
-----|------|------|
|`gpu.intel.com/pci-groups`| string | list of pci-groups separated by '`_`'. GPU numbers in the groups are separated by '`.`'. The numbers correspond to device file numbers for the primary nodes of given GPUs in kernel DRI subsystem, listed as `/dev/dri/card<num>` in devfs, and `/sys/class/drm/card<num>` in sysfs.

If the value of the `pci-groups` label would not fit into the 63 character length limit, you will also get labels `pci-groups2`,
`pci-groups3`... until all the pci groups have been labeled.

## Capability labels (optional)

Capability labels are created from information found inside debugfs, and therefore
unfortunately require running the NFD worker as root. Due to coming from debugfs,
which is not guaranteed to be stable, these are not guaranteed to be stable either.
If you don't need these, simply do not run NFD worker as root, that is also more secure.
Depending on your kernel driver, running the NFD hook as root may introduce following labels:

name | type | description|
-----|------|------|
|`gpu.intel.com/platform_gen`| string | GPU platform generation name, typically an integer. Deprecated.
|`gpu.intel.com/media_version`| string | GPU platform Media pipeline generation name, typically a number.
|`gpu.intel.com/graphics_version`| string | GPU platform graphics/compute pipeline generation name, typically a number.
|`gpu.intel.com/platform_<PLATFORM_NAME>.count`| number | GPU count for the named platform.
|`gpu.intel.com/platform_<PLATFORM_NAME>.tiles`| number | GPU tile count in the GPUs of the named platform.
|`gpu.intel.com/platform_<PLATFORM_NAME>.present`| string | "true" for indicating the presense of the GPU platform.

## Limitations

For the above to work as intended, GPUs on the same node must be identical in their capabilities.
