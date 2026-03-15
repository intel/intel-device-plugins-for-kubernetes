# Monitoring GPUs

## Monitoring mode

GPU plugin has moved to single monitoring resource: `gpu.intel.com/monitoring`. It includes both `i915` and `xe` KMD devices. In case, the old behaviour is required, the plugin can be started with `-monitoring-mode=split` argument, which brings back the old `i915` and `xe` resources.

The problem with the split monitoring resources is that Pod scheduling becomes difficult for nodes which have both devices. Especially if the cluster has nodes with only `xe` devices, and nodes with both `xe` and `i915` devices. Nodes with integrated GPUs are still mostly using `i915` while new GPUs are using `xe`. To get around this, one can use node selectors etc. to guide scheduling, but using a single monitoring resource for all fixes it.

## Monitoring resource

GPU plugin can be configured to register a monitoring resource for the nodes that have Intel GPUs on them. `gpu.intel.com/monitoring` is a singular resource on the nodes. A container requesting it, will get access to _all_ the Intel GPUs (`i915` and `xe` KMD device files) on the node. The idea behind this resource is to allow the container to _monitor_ the GPUs. A container requesting the `monitoring` resource would typically export data to some metrics consumer. An example for such a consumer is [Prometheus](https://prometheus.io/).

<figure>
  <img src="monitoring.png"/>
  <figcaption>Monitoring Pod listening to all GPUs while one Pod is using a GPU.</figcaption>
</figure>

For the monitoring applications, there are two possibilities: [Intel XPU Manager](https://github.com/intel/xpumanager/) and [collectd](https://github.com/collectd/collectd/tree/collectd-6.0). Intel XPU Manager is readily available as a container and with a deployment yaml. collectd has Intel GPU support in its 6.0 branch, but there are no public containers available for it.

To deploy XPU Manager to a cluster, one has to run the following kubectl:
```
$ kubectl apply -k https://github.com/intel/xpumanager/deployment/kubernetes/daemonset/base
& kubectl patch ds intel-xpumanager --type='strategic' -p '{"spec": {"template": {"spec": {"containers": [{"name": "xpumd","resources": {"limits": {"gpu.intel.com/monitoring": "1","gpu.intel.com/i915_monitoring": null}}}]}}}}'
```

This will deploy an XPU Manager daemonset to run on all the nodes having the `monitoring` resource.

## Prometheus integration with XPU Manager

For deploying Prometheus to a cluster, see [this page](https://prometheus-operator.dev/docs/user-guides/getting-started/). One can also use Prometheus' [helm chart](https://github.com/prometheus-community/helm-charts).

Prometheus requires additional Kubernetes configuration so it can fetch GPU metrics. The following steps will add a Kubernetes Service and a ServiceMonitor components. The components instruct Prometheus how and where from to retrieve the metrics.

```
$ kubectl apply -f https://raw.githubusercontent.com/intel/xpumanager/master/deployment/kubernetes/monitoring/service-intel-xpum.yaml
$ kubectl apply -f https://raw.githubusercontent.com/intel/xpumanager/master/deployment/kubernetes/monitoring/servicemonitor-intel-xpum.yaml
```

With those components in place, one can query Intel GPU metrics from Prometheus with `xpum_` prefix.
