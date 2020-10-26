# Intel GPU NFD hook

This is the Node Feature Discovery binary hook implementation for the Intel
GPUs. The intel-gpu-initcontainer which is built among other images can be
placed as part of the gpu-plugin deployment, so that it copies this hook to the
host system only in those hosts, in which also gpu-plugin is deployed.

When NFD worker runs this hook, it will add a number of labels to the nodes,
which can be used for example to deploy services to nodes with specific GPU
types. Selected numeric labels can be turned into kubernetes extended resources
by the NFD, allowing for finer grained resource management for GPU-using PODs.

In the NFD deployment, the hook requires /host-sys -folder to have the host /sys
-folder content mounted, and /host-dev to have the host /dev -folder content
mounted. Write access is not necessary.

GPU memory amount is read from sysfs gt/gt* files and turned into a label.
There are two supported environment variables named GPU_MEMORY_OVERRIDE and
GPU_MEMORY_RESERVED. Both are supposed to hold numeric values. For systems with
older kernel drivers or GPUs which do not support reading the GPU memory
amount, the GPU_MEMORY_OVERRIDE environment variable value is turned into a GPU
memory amount label instead of a read value. GPU_MEMORY_RESERVED value will be
scoped out from the GPU memory amount found from sysfs.
