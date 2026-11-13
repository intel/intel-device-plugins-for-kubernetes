# intel-sgx-device-plugin

Helm chart for the [Intel SGX device plugin](../../cmd/sgx_plugin/README.md).
It deploys the same DaemonSet as `deployments/sgx_plugin/base`.

## Installing

```bash
helm install intel-sgx-plugin oci://ghcr.io/intel/intel-sgx-device-plugin --namespace kube-system
```

Pre-release chart versions (e.g. `0.37.0-alpha.1`) are not picked by default; pass `--version <version>` explicitly.

## Values

| Key | Default | Description |
|-----|---------|-------------|
| `image.repository` | `intel/intel-sgx-plugin` | Plugin image repository |
| `image.tag` | `""` (chart `appVersion`) | Plugin image tag |
| `image.digest` | digest of `intel/intel-sgx-plugin:<appVersion>` | Image digest (`sha256:...`); when set, appended as `repository:tag@digest` to pin the image |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `logLevel` | `2` | Log verbosity (`-v`) |
| `enclaveLimit` | `""` (plugin default) | Value for `-enclave-limit` (currently also used for `-provision-limit`) |
| `nodeSelector` | `{}` | Extra node selector labels, merged with `kubernetes.io/arch: amd64` |
| `nodeFeatureRule.enabled` | `false` | Create an NFD `NodeFeatureRule` that applies the node selector labels to SGX-capable nodes (see below) |
| `global.nodeSelector` | `{}` | Node selector labels shared with a parent chart; `nodeSelector` takes precedence on conflicting keys |

## Node Feature Discovery rule

With `nodeFeatureRule.enabled=true`, the chart creates a cluster-scoped
[NodeFeatureRule](https://kubernetes-sigs.github.io/node-feature-discovery/stable/usage/custom-resources.html#nodefeaturerule)
that detects SGX-capable nodes, labels them with the chart's node selector labels
(`nodeSelector` and `global.nodeSelector`, except `kubernetes.io/arch`) and advertises
`sgx.intel.com/epc`. The plugin DaemonSet then runs only on the labeled nodes:

```bash
helm install intel-sgx-plugin oci://ghcr.io/intel/intel-sgx-device-plugin --namespace kube-system \
  --set nodeFeatureRule.enabled=true \
  --set-string 'nodeSelector.intel\.feature\.node\.kubernetes\.io/sgx=true'
```

Node Feature Discovery and its CRDs must be installed. By default, NFD only creates labels in the
`feature.node.kubernetes.io` and `*.feature.node.kubernetes.io` namespaces; other label namespaces
must be allowed in the NFD configuration. Leave the rule disabled if an equivalent rule is already
deployed, for example from `deployments/nfd/overlays/node-feature-rules`.
