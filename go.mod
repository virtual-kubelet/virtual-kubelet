module github.com/virtual-kubelet/virtual-kubelet

go 1.12

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/google/go-cmp v0.3.0
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/gorilla/mux v1.7.0
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.1
	github.com/spf13/cobra v0.0.2
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0 // indirect
	go.opencensus.io v0.21.0
	golang.org/x/crypto v0.0.0-20190325154230-a5d413f7728c // indirect
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3 // indirect
	golang.org/x/sys v0.0.0-20190403152447-81d4e9dc473e // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/genproto v0.0.0-20190404172233-64821d5d2107 // indirect
	google.golang.org/grpc v1.20.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/klog v0.3.1
	k8s.io/kube-openapi v0.0.0-20190510232812-a01b7d5d6c22 // indirect
	k8s.io/kubernetes v1.15.2
)

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190805144654-3d5bf3a310c1

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190805144409-8484242760e7

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190805143448-a07e59fb081d

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190805142138-368b2058237c

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190805144531-3985229e1802

replace k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190531030430-6117653b35f1

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190805142416-fd821fbbb94e

replace k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190805143852-517ff267f8d1

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190805144128-269742da31dd

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719

replace k8s.io/api => k8s.io/api v0.0.0-20190805141119-fdd30b57c827

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190805144246-c01ee70854a1

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190805143734-7f1675b90353

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20190805141645-3a5e5ac800ae

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190805144012-2a1ed1f3d8a4

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190805143126-cdb999c96590

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20190805143318-16b07057415d

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190805142637-3b65bc4bb24f

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190805141520-2fe0317bcee0
