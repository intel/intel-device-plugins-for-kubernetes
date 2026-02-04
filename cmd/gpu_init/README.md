# Intel GPU Initcontainer

**Note:** Intel GPU Initcontainer has shifted from an NFD hook to a VFIO provisioning container. The previous NFD hook initcontainer has not been actively used since 0.28.0.

The initcontainer performs a single task: it unbinds Intel GPU devices from their GPU driver (`i915` or `xe`) and binds them to the `vfio-pci` driver so they can be used by VMs. This enables the GPU plugin’s KubeVirt use case without manual device binding.

The initcontainer only supports one-way binding. To rebind a GPU back to its GPU driver, you must do it manually. Example for device ID `e20b` with BDF `0000:04:00.0`:

```
DEVICEID="e20b"
BDF="0000:04:00.0"

echo -n "8086 $DEVICEID" | sudo tee /sys/bus/pci/drivers/vfio-pci/remove_id
echo -n $BDF | sudo tee /sys/bus/pci/drivers/vfio-pci/unbind
echo -n $BDF | sudo tee /sys/bus/pci/drivers/xe/bind
```
