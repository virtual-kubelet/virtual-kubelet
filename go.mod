module github.com/virtual-kubelet/virtual-kubelet

go 1.15

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/bombsimon/logrusr v1.0.0
	github.com/coreos/go-etcd v2.0.0+incompatible // indirect
	github.com/cpuguy83/go-md2man v1.0.10 // indirect
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/gorilla/mux v1.7.3
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8 // indirect
	go.opencensus.io v0.22.2
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20201112073958-5cba982894dd
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.19.10
	k8s.io/apimachinery v0.19.10
	k8s.io/apiserver v0.19.10
	k8s.io/client-go v0.19.10
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.2.0
	k8s.io/kubernetes v1.19.10
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/controller-runtime v0.7.1
)

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.10

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.10

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.10

replace k8s.io/apiserver => k8s.io/apiserver v0.19.10

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.10

replace k8s.io/cri-api => k8s.io/cri-api v0.19.10

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.10

replace k8s.io/kubelet => k8s.io/kubelet v0.19.10

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.10

replace k8s.io/apimachinery => k8s.io/apimachinery v0.19.10

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.10

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.10

replace k8s.io/component-base => k8s.io/component-base v0.19.10

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.10

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.10

replace k8s.io/metrics => k8s.io/metrics v0.19.10

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.10

replace k8s.io/code-generator => k8s.io/code-generator v0.19.10

replace k8s.io/client-go => k8s.io/client-go v0.19.10

replace k8s.io/kubectl => k8s.io/kubectl v0.19.10

replace k8s.io/api => k8s.io/api v0.19.10
