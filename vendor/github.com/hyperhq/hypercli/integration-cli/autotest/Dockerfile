FROM centos:7.3.1611

#This Dockerfile is used for autotest hypercli
#REF: integration-cli/README.md

###################################
##    install common package     ##
###################################
RUN yum install -y\
 automake \
 autoconf \
 make \
 gcc \
 wget \
 time \
 git \
 which \
 screen &&\
 yum clean all


########################################
##        prepare java run env        ##
########################################
RUN wget --no-check-certificate --no-cookies \
        --header "Cookie: oraclelicense=accept-securebackup-cookie" \
        http://download.oracle.com/otn-pub/java/jdk/8u141-b15/336fa29ff2bb4ef291e347e091f7f4a7/jdk-8u141-linux-x64.rpm \
		&& rpm -ivh jdk-8u141-linux-x64.rpm && rm -rf jdk-8u141-linux-x64.rpm

ENV JAVA_HOME /usr/java/jdk1.8.0_141
ENV PATH $PATH:$JAVA_HOME/bin


###########################
##    install golang     ##
###########################
ENV GO_VERSION 1.8.3
RUN wget http://golangtc.com/static/go/${GO_VERSION}/go${GO_VERSION}.linux-amd64.tar.gz
RUN tar -xzf go${GO_VERSION}.linux-amd64.tar.gz -C /usr/local
ENV GOROOT /usr/local/go
ENV PATH $GOROOT/bin:$PATH


##########################################
##    prepare jenkins slave run env     ##
##########################################
ENV HOME /home/jenkins
RUN groupadd -g 10000 jenkins
RUN useradd -c "Jenkins user" -d $HOME -u 10000 -g 10000 -m jenkins
RUN mkdir /home/jenkins/.tmp
VOLUME ["/home/jenkins"]

WORKDIR $HOME
USER root

################################
##    prepare for build env   ##
################################
## Env
ENV PATH /go/bin:/usr/local/go/bin:/usr/bin:/usr/local/bin:$PATH
ENV GOPATH /go:/go/src/github.com/hyperhq/hypercli/integration-cli/vendor:/go/src/github.com/hyperhq/hypercli/vendor

#TARGET_REGION could be: us-west-1|eu-central-1|RegionOne
ENV TARGET_REGION=${TARGET_REGION:-us-west-1}
ENV BRANCH=${BRANCH:-master}
ENV TEST_CASE_REG=${TEST_CASE_REG:-TestCli.*}

## hyper account for test
ENV ACCESS_KEY=
ENV SECRET_KEY=

## slack parameter
ENV SLACK_TOKEN=
ENV SLACK_CHANNEL_ID=


COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY script/slack.sh /usr/local/bin/slack.sh
COPY script/run.sh /usr/local/bin/run.sh

ENTRYPOINT ["entrypoint.sh"]
CMD ["run.sh"]