module github.com/virtual-kubelet/virtual-kubelet

require (
	contrib.go.opencensus.io/exporter/ocagent v0.3.0
	contrib.go.opencensus.io/exporter/stackdriver v0.9.1 // indirect
	git.apache.org/thrift.git v0.0.0-20180916235629-12f8b14fff98 // indirect
	github.com/Azure/azure-sdk-for-go v21.1.0+incompatible
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Azure/go-autorest v10.15.5+incompatible
	github.com/BurntSushi/toml v0.3.0
	github.com/Microsoft/go-winio v0.4.7 // indirect
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/PuerkitoBio/purell v1.1.0 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/SAP/go-hdb v0.13.2 // indirect
	github.com/SermoDigital/jose v0.9.1 // indirect
	github.com/Sirupsen/logrus v1.0.5
	github.com/aliyun/alibaba-cloud-sdk-go v0.0.0-20180828111155-cad214d7d71f
	github.com/araddon/gou v0.0.0-20190110011759-c797efecbb61 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20180315120708-ccb8e960c48f // indirect
	github.com/aws/aws-sdk-go v1.15.31
	github.com/bitly/go-hostpool v0.0.0-20171023180738-a3a6125de932 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/boombuler/barcode v1.0.0 // indirect
	github.com/briankassouf/jose v0.9.1 // indirect
	github.com/cenkalti/backoff v2.0.0+incompatible
	github.com/census-instrumentation/opencensus-proto v0.0.2 // indirect
	github.com/centrify/cloud-golang-sdk v0.0.0-20180119173102-7c97cc6fde16 // indirect
	github.com/chrismalek/oktasdk-go v0.0.0-20181212195951-3430665dfaa0 // indirect
	github.com/cloudfoundry-incubator/candiedyaml v0.0.0-20170901234223-a41693b7b7af // indirect
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808 // indirect
	github.com/coreos/bbolt v1.3.2 // indirect
	github.com/coreos/etcd v3.3.11+incompatible // indirect
	github.com/coreos/go-oidc v2.0.0+incompatible // indirect
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20181031085051-9002847aa142 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/cpuguy83/strongerrors v0.2.1
	github.com/dancannon/gorethink v4.0.0+incompatible // indirect
	github.com/denisenkom/go-mssqldb v0.0.0-20190121005146-b04fd42d9952 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/dimchansky/utfbom v0.0.0-20170328061312-6c6132ff69f0
	github.com/docker/distribution v2.6.2+incompatible // indirect
	github.com/docker/docker v1.13.0
	github.com/docker/engine-api v0.4.0 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/docker/go-units v0.3.3 // indirect
	github.com/docker/libnetwork v0.5.6 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/duosecurity/duo_api_golang v0.0.0-20190107154727-539434bf0d45 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0 // indirect
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20180613152042-8306686428a5 // indirect
	github.com/gammazero/deque v0.0.0-20190130191400-2afb3858e9c7 // indirect
	github.com/gammazero/workerpool v0.0.0-20181230203049-86a96b5d5d92 // indirect
	github.com/garyburd/redigo v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-ini/ini v1.36.0 // indirect
	github.com/go-ldap/ldap v3.0.1+incompatible // indirect
	github.com/go-openapi/analysis v0.0.0-20161231055341-d5a75b7d751c // indirect
	github.com/go-openapi/errors v0.0.0-20170104180542-fc3f73a22449 // indirect
	github.com/go-openapi/jsonpointer v0.0.0-20170102174223-779f45308c19 // indirect
	github.com/go-openapi/jsonreference v0.0.0-20161105162150-36d33bfe519e // indirect
	github.com/go-openapi/loads v0.0.0-20170118010502-6bb6486231e0 // indirect
	github.com/go-openapi/runtime v0.0.0-20170120020924-3b13ebb46790 // indirect
	github.com/go-openapi/spec v0.0.0-20171127190025-bfb48d37839b // indirect
	github.com/go-openapi/strfmt v0.0.0-20170112235747-0cb3db44c13b // indirect
	github.com/go-openapi/swag v0.0.0-20171111214437-cf0bdb963811 // indirect
	github.com/go-openapi/validate v0.0.0-20170117155836-035dcd74f1f6 // indirect
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/go-stomp/stomp v2.0.2+incompatible // indirect
	github.com/go-test/deep v1.0.1 // indirect
	github.com/gocql/gocql v0.0.0-20190126123547-8516aabb0f99 // indirect
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/gogo/protobuf v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/google/go-github v17.0.0+incompatible // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf // indirect
	github.com/google/uuid v0.0.0-20161128191214-064e2069ce9c
	github.com/googleapis/gax-go v2.0.2+incompatible // indirect
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gorhill/cronexpr v0.0.0-20140423231348-a557574d6c02 // indirect
	github.com/gorilla/context v0.0.0-20160226214623-1ea25387ff6f // indirect
	github.com/gorilla/mux v1.6.1
	github.com/gorilla/websocket v1.2.0
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.7.0 // indirect
	github.com/hashicorp/consul v1.4.0 // indirect
	github.com/hashicorp/go-gcp-common v0.0.0-20180425173946-763e39302965 // indirect
	github.com/hashicorp/go-hclog v0.0.0-20190109152822-4783caec6f2e // indirect
	github.com/hashicorp/go-memdb v0.0.0-20181108192425-032f93b25bec // indirect
	github.com/hashicorp/go-plugin v0.0.0-20190129155509-362c99b11937 // indirect
	github.com/hashicorp/go-retryablehttp v0.5.0 // indirect
	github.com/hashicorp/go-rootcerts v0.0.0-20160503143440-6bb64b370b90 // indirect
	github.com/hashicorp/go-version v1.0.0 // indirect
	github.com/hashicorp/hcl v0.0.0-20180404174102-ef8a98b0bbce // indirect
	github.com/hashicorp/memberlist v0.1.3 // indirect
	github.com/hashicorp/nomad v0.8.6
	github.com/hashicorp/raft v1.0.0 // indirect
	github.com/hashicorp/serf v0.8.1 // indirect
	github.com/hashicorp/vault v1.0.1 // indirect
	github.com/hashicorp/vault-plugin-auth-alicloud v0.0.0-20181109180636-f278a59ca3e8 // indirect
	github.com/hashicorp/vault-plugin-auth-azure v0.0.0-20190201222632-0af1d040b5b3 // indirect
	github.com/hashicorp/vault-plugin-auth-centrify v0.0.0-20180816201131-66b0a34a58bf // indirect
	github.com/hashicorp/vault-plugin-auth-gcp v0.0.0-20190201215414-7d4c2101e7d0 // indirect
	github.com/hashicorp/vault-plugin-auth-jwt v0.0.0-20190128234440-a608a5ad1c24 // indirect
	github.com/hashicorp/vault-plugin-auth-kubernetes v0.0.0-20190201222209-db96aa4ab438 // indirect
	github.com/hashicorp/vault-plugin-secrets-ad v0.0.0-20190131222416-4796d9980125 // indirect
	github.com/hashicorp/vault-plugin-secrets-alicloud v0.0.0-20190131211812-b0abe36195cb // indirect
	github.com/hashicorp/vault-plugin-secrets-azure v0.0.0-20181207232500-0087bdef705a // indirect
	github.com/hashicorp/vault-plugin-secrets-gcp v0.0.0-20180921173200-d6445459e80c // indirect
	github.com/hashicorp/vault-plugin-secrets-gcpkms v0.0.0-20190116164938-d6b25b0b4a39 // indirect
	github.com/hashicorp/vault-plugin-secrets-kv v0.0.0-20190115203747-edbfe287c5d9 // indirect
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d // indirect
	github.com/hyperhq/hyper-api v0.0.0-20171127032913-18c77d3f9fe0
	github.com/hyperhq/hypercli v0.0.0-20180414054930-29217d318cab
	github.com/hyperhq/libcompose v0.0.0-20170620062718-15d3a105140f // indirect
	github.com/imdario/mergo v0.3.4 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jeffchao/backoff v0.0.0-20140404060208-9d7fd7aa17f2 // indirect
	github.com/jefferai/jsonx v1.0.0 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/jtolds/gls v4.2.1+incompatible // indirect
	github.com/keybase/go-crypto v0.0.0-20181127160227-255a5089e85a // indirect
	github.com/kr/pretty v0.1.0
	github.com/lawrencegripper/pod2docker v0.5.1
	github.com/lib/pq v1.0.0 // indirect
	github.com/magiconair/properties v1.7.6 // indirect
	github.com/mailru/easyjson v0.0.0-20180606163543-3fdea8d05856 // indirect
	github.com/mattbaird/elastigo v0.0.0-20170123220020-2fe47fd29e4b // indirect
	github.com/michaelklishin/rabbit-hole v1.4.0 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-homedir v0.0.0-20161203194507-b8bc1bf76747
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/mitchellh/mapstructure v0.0.0-20180220230111-00c29f56e238 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/onsi/ginkgo v1.7.0 // indirect
	github.com/onsi/gomega v1.4.3 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/opencontainers/runtime-spec v0.0.0-20160926201932-1c7c27d043c2 // indirect
	github.com/ory-am/common v0.4.0 // indirect
	github.com/ory/dockertest v3.3.4+incompatible // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pborman/uuid v0.0.0-20170612153648-e790cca94e6c // indirect
	github.com/pelletier/go-toml v1.1.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pierrec/lz4 v0.0.0-20181005164709-635575b42742 // indirect
	github.com/pkg/errors v0.8.0
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/pquerna/otp v1.1.0 // indirect
	github.com/ryanuber/go-glob v0.0.0-20160226084822-572520ed46db // indirect
	github.com/samuel/go-zookeeper v0.0.0-20180130194729-c4fab1ac1bec // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/sirupsen/logrus v1.3.0 // indirect
	github.com/smartystreets/assertions v0.0.0-20190116191733-b6c0e53d7304 // indirect
	github.com/smartystreets/goconvey v0.0.0-20181108003508-044398e4856c // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/afero v1.1.0 // indirect
	github.com/spf13/cast v1.2.0 // indirect
	github.com/spf13/cobra v0.0.2
	github.com/spf13/jwalterweatherman v0.0.0-20180109140146-7c0cea34c8ec // indirect
	github.com/spf13/pflag v1.0.1 // indirect
	github.com/spf13/viper v1.0.2
	github.com/stevvooe/resumable v0.0.0-20180830230917-22b14a53ba50 // indirect
	github.com/streadway/amqp v0.0.0-20181205114330-a314942b2fd9 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/tchap/go-patricia v2.2.6+incompatible // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/ugorji/go v0.0.0-20170620060102-0053ebfd9d0e // indirect
	github.com/vbatts/tar-split v0.10.2 // indirect
	github.com/vishvananda/netlink v0.0.0-20161119221931-482f7a52b758 // indirect
	github.com/vishvananda/netns v0.0.0-20171111001504-be1fbeda1936 // indirect
	github.com/vmware/govmomi v0.17.1 // indirect
	github.com/vmware/vic v0.0.0-20180820162446-c7d40ac878b0
	github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20170528113821-0c8571ac0ce1 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.2 // indirect
	go.opencensus.io v0.17.0
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20190103213133-ff983b9c42bc // indirect
	golang.org/x/net v0.0.0-20181023162649-9b4f9f5ad519
	golang.org/x/oauth2 v0.0.0-20181003184128-c57b0facaced // indirect
	golang.org/x/sync v0.0.0-20181221193216-37e7f081c4d4
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2 // indirect
	google.golang.org/api v0.0.0-20180916000451-19ff8768a5c0 // indirect
	google.golang.org/appengine v1.2.0 // indirect
	google.golang.org/grpc v1.15.0
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/gorethink/gorethink.v4 v4.1.0 // indirect
	gopkg.in/ini.v1 v1.41.0 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/ory-am/dockertest.v2 v2.2.3 // indirect
	gopkg.in/square/go-jose.v2 v2.2.2 // indirect
	gopkg.in/yaml.v2 v2.2.1
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.0.0-20181213150558-05914d821849
	k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93
	k8s.io/apiserver v0.0.0-20180727190041-25e79651c7e5 // indirect
	k8s.io/client-go v0.0.0-20181213151034-8d9ed539ba31
	k8s.io/klog v0.1.0 // indirect
	k8s.io/kube-openapi v0.0.0-20181114233023-0317810137be // indirect
	k8s.io/kubernetes v1.12.1
	k8s.io/utils v0.0.0-20180801164400-045dc31ee5c4 // indirect
	layeh.com/radius v0.0.0-20190118135028-0f678f039617 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)
