## Helm charts

Make "helm" target provides a way to to create a versioned chart archive file:

```
$ make helm
```

The generated chart archives can be uploaded to helm package repository to enable further distribution. It is also needed to generate an index file and upload it to the repository.

```
helm repo index intel-device-plugins-operator-VERSION.tgz
```

To download a chart directly from a repository, use the following commands:

```
$ helm pull intel-device-plugins-chart-repo/intel-device-plugins-helm-chart
$ helm install intel-device-plugins intel-device-plugins-chart-repo/intel-device-plugins-helm-chart
```
