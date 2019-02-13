How to develop simple device plugins
====================================

To create a simple device plugin without the hassle of developing your own gRPC
server, you can use a package included in this repository called
`github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin`.

All you have to do is instantiate a `deviceplugin.Manager` and call
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
   The manager will instantiate multiple gRPC servers for every registered "color".
2. `plugin` which is a reference to an object implementing one mandatory
   interface `deviceplugin.Scanner`.

`deviceplugin.Scanner` defines one method `Scan()` which is called only once
for every device plugin by `deviceplugin.Manager` in a goroutine and operates
in an infinite loop. A `Scan()` implementation scans the host for devices and
sends all found devices to a `deviceplugin.Notifier` instance. The
`deviceplugin.Notifier` is implemented and provided by the `deviceplugin`
package itself. The found devices are organized in an instance of
`deviceplugin.DeviceTree` object. The object is filled in with its 
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

Optionally, your device plugin may also implement the 
`deviceplugin.PostAllocator` interface. If implemented, its method 
`PostAllocate()` modifies `pluginapi.AllocateResponse` responses just
before they are sent to `kubelet`. To see an example, refer to the FPGA
plugin which implements this interface to annotate its responses.

