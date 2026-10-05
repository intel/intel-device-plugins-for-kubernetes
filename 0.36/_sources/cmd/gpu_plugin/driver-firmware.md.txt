# Driver and firmware for Intel GPUs

Access to a GPU device requires firmware, kernel and user-space
drivers supporting it.  Firmware and kernel driver need to be on the
host, user-space drivers in the GPU workload containers.

Intel GPU devices supported by the current kernel can be listed with:
```
$ grep i915 /sys/class/drm/card?/device/uevent
/sys/class/drm/card0/device/uevent:DRIVER=i915
/sys/class/drm/card1/device/uevent:DRIVER=i915
```

## Drivers for discrete GPUs

> **Note**: Kernel (on host) and user-space drivers (in containers)
> should be installed from the same repository as there are some
> differences between DKMS and upstream GPU driver uAPI.

##### Kernel driver

###### Intel DKMS packages

`i915` GPU driver DKMS[^dkms] package is recommended for Intel
discrete GPUs, until their support in upstream is complete.  DKMS
package(s) can be installed from Intel package repositories for a
subset of older kernel versions used in enterprise / LTS
distributions:
https://dgpu-docs.intel.com/installation-guides/index.html

[^dkms]: [intel-gpu-i915-backports](https://github.com/intel-gpu/intel-gpu-i915-backports).

###### Upstream kernel

Support for first Intel discrete GPUs was added to upstream Linux kernel in v6.2,
and expanded in later versions. For now, upstream kernel is still missing support
for few of the features available in DKMS kernels, listed here:
https://dgpu-docs.intel.com/driver/kernel-driver-types.html

##### GPU Version

PCI IDs for the Intel GPUs on given host can be listed with:
```
$ lspci | grep -e VGA -e Display | grep Intel
88:00.0 Display controller: Intel Corporation Device 56c1 (rev 05)
8d:00.0 Display controller: Intel Corporation Device 56c1 (rev 05)
```

(`lspci` lists GPUs with display support as "VGA compatible controller",
and server GPUs without display support, as "Display controller".)

A mapping between GPU PCI IDs and their Intel brand names is available here:
https://dgpu-docs.intel.com/devices/hardware-table.html

###### GPU Firmware

If your kernel build does not find the correct firmware version for
a given GPU from the host (see `dmesg | grep i915` output), latest
firmware versions are available in upstream:
https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/tree/i915

##### User-space drivers

Until new enough user-space drivers (supporting also discrete GPUs)
are available directly from distribution package repositories, they
can be installed to containers from Intel package repositories. See:
https://dgpu-docs.intel.com/installation-guides/index.html

Example container is listed in [Testing and demos](#testing-and-demos).

Validation status against *upstream* kernel is listed in the user-space drivers release notes:
* Media driver: https://github.com/intel/media-driver/releases
* Compute driver: https://github.com/intel/compute-runtime/releases

#### Drivers for older (integrated) GPUs

For the older (integrated) GPUs, new enough firmware and kernel driver
are typically included already with the host OS, and new enough
user-space drivers (for the GPU containers) are in the host OS
repositories.
