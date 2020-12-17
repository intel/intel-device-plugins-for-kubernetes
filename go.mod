module github.com/intel/intel-device-plugins-for-kubernetes

go 1.15

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-ini/ini v1.46.0
	github.com/go-logr/logr v0.2.1
	github.com/google/gousb v1.1.0
	github.com/klauspost/cpuid/v2 v2.0.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	google.golang.org/grpc v1.27.0
	gopkg.in/ini.v1 v1.46.0 // indirect
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/component-base v0.19.3
	k8s.io/klog v1.0.0
	k8s.io/kubelet v0.19.3
	k8s.io/kubernetes v1.19.3
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/controller-runtime v0.7.0-alpha.6
)

replace (
	k8s.io/api => k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.4-rc.0
	k8s.io/apiserver => k8s.io/apiserver v0.19.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.3
	k8s.io/client-go => k8s.io/client-go v0.19.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.3
	k8s.io/code-generator => k8s.io/code-generator v0.19.4-rc.0
	k8s.io/component-base => k8s.io/component-base v0.19.3
	k8s.io/controller-manager => k8s.io/controller-manager v0.19.4-rc.0
	k8s.io/cri-api => k8s.io/cri-api v0.19.4-rc.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.3
	k8s.io/kubectl => k8s.io/kubectl v0.19.3
	k8s.io/kubelet => k8s.io/kubelet v0.19.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.3
	k8s.io/metrics => k8s.io/metrics v0.19.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.3
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.19.3
	k8s.io/sample-controller => k8s.io/sample-controller v0.19.3
)
