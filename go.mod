module github.com/virtual-kubelet/virtual-kubelet

go 1.12

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/google/go-cmp v0.3.0
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.1
	github.com/spf13/cobra v0.0.2
	github.com/spf13/pflag v1.0.3
	go.opencensus.io v0.21.0
	golang.org/x/sync v0.0.0-20190227155943-e225da77a7e6
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/genproto v0.0.0-20190404172233-64821d5d2107 // indirect
	google.golang.org/grpc v1.20.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gotest.tools v0.0.0-20181223230014-1083505acf35
	k8s.io/api v0.0.0-20190722141453-b90922c02518
	k8s.io/apimachinery v0.0.0-20190719140911-bfcf53abc9f8
	k8s.io/apiserver v0.0.0-20181213151703-3ccfe8365421 // indirect
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/klog v0.3.1
	k8s.io/kubernetes v1.13.7
)

replace k8s.io/client-go => /Users/sargun/projects/client-go
