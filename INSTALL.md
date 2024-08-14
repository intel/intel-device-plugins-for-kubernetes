# Installing device plugins to cluster

## Install device plugins via a DaemonSet

Each plugin can be installed via a DaemonSet. The install changes slightly based on the desired plugin. See install instructions per [plugin](README.md#plugins).

Installing plugins via DaemonSets deployes them to the ```default``` (or currently active) namespace. Use kubectl's ```--namespace``` argument to change the deployment namespace.

## Install device plugins via device plugin operator

A more advanced install method is via device plugin operator. Operator configures plugin deployments based on the supplied device plugin CRDs (Custom Resource Definitions). See installation instructions in the [operator README](cmd/operator/README.md#installation).

Operator installs device plugins to the same namespace where the operator itself is deployed. The default operator namespace is ```inteldeviceplugins-system```.

## Install with HELM charts

Device plugins can also be installed to a cluster using the device plugin [operator Helm chart](https://github.com/intel/helm-charts/tree/main/charts/device-plugin-operator) (depending on cert-manager and NFD). Individual plugin projects are under https://github.com/intel/helm-charts/tree/main/charts/.

These steps will install device plugin operator and plugins under ```inteldeviceplugins-system``` namespace. It's possible to change the target namespace by changing the ```--namespace``` value in the helm install command.

### Installing HELM repositories

```bash
helm repo add jetstack https://charts.jetstack.io # for cert-manager
helm repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts # for NFD
helm repo add intel https://intel.github.io/helm-charts/ # for device-plugin-operator and plugins
helm repo update
```

### Installing cert-manager

```bash
helm install --wait \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.15.2 \
  --set installCRDs=true
```

NOTE: cert-manager install takes a while to complete.

### Installing NFD

```bash
helm install nfd nfd/node-feature-discovery \
  --namespace node-feature-discovery --create-namespace --version 0.16.4
```

### Installing operator

```bash
helm install dp-operator intel/intel-device-plugins-operator --namespace inteldeviceplugins-system --create-namespace
```

### Installing specific plugins

Replace PLUGIN with the desired plugin name. At least the following plugins are supported: **gpu, sgx, qat, dlb, dsa & iaa**.

```bash
helm install <PLUGIN> intel/intel-device-plugins-<PLUGIN> --namespace inteldeviceplugins-system --create-namespace \
  --set nodeFeatureRule=true
```

### Listing available versions

Use helm's search functionality to list available versions.

```bash
helm search repo intel/intel-device-plugins-operator --versions
helm search repo intel/intel-device-plugins-<plugin> --versions
```

For example, operator chart versions with development versions included.
```bash
$ helm search repo intel/intel-device-plugins-operator --versions --devel
NAME                               	CHART VERSION	APP VERSION	DESCRIPTION
intel/intel-device-plugins-operator	0.26.0       	0.26.0     	A Helm chart for Intel Device Plugins Operator ...
intel/intel-device-plugins-operator	0.25.1       	0.25.1     	A Helm chart for Intel Device Plugins Operator ...
intel/intel-device-plugins-operator	0.25.1-helm.0	0.25.0     	A Helm chart for Intel Device Plugins Operator ...
intel/intel-device-plugins-operator	0.25.0       	0.25.0     	A Helm chart for Intel Device Plugins Operator ...
intel/intel-device-plugins-operator	0.24.1       	0.24.1     	A Helm chart for Intel Device Plugins Operator ...
intel/intel-device-plugins-operator	0.24.1-helm.0	0.24.0     	A Helm chart for Intel Device Plugins Operator ...
intel/intel-device-plugins-operator	0.24.0       	0.24.0     	A Helm chart for Intel Device Plugins Operator ...
```

### Customizing plugins

To customize plugin features, see the available chart values:
```bash
helm show values intel/intel-device-plugins-<PLUGIN>
```

For example, qat plugin has these values:
```bash
$ helm show values intel/intel-device-plugins-qat
name: qatdeviceplugin-sample

image:
  hub: intel
  tag: ""

initImage:
  hub: intel
  tag: ""

dpdkDriver: vfio-pci
kernelVfDrivers:
  - 4xxxvf
  - 420xxvf
maxNumDevices: 128
logLevel: 4

nodeSelector:
  intel.feature.node.kubernetes.io/qat: 'true'

nodeFeatureRule: true
```

### Uninstall

Uninstall each installed component with ```helm uninstall```:

```bash
# repeat first step as many times as there are plugins installed
helm uninstall -n inteldeviceplugins-system <PLUGIN>
helm uninstall -n inteldeviceplugins-system dp-operator
helm uninstall -n node-feature-discovery nfd
helm uninstall -n cert-manager cert-manager
```
