module github.com/virtual-kubelet/virtual-kubelet

go 1.12

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/google/go-cmp v0.3.1
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/gorilla/mux v1.7.0
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	go.opencensus.io v0.21.0
	golang.org/x/sys v0.0.0-20190826190057-c7b8b68b1456
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.6
)

replace k8s.io/legacy-cloud-providers => github.com/kubernetes/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/cloud-provider => github.com/kubernetes/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/cli-runtime => github.com/kubernetes/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/apiserver => github.com/kubernetes/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/csi-translation-lib => github.com/kubernetes/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/cri-api => github.com/kubernetes/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/kube-aggregator => github.com/kubernetes/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/kubelet => github.com/kubernetes/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/kube-controller-manager => github.com/kubernetes/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/apimachinery => github.com/kubernetes/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/cluster-bootstrap => github.com/kubernetes/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/kube-proxy => github.com/kubernetes/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/component-base => github.com/kubernetes/kubernetes/staging/src/k8s.io/component-base v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/kube-scheduler => github.com/kubernetes/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/apiextensions-apiserver => github.com/kubernetes/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/metrics => github.com/kubernetes/kubernetes/staging/src/k8s.io/metrics v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/sample-apiserver => github.com/kubernetes/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/code-generator => github.com/kubernetes/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/client-go => github.com/kubernetes/kubernetes/staging/src/k8s.io/client-go v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/kubectl => github.com/kubernetes/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20200520130745-d32e40e20d16

replace k8s.io/api => github.com/kubernetes/kubernetes/staging/src/k8s.io/api v0.0.0-20200520130745-d32e40e20d16
