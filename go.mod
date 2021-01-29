module github.com/virtual-kubelet/virtual-kubelet

go 1.15

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/bombsimon/logrusr v1.0.0
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/gorilla/mux v1.7.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	go.opencensus.io v0.21.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200323222414-85ca7c5b95cd
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/apiserver v0.18.4
	k8s.io/client-go v0.18.6
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.0.0
	k8s.io/kubernetes v1.18.4
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	sigs.k8s.io/controller-runtime v0.6.3
)

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.4

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.4

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.4

replace k8s.io/apiserver => k8s.io/apiserver v0.18.4

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.4

replace k8s.io/cri-api => k8s.io/cri-api v0.18.4

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.4

replace k8s.io/kubelet => k8s.io/kubelet v0.18.4

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.4

replace k8s.io/apimachinery => k8s.io/apimachinery v0.18.4

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.4

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.4

replace k8s.io/component-base => k8s.io/component-base v0.18.4

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.4

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.4

replace k8s.io/metrics => k8s.io/metrics v0.18.4

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.4

replace k8s.io/code-generator => k8s.io/code-generator v0.18.4

replace k8s.io/client-go => k8s.io/client-go v0.18.4

replace k8s.io/kubectl => k8s.io/kubectl v0.18.4

replace k8s.io/api => k8s.io/api v0.18.4
