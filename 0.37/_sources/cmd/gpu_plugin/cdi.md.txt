# CDI specifications with Intel GPU devices

Container Device Interface (CDI) enables defining specifications that allow greater control on how devices are exposed to containers. The most common way to expose GPU devices to containers is to use privileged mode: `docker run --privileged`. The problem with that is it exposes every device in the host as well as gives unlimited access to the host. It also lacks some details from the GPU device files; for example, the `by-path` symlinks under `/dev/dri/by-path` are not included in the container. A slightly better way to expose GPU devices is to expose the device files one by one: `docker run --device /dev/dri/card0 --device /dev/dri/renderD128`. That has better control on which devices are exposed, but it creates long and error-prone command lines.

## CDI specification

An improved way to access Intel GPUs is to use CDI and its device specifications to expose Intel GPUs to containers. CDI uses predefined YAML or JSON specification files to describe which device files, mounts and environment variables should be exposed to the container. An example is shown below:

```
---
cdiVersion: 0.5.0
kind: intel.com/gpu
devices:
    - name: gpu0
      containerEdits:
        deviceNodes:
            - path: /dev/dri/card0
              hostPath: /dev/dri/card0
              type: c
            - path: /dev/dri/renderD128
              hostPath: /dev/dri/renderD128
              type: c
        mounts:
            - hostPath: /dev/dri/by-path
              containerPath: /dev/dri/by-path
              options:
                - rw
                - rbind
              type: bind
```

Docker's `--device` argument can use the CDI device name:
```
docker run --rm -it --device intel.com/gpu=gpu0 ubuntu:24.04
```

## Generating CDI specs automatically

Intel GPU DRA driver uses CDI specs extensively as DRA's functionality is based on it. It has [a tool](https://github.com/intel/intel-resource-drivers-for-kubernetes/tree/main/cmd/cdi-specs-generator) that can generate CDI specs for the devices it detects in the host.

Build it:
```
go install github.com/intel/intel-resource-drivers-for-kubernetes/cmd/cdi-specs-generator@gpu-v0.8.0
```
> NOTE: Using newer tags is possible, but there are extra dependencies needed for the build to succeed.

And run it:
```
sudo cdi-specs-generator --naming classic --cdi-dir /etc/cdi gpu
```

The tool creates a `intel.com-gpu.yaml` file under `/etc/cdi` and uses `cardX` names for the CDI devices. To use them, the docker command is as follows:
```
docker run --rm -it --device intel.com/gpu=card1 ubuntu:24.04
```
