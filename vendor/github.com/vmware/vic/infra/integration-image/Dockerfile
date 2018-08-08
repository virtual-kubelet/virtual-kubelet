# Building:
# cp -R /usr/lib/vmware-ovftool/ .
# docker build --no-cache -t vic-test -f Dockerfile .
# docker tag vic-test gcr.io/eminent-nation-87317/vic-integration-test:1.x
# gcloud auth login
# gcloud docker -- push gcr.io/eminent-nation-87317/vic-integration-test:1.x
# download and install harbor certs, then docker login, then:
# docker tag vic-test wdc-harbor-ci.eng.vmware.com/default-project/vic-integration-test:1.x
# docker push wdc-harbor-ci.eng.vmware.com/default-project/vic-integration-test:1.x

FROM golang:1.8

RUN wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
RUN sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list'

RUN curl -sL https://deb.nodesource.com/setup_7.x | bash - && \
    apt-get update && apt-get install -y --no-install-recommends \
    jq \
    bc \
    time \
    gcc \
    python-dev \
    libffi-dev \
    libssl-dev \
    sshpass \
    ant \
	ant-optional \
    openjdk-7-jdk \
    rpcbind \
    nfs-common \
    unzip \
    zip \
    bzip2 \
	nodejs \
	parted \
    # Add docker in docker support
    btrfs-tools \
    e2fsprogs \
    iptables \
    xfsprogs \
    dnsutils \
    netcat \
    # Add headless chrome support
    google-chrome-stable \
    # Speed up ISO builds with already installed reqs
    yum \
    yum-utils \
    cpio \
    rpm \
    ca-certificates \
    xz-utils \
    xorriso \
    sendmail && \
	# Cleanup
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*


RUN wget https://bootstrap.pypa.io/get-pip.py && \
    python ./get-pip.py  && \
    pip install pyasn1 google-apitools==0.5.15 gsutil==4.28 robotframework robotframework-sshlibrary robotframework-httplibrary requests dbbot robotframework-selenium2library robotframework-pabot codecov --upgrade

# Install docker, docker clients 1.11,1.12 and 1.13
# Also install docker compose 1.13
RUN curl -sSL https://get.docker.com/ | sh && \
    curl -fsSLO https://get.docker.com/builds/Linux/x86_64/docker-1.11.2.tgz && \
    tar --strip-components=1 -xvzf docker-1.11.2.tgz -C /usr/bin &&  \
    mv /usr/bin/docker /usr/bin/docker1.11 && \
    curl -fsSLO https://get.docker.com/builds/Linux/x86_64/docker-1.12.6.tgz && \
    tar --strip-components=1 -xvzf docker-1.12.6.tgz -C /usr/bin  && \
    mv /usr/bin/docker /usr/bin/docker1.12 && \
    curl -fsSLO https://get.docker.com/builds/Linux/x86_64/docker-1.13.0.tgz && \
    tar --strip-components=1 -xvzf docker-1.13.0.tgz -C /usr/bin && \
    mv /usr/bin/docker /usr/bin/docker1.13 && \
    ln -s /usr/bin/docker1.13 /usr/bin/docker && \
    curl -L https://github.com/docker/compose/releases/download/1.11.2/docker-compose-`uname -s`-`uname -m` > /usr/local/bin/docker-compose && \
    chmod +x /usr/local/bin/docker-compose

COPY vmware-ovftool /usr/lib/vmware-ovftool
RUN ln -s /usr/lib/vmware-ovftool/ovftool /usr/local/bin/ovftool

RUN curl -fsSLO https://releases.hashicorp.com/packer/0.12.2/packer_0.12.2_linux_amd64.zip && \
    unzip packer_0.12.2_linux_amd64.zip -d /usr/bin && \
	rm packer_0.12.2_linux_amd64.zip

RUN wget https://github.com/drone/drone-cli/releases/download/v0.8.3/drone_linux_amd64.tar.gz && tar zxf drone_linux_amd64.tar.gz && \
    install -t /usr/local/bin drone

RUN curl -sSL https://github.com/vmware/govmomi/releases/download/v0.17.1/govc_linux_amd64.gz | gzip -d > /usr/local/bin/govc && chmod +x /usr/local/bin/govc

RUN  wget https://launchpad.net/ubuntu/+source/wget/1.18-2ubuntu1/+build/10470166/+files/wget_1.18-2ubuntu1_amd64.deb && \
     dpkg -i wget_1.18-2ubuntu1_amd64.deb

# Add docker in docker support
# version: docker:1.13-dind
# reference: https://github.com/docker-library/docker/blob/b202ec7e529f5426e2ad7e8c0a8b82cacd406573/1.13/dind/Dockerfile
#
# https://github.com/docker/docker/blob/master/project/PACKAGERS.md#runtime-dependencies

# set up subuid/subgid so that "--userns-remap=default" works out-of-the-box
RUN set -x \
        && groupadd --system dockremap \
        && adduser --system --ingroup dockremap dockremap \
        && echo 'dockremap:165536:65536' >> /etc/subuid \
        && echo 'dockremap:165536:65536' >> /etc/subgid

ENV DIND_COMMIT 3b5fac462d21ca164b3778647420016315289034

RUN wget "https://raw.githubusercontent.com/docker/docker/${DIND_COMMIT}/hack/dind" -O /usr/local/bin/dind \
        && chmod +x /usr/local/bin/dind

# This container needs to be run in privileged mode(run with --privileged option) to make it work
COPY dockerd-entrypoint.sh /usr/local/bin/dockerd-entrypoint.sh
RUN chmod +x /usr/local/bin/dockerd-entrypoint.sh

COPY scripts /opt/vmware/scripts
ENV PATH="${PATH}:/opt/vmware/scripts"

VOLUME /var/lib/docker
