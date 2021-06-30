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
|`gpu.intel.com/cards`| string | list of card names separated by '`.`'. The names match host `card*`-folders under `/sys/class/drm/`.

## Capability labels (optional)

Capability labels are created from information found inside debugfs, and therefore
unfortunately require running the NFD worker as root. Due to coming from debugfs,
which is not guaranteed to be stable, these are not guaranteed to be stable either.
If you don't need these, simply do not run NFD worker as root, that is also more secure.
Depending on your kernel driver, running the NFD hook as root may introduce following labels:

name | type | description|
-----|------|------|
|`gpu.intel.com/platform_gen`| string | GPU platform generation name, typically a number.
|`gpu.intel.com/platform_<PLATFORM_NAME>_.count`| number | GPU count for the named platform.
|`gpu.intel.com/platform_<PLATFORM_NAME>_.tiles`| number | GPU tile count in the GPUs of the named platform.
|`gpu.intel.com/platform_<PLATFORM_NAME>_.present`| string | "true" for indicating the presense of the GPU platform.

For the above to work as intended, installed GPUs must be identical in their capabilities.