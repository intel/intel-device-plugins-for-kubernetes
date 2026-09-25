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
| `image.digest` | digest of `intel/intel-sgx-plugin:<appVersion>` | Image digest (`sha256:...`); when set, pins the image and overrides the tag |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `logLevel` | `2` | Log verbosity (`-v`) |
| `enclaveLimit` | `""` (plugin default) | Value for `-enclave-limit` (currently also used for `-provision-limit`) |
| `nodeSelector` | `{}` | Extra node selector labels, merged with `kubernetes.io/arch: amd64` |
