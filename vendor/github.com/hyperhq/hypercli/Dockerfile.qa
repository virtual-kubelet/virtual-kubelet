FROM centos:7.3.1611

#This Dockerfile is used for autotest hypercli
#REF: integration-cli/README.md

##########################################################################
RUN yum install -y\
 automake\
 gcc\
 wget\
 time\
 git


## Install Go
ENV GO_VERSION 1.8.3
RUN wget http://golangtc.com/static/go/${GO_VERSION}/go${GO_VERSION}.linux-amd64.tar.gz
#RUN wget http://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz
RUN tar -xzf go${GO_VERSION}.linux-amd64.tar.gz -C /usr/local

## Env
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go:/go/src/github.com/hyperhq/hypercli/integration-cli/vendor:/go/src/github.com/hyperhq/hypercli/vendor

ENV HYPER_CONFIG=/root/.hyper
ENV DOCKER_REMOTE_DAEMON=1
ENV DOCKER_CERT_PATH=fixtures/hyper_ssl
ENV DOCKER_TLS_VERIFY=

ENV DOCKER_HOST="tcp://us-west-1.hyper.sh:443"
## if BRANCH start with '#', then it means PR number, otherwise it means branch name
ENV BRANCH="master"

ENV ACCESS_KEY=
ENV SECRET_KEY=
ENV REGION=

RUN mkdir -p /go/src/github.com/hyperhq
WORKDIR /go/src/github.com/hyperhq

ADD hack/generate-hyper-conf-qa.sh /generate-hyper-conf-qa.sh
ENTRYPOINT ["/generate-hyper-conf-qa.sh"]
