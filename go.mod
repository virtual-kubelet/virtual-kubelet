module github.com/virtual-kubelet/virtual-kubelet

go 1.12

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190630181448-f1e96bc0f4c5 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190630181448-f1e96bc0f4c5 // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/google/go-cmp v0.2.0
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gorilla/mux v1.7.3
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/virtual-kubelet/node-cli v0.1.1
	go.opencensus.io v0.21.0
	google.golang.org/appengine v1.5.0 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20190627205229-acea843d18eb
	k8s.io/apimachinery v0.0.0-20190629125103-05b5762916b3
	k8s.io/apiserver v0.0.0-20190702125801-bf12d466f628 // indirect
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	k8s.io/kubernetes v1.13.7
	k8s.io/utils v0.0.0-20190607212802-c55fbcfc754a // indirect
)

replace k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
