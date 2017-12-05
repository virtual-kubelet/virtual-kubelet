FROM centos:7.3.1611

#This Dockerfile is used for dev hypercli
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
ENV GOPATH /go:/go/src/github.com/hyperhq/hypercli/vendor

ENV HYPER_CONFIG=/root/.hyper
ENV DOCKER_REMOTE_DAEMON=1
ENV DOCKER_CERT_PATH=fixtures/hyper_ssl
ENV DOCKER_TLS_VERIFY=
ENV DOCKER_HOST=
ENV ACCESS_KEY=
ENV SECRET_KEY=
ENV REGION=

## Ensure /usr/bin/hyper
RUN ln -s /go/src/github.com/hyperhq/hypercli/hyper/hyper /usr/bin/hyper
RUN echo alias hypercli=\"hyper --region \${DOCKER_HOST}\" >> /root/.bashrc


## Ensure /go/src/github.com/docker/docker
RUN mkdir -p /go/src/github.com/docker
RUN ln -s /go/src/github.com/hyperhq/hypercli /go/src/github.com/docker/docker


WORKDIR /go/src/github.com/hyperhq/hypercli
VOLUME ["/go/src/github.com/hyperhq/hypercli"]
ENTRYPOINT ["hack/generate-hyper-conf-dev.sh"]


##########################################################################
# install on-my-zsh
RUN yum install -y zsh
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/robbyrussell/oh-my-zsh/master/tools/install.sh)"
RUN sed -i "s/^ZSH_THEME=.*/ZSH_THEME=\"gianu\"/g" /root/.zshrc
RUN echo alias hypercli=\"hyper --region \${DOCKER_HOST}\" >> /root/.zshrc

# config git
RUN git config --global color.ui true; \
    git config --global color.status auto; \
    git config --global color.diff auto; \
    git config --global color.branch auto; \
    git config --global color.interactive auto; \
    git config --global alias.st  'status'; \
    git config --global alias.ci  'commit'; \
    git config --global alias.co  'checkout'; \
    git config --global alias.br 'branch'; \
    git config --global alias.sr 'show-ref'; \
    git config --global alias.cm '!sh -c "br_name=`git symbolic-ref HEAD|sed s#refs/heads/##`; git commit -em \"[\${br_name}] \""'; \
    git config --global alias.lg "log --graph --pretty=format:'[%ci] %Cgreen(%cr) %Cred%h%Creset -%x09%C(yellow)%Creset %C(cyan)[%an]%Creset %x09 %s%Creset' --abbrev-commit --date=short"; \
    git config --global push.default current
