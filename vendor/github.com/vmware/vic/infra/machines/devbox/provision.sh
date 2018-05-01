#!/bin/bash -e
# Copyright 2016 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# add key for docker repo
apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
# add docker apt sources
echo "deb https://apt.dockerproject.org/repo ubuntu-xenial main" > /etc/apt/sources.list.d/docker.list

# https://github.com/mitchellh/vagrant/issues/289
apt-get update && sudo DEBIAN_FRONTEND=noninteractive apt-get -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" dist-upgrade

# set GOPATH based on shared folder of vagrant
pro="/home/${BASH_ARGV[0]}/.profile"
echo "export GOPATH=${BASH_ARGV[1]}" >> "$pro"

# add GOPATH/bin to the PATH
echo "export PATH=$PATH:${BASH_ARGV[1]}/bin" >> "$pro"

apt-get -y install curl lsof strace git shellcheck tree mc silversearcher-ag jq htpdate apt-transport-https ca-certificates nfs-common sshpass

function update_go {
    (cd /usr/local &&
            (curl --silent -L $go_file | tar -zxf -) &&
            ln -fs /usr/local/go/bin/* /usr/local/bin/)
}

# install / upgrade go
go_file="https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz"
go_version=$(basename $go_file | cut -d. -f1-3)

if [[ ! -d "/usr/local/go" || $(go version | awk '{print $(3)}') != "$go_version" ]] ; then
    update_go
fi

# Install docker
apt-get -y install linux-image-extra-$(uname -r)
apt-get -y --allow-downgrades install docker-engine=1.13.1-0~ubuntu-xenial
apt-mark hold docker-engine
usermod -aG docker vagrant
systemctl start docker
