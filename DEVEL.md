# Instructions for Device Plugin Development and Maintenance

Table of Contents

* [Day-to-day Development How to's](#day-to-day-development)
   * [Get the Source Code](#get-the-source-code)
   * [Build and Run Plugin Binaries](#build-and-run-plugin-binaries)
   * [Build Container Images](#build-container-images)
   * [Build Against a Newer Version of Kubernetes](#build-against-a-newer-version-of-kubernetes)
   * [Work with Intel Device Plugins Operator Modifications](#work-with-intel-device-plugins-operator-modifications)
   * [Publish a New Version of the Intel Device Plugins Operator to operatorhub.io](#publish-a-new-version-of-the-intel-device-plugins-operator-to-operatorhubio)
   * [Run E2E Tests](#run-e2e-tests)
   * [Run Controller Tests with a Local Control Plane](#run-controller-tests-with-a-local-control-plane)
* [How to Develop Simple Device Plugins](#how-to-develop-simple-device-plugins)
    * [Logging](#logging)
    * [Error Conventions](#error-conventions)
* [Checklist for New Device Plugins](#checklist-for-new-device-plugins)

## Day-to-day Development How to's
### Get the Source Code

With `git` installed on the system, just clone the repository:

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Build and Run Plugin Binaries

With `go` development environment installed on the system, build the plugin:

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make <plugin-build-target>
```

**Note:** All the available plugin build targets is roughly the output of `ls ${INTEL_DEVICE_PLUGINS_SRC}/cmd`.

To test the plugin binary on the development system, run as administrator:

```bash
$ sudo -E ${INTEL_DEVICE_PLUGINS_SRC}/cmd/<plugin-build-target>/<plugin-build-target>
```

### Build Container Images

The dockerfiles are generated on the fly from `.in` suffixed files and `.docker` include-snippets which are stitched together with
cpp preprocessor. You need to install cpp for that, e.g. in ubuntu it is found from build-essential (sudo apt install build-essential).
Don't edit the generated dockerfiles. Edit the inputs.

The simplest way to build all the docker images, is:
```
$ make images
```

But it is very slow. You can drastically speed it up by first running once:
```
$ make vendor
```

Which brings the libraries into the builder container without downloading them again and again for each plugin.

But it is still slow. You can further speed it up by first running once:
```
$ make licenses
```

Which pre-creates the go-licenses for all plugins, instead of re-creating them for each built plugin, every time.

But it is still rather slow to build all the images, and unnecessary, if you iterate on just one. Instead, build just the one you are iterating on, example:

```
$ make <image-build-target>
```

**Note:** All the available image build targets is roughly the output of `ls ${INTEL_DEVICE_PLUGINS_SRC}/build/docker/*.Dockerfile`.

If you iterate on only one plugin and if you know what its target cmd is (see folder `cmd/`), you can opt to pre-create just its licenses, example:
```
$ make licenses/<plugin-build-target>
```

The container image target names in the Makefile are derived from the `.Dockerfile.in` suffixed filenames under folder `build/docker/templates/`.

Recap:
```
$ make vendor
$ make licenses (or just make licenses/<plugin-build-target>)
$ make <image-build-target>
```

Repeat the last step only, unless you change library dependencies. If you pull in new sources, start again from `make vendor`.

**Note:** The image build tool can be changed from the default `docker` by setting the `BUILDER` argument
to the [`Makefile`](Makefile): `make <image-build-target> BUILDER=<builder>`. Supported values are `docker`, `buildah`, and `podman`.

### Build Against a Newer Version of Kubernetes

First, you need to update module dependencies. The easiest way is to use
`scripts/upgrade_k8s.sh` copied [from a k/k issue](https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-521493597):

Just run it inside the repo's root, e.g.

```
$ ${INTEL_DEVICE_PLUGINS_SRC}/scripts/upgrade_k8s.sh <k8s version>
```
Finally, run:

```
$ make generate
$ make test
```

and fix all new compilation issues.

### Work with Intel Device Plugins Operator Modifications

There are few useful steps when working with changes to Device Plugins CRDs and controllers:

1. Install controller-gen: `GO111MODULE=on go get -u sigs.k8s.io/controller-tools/cmd/controller-gen@<release ver>, e.g, v0.4.1`
2. Generate CRD and Webhook artifacts: `make generate`
3. Test local changes using [envtest](https://book.kubebuilder.io/reference/envtest.html): `make envtest`
4. Build a custom operator image: `make intel-deviceplugin-operator`
5. (Un)deploy operator: `kubectl [apply|delete] -k deployments/operator/default`

### Publish a New Version of the Intel Device Plugins Operator to operatorhub.io

Check if the fields mentioned below in the [base CSV manifest file](deployments/operator/manifests/bases/intel-device-plugins-operator.clusterserviceversion.yaml) have the correct values. If not, fix them manually (operator-sdk does not support updating these fields in any other way).
- spec.version
- spec.replaces
- metadata.annotations.containerImage
- metadata.annotations.createdAT

Check if [manager yaml file](deployments/operator/manager/manager.yaml) `spec.template.spec.containers.env` has correct sha256 digest for each plugin image.

Fork the [Community Operators](https://github.com/k8s-operatorhub/community-operators) repo and clone it:
```
$ git clone https://github.com/<GitHub Username>/community-operators
```

Generate bundle and build bundle image:
```
$ make bundle TAG=0.X.Y CHANNELS=alpha DEFAULT_CHANNEL=alpha
$ make bundle-build
```

Push the image to a registry:
- If pushing to the Docker hub, specify `docker.io/` in front of the image name for running bundle.
- If pushing to the local registry, put the option `--use-http` for running bundle.

Verify the operator deployment works OK via OLM in your development cluster:
```
$ operator-sdk olm install
$ kubectl create namespace testoperator
$ operator-sdk run bundle <Registry>:<Tag> -n testoperator
# do verification checks
...
# do clean up
$ operator-sdk cleanup intel-device-plugins-operator --namespace testoperator
$ kubectl delete namespace testoperator
$ operator-sdk olm uninstall
```

Commit files:
```
$ cd community-operators
$ git add operators/intel-device-plugins-operator/0.X.Y
$ git commit -am 'operators intel-device-plugins-operator (0.X.Y)' -s
```

Submit a PR to [Community Operators](https://github.com/k8s-operatorhub/community-operators) repo.

Check operator page
https://operatorhub.io/operator/intel-device-plugins-operator
after PR is merged.

### Run E2E Tests

Currently the E2E tests require having a Kubernetes cluster already configured
on the nodes with the hardware required by the device plugins. Also all the
container images with the executables under test must be available in the
cluster. If these two conditions are satisfied, run the tests with:

```bash
# Run all e2e tests in this repository
go test -v ./test/e2e/...
```

If you need to specify paths to your custom `kubeconfig` containing
embedded authentication info then add the `-kubeconfig` argument:

```bash
go test -v ./test/e2e/... -args -kubeconfig /path/to/kubeconfig
```

The full list of available options can be obtained with:

```bash
go test ./test/e2e/... -args -help
```

In most cases, it would not be possible to run all E2E tests in one system.
For running a subset of tests, there are labels that you can use to pick out specific parts.
You can run the tests with:
```bash
# Run a subset of tests
go test -v ./test/e2e/... -args -ginkgo.focus <labels in regex> -ginkgo.skip <labels in regex>
```

#### Table of Labels

| Device | Mode             | Resource    | App                            |
|:-------|:-----------------|:------------|:-------------------------------|
| `dlb`  |-                 | `pf`, `vf`  | `libdlb`                       |
| `dsa`  |-                 | `dedicated` | `accel-config`                 |
| `fpga` | `af`, `region`   |             | `opae-nlb-demo`                |
| `gpu`  |-                 | `i915`      | `busybox`, `tensorflow`        |
| `iaa`  |-                 | `dedicated` | `accel-config`                 |
| `qat`  | `dpdk`           | `dc`        | `openssl`, `compress-perf`     |
| `qat`  | `dpdk`           | `cy`        | `openssl`, `crypto-perf`       |
| `qat`  | `kernel`         | `cy1_dc0`   | `busybox`                      |
| `sgx`  |-                 |             | `sgx-sdk-demo`                 |

#### Examples

```bash
# DLB for VF resource without any app running
go test -v ./test/e2e/... -args -ginkgo.focus "Device:dlb.*Resource:vf.*App:noapp"

# FPGA with af mode with opae-nlb-demo app running
go test -v ./test/e2e/... -args -ginkgo.focus "Device:fpga.*Mode:af.*App:opae-nlb-demo"

# GPU with running only tensorflow app
go test -v ./test/e2e/... -args -ginkgo.focus "Device:gpu.*App:tensorflow"
#or
go test -v ./test/e2e/... -args -ginkgo.focus "Device:gpu" -ginkgo.skip "App:busybox"

# QAT for qat4 cy resource with openssl app running
go test -v ./test/e2e/... -args -ginkgo.focus "Device:qat.*Resource:cy.*App:openssl"

# SGX without running sgx-sdk-demo app
go test -v ./test/e2e/... -args -ginkgo.focus "Device:sgx" -ginkgo.skip "App:sgx-sdk-demo"

# All of Sapphire Rapids device plugins
go test -v ./test/e2e/... -args -ginkgo.focus "Device:(dlb|dsa|iaa|qat|sgx)"
```

## Predefined E2E Tests

It is possible to run predefined e2e tests with:
```
make e2e-<device> [E2E_LEVEL={basic|full}] [FOCUS=<labels in regex>] [SKIP=<labels in regex>]
```  

| `E2E_LEVEL`   | Equivalent `FOCUS` or `SKIP` | Explanation                                                                                      |
:-------------- |:---------------------------- |:------------------------------------------------------------------------------------------------ |
| `basic`       | `FOCUS=App:noapp`            | `basic` does not run any app pod, but checks if the plugin works and the resources are available |
| `full`        | `SKIP=App:noapp`             | `full` checks all resources, runs all apps except the spec kept for no app running               | 

### Examples

```bash
# DLB for both of pf and vf resources with running libdlb app
make e2e-dlb E2E_LEVEL=full

# QAT for cy resource with running only openssl app
make e2e-qat FOCUS=Resource:cy.*App:openssl

# QAT for dc resource without running any app
make e2e-qat E2E_LEVEL=basic FOCUS=Resource:dc

# GPU without running tensorflow app
make e2e-gpu E2E_LEVEL=full SKIP=tensorflow
```

It is also possible to run the tests which don't depend on hardware
without a pre-configured Kubernetes cluster. Just make sure you have
[Kind](https://kind.sigs.k8s.io/) installed on your host and run:

```
make test-with-kind
```

### Run Controller Tests with a Local Control Plane

The controller-runtime library provides a package for integration testing by
starting a local control plane. The package is called
[envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest). The
operator uses this package for its integration testing.

For setting up the environment for testing, `setup-envtest` can be used:

```bash
$ go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
$ setup-envtest use <K8S_VERSION>
$ KUBEBUILDER_ASSETS=$(setup-envtest use -i -p path <K8S_VERSION>) make envtest
```
## How to Develop Simple Device Plugins

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

In case you want to implement the whole allocation functionality in your
device plugin, you can implement the optional `deviceplugin.Allocator`
interface. In this case `PostAllocate()` is not called. But if you decide in your
implementation of `deviceplugin.Allocator` that you need to resort to the default
implementation of the allocation functionality then return an error of the type
`deviceplugin.UseDefaultMethodError`.

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

## Checklist for New Device Plugins

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
9. Plugin [`NodeFeatureRule`](deployments/nfd/overlays/node-feature-rules)s added for Node Feature Discovery labeling.
