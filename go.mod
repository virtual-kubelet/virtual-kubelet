module github.com/virtual-kubelet/virtual-kubelet

go 1.15

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/bombsimon/logrusr v1.0.0
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
	go.opencensus.io v0.22.2
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20201112073958-5cba982894dd
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/api v0.15.1 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.19.10
	k8s.io/apimachinery v0.19.10
	k8s.io/apiserver v0.19.10
	k8s.io/client-go v0.19.10
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.2.0
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/controller-runtime v0.7.1
)
