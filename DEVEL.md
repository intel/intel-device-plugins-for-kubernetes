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

Logging
-------

The framework uses [`klog`](https://github.com/kubernetes/klog) as its logging
framework. It is encouraged for plugins to also use `klog` to maintain uniformity
in the logs and command line options.

The framework initialises `klog`, so further calls to `klog.InitFlags()` by
plugins should not be necessary. This does add a number of log configuration
options to your plugin, which can be viewed with the `-h` command line option of your
plugin.

The framework tries to adhere to the Kubernetes
[Logging Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md). The advise is to use the `V()` levels for `Info()` calls, as calling `Info()`
with no set level will make configuration and filtering of logging via the command
line more difficult.

The default is to not log `Info()` calls. This can be changed using the plugin command
line `-v` parameter. The additional annotations prepended to log lines by 'klog' can be disabled
with the `-skip_headers` option.

Error Conventions
-----------------

The framework has a convention for producing and logging errors. Ideally plugins will also adhere
to the convention.

Errors generated within the framework and plugins are instantiated with the `New()` and
`Errorf()` functions of the [errors package](https://golang.org/pkg/errors/):

```golang
    return errors.New("error message")
```

Errors generated from outside the plugins and framework are augmented with their stack dump with code such as

```golang
    return errors.WithStack(err)
```

or

```golang
    return errors.Wrap(err, "some additional error message")
```

These errors are then logged using a default struct value format like:

```golang
    klog.Errorf("Example of an internal error death: %+v", err)
```

at the line where it's certain that the error cannot be passed out farther nor handled gracefully.
Otherwise, they can be logged as simple values:

```golang
    klog.Warningf("Example of a warning due to an external error: %v", err)
```
