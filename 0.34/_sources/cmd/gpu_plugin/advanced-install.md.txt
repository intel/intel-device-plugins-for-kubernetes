# Alternative installation methods for Intel GPU plugin

## Install to all nodes

In case the target cluster will not have NFD (or you don't want to install it), Intel GPU plugin can be installed to all nodes. This installation method will consume little unnecessary CPU resources on nodes without Intel GPUs.

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin?ref=<RELEASE_VERSION>'
```

## Install to nodes via NFD, with Monitoring and Shared-dev

Intel GPU plugin is installed via NFD's labels and node selector. Plugin is configured with monitoring and shared devices enabled. This option is useful when there is a desire to retrieve GPU metrics from nodes. For example with [XPU-Manager](https://github.com/intel/xpumanager/) or [collectd](https://github.com/collectd/collectd/tree/collectd-6.0).

```bash
# Start NFD - if your cluster doesn't have NFD installed yet
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>'

# Create NodeFeatureRules for detecting GPUs on nodes
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>'

# Create GPU plugin daemonset
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin/overlays/monitoring_shared-dev_nfd/?ref=<RELEASE_VERSION>'
```
