# Development


## How to develop simple device plugins

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

### Logging

The framework uses [`klog`](https://github.com/kubernetes/klog) as its logging
framework. It is encouraged for plugins to also use `klog` to maintain uniformity
in the logs and command line options.

The framework initialises `klog`, so further calls to `klog.InitFlags()` by
plugins should not be necessary. This does add a number of log configuration
options to your plugin, which can be viewed with the `-h` command line option of your
plugin.

The framework tries to adhere to the Kubernetes
[Logging Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md).
The advise is to use the `V()` levels for `Info()` calls, as calling `Info()`
with no set level will make configuration and filtering of logging via the command
line more difficult.

The default is to not log `Info()` calls. This can be changed using the plugin command
line `-v` parameter. The additional annotations prepended to log lines by 'klog' can be disabled
with the `-skip_headers` option.

### Error Conventions

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

### Checklist for new device plugins

For new device plugins contributed to this repository, below is a
checklist to get the plugin on par feature and quality wise with
others:

1. Plugin binary available in [`cmd/`](cmd), its corresponding Dockerfile in [`build/docker/`](build/docker) and deployment Kustomization/YAMLs in [`deployments/`](deployments).
2. Plugin binary Go unit tests implemented and passing with >80% coverage: `make test WHAT=./cmd/<plugin>`.
3. Plugin binary linter checks passing: `make lint`.
4. Plugin e2e tests implemented in [`test/e2e/`](test/e2e) and passing: `go test -v ./test/e2e/... -args -ginkgo.focus "<plugin>"`.
5. Plugin CRD API added to [`pkg/apis/deviceplugin/v1`](pkg/apis/deviceplugin/v1) and CRDs generated: `make generate`.
6. Plugin CRD validation tests implemented in [`test/envtest/`](test/envtest) and passing: `make envtest`.
7. Plugin CRD controller implemented in [`pkg/controllers/`](pkg/controllers) and added to the manager in `cmd/operator/main.go`.
8. Plugin documentation written `cmd/<plugin>/README.md` and optionally end to end demos created in [`demo`](demo).

## How to build against a newer version of Kubernetes

First you need to update module dependencies. The easiest way is to use the
script copied from https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-521493597:

```bash
#!/bin/sh
set -euo pipefail

VERSION=${1#"v"}
if [ -z "$VERSION" ]; then
    echo "Must specify version!"
    exit 1
fi
MODS=($(
    curl -sS https://raw.githubusercontent.com/kubernetes/kubernetes/v${VERSION}/go.mod |
    sed -n 's|.*k8s.io/\(.*\) => ./staging/src/k8s.io/.*|k8s.io/\1|p'
))
for MOD in "${MODS[@]}"; do
    V=$(
        go mod download -json "${MOD}@kubernetes-${VERSION}" |
        sed -n 's|.*"Version": "\(.*\)".*|\1|p'
    )
    go mod edit "-replace=${MOD}=${MOD}@${V}"
done
go get "k8s.io/kubernetes@v${VERSION}"
```

Just run it inside the repo's root, e.g.

```
$ ./k8s_gomod_update.sh 1.18.1
```
Finally run

```
$ make generate
$ make test
```

and fix all new compilation issues.

## How to publish a new version of the Intel Device Plugins operator

Generate package manifests with:
```
$ make packagemanifests OPERATOR_VERSION=0.X.Y
```

Verify the operator deployment works OK via OLM in your development cluster:
```
$ operator-sdk olm install
$ kubectl create namespace testoperator
$ operator-sdk run packagemanifests --namespace testoperator --version 0.X.Y
# do verification checks
...
# do clean up
$ operator-sdk cleanup intel-device-plugins-operator --namespace testoperator
$ kubectl delete namespace testoperator
$ operator-sdk olm uninstall
```

Review the package manifests by uploading the generated `packagemanifests` folder to
https://operatorhub.io -> Contribute -> Package Your Operator.

Clone the [Community Operators](https://github.com/operator-framework/community-operators) repo:
```
$ git clone https://github.com/operator-framework/community-operators
```

Copy the generated files to the Community Operators repo:
```
$ cp -r packagemanifests/* community-operators/upstream-community-operators/intel-device-plugins/
```

Finally, submit a PR.
