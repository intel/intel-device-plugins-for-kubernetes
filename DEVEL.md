How to develop simple device plugins
====================================

This repository provides the package `github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin`
which enables you to create a simple device plugin without the hassle of
developing your own gRPC server.

All you have to do is to instantiate a `deviceplugin.Manager` and call
its `Run()` method:

```go
func main() {
    ...

    manager := dpapi.NewManager(namespace, plugin)
    manager.Run()
}
```

The manager's constructor accepts two parameters:

1. `namespace` which is a string like "color.example.com". All your devices
   will be exposed under this name space, e.g. "color.example.com/yellow".
   Please note that one device plugin can register many such "colors".
   The manager will take care of instantiating multiple gRPC servers
   each for every "color".
2. `plugin` which is a reference to an object implementing one mandatory
   interface `deviceplugin.Scanner`.

`deviceplugin.Scanner` defines one method `Scan()` which is called only
once for every device plugin by `deviceplugin.Manager` in a goroutine and
operates in an infinite loop. An implementation of this method is supposed
to scan the host for devices and to send all found devices to a
`deviceplugin.Notifier` instance. The latter is implemented and provided by the
`deviceplugin` package itself. The found devices are organized in an instance
of `deviceplugin.DeviceTree` object. The object is filled in with its
`AddDevice()` method:

```go
func (dp *devicePlugin) Scan(notifier deviceplugin.Notifier) error {
    for {
        devTree := deviceplugin.NewDeviceTree()
        ...
        devTree.AddDevice("yellow", devID, deviceplugin.DeviceInfo{
            State: health,
            Nodes: []pluginapi.DeviceSpec{
                {
                    HostPath:      devPath,
                    ContainerPath: devPath,
                    Permissions:   "rw",
                },
            },
        })
        ...
        notifier.Notify(devTree)
    }
}
```

Optionally, your device plugin may also implement the `deviceplugin.PostAllocator`
interface. If implemented its method `PostAllocate()` modifies
`pluginapi.AllocateResponse` responses just before they are sent to `kubelet`
e.g. to augment the responses with annotations like in the FPGA plugin.
