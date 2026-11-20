# Labels from NFD rules

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

