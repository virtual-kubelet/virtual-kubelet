#!/bin/bash
# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
# limitations under the License.!/bin/bash

OS=$(uname | tr '[:upper:]' '[:lower:]')

tls () {
    unset TLS_OPTS
}

no-tls () {
    export TLS_OPTS="--no-tls"
}

unset-vic () {
    unset MAPPED_NETWORKS NETWORKS IMAGE_STORE DATASTORE COMPUTE VOLUME_STORES IPADDR GOVC_INSECURE TLS THUMBPRINT OPS_CREDS VIC_NAME
}

vic-path () {
    echo "${GOPATH}/src/github.com/vmware/vic"
}

vic-create () {
    base=$(pwd)
    (
        cd "$(vic-path)"/bin || return
        "$(vic-path)"/bin/vic-machine-"$OS" create --target="$GOVC_URL" "${OPS_CREDS[@]}" --image-store="$IMAGE_STORE" --compute-resource="$COMPUTE" "${TLS[@]}" ${TLS_OPTS} --name="${VIC_NAME:-${USER}test}" "${MAPPED_NETWORKS[@]}" "${VOLUME_STORES[@]}" "${NETWORKS[@]}" ${IPADDR} ${TIMEOUT} --thumbprint="$THUMBPRINT" "$@"
    )

    unset DOCKER_CERT_PATH DOCKER_TLS_VERIFY
    unalias docker 2>/dev/null

    envfile=$(vic-path)/bin/${VIC_NAME:-${USER}test}/${VIC_NAME:-${USER}test}.env
    if [ -f "$envfile" ]; then
        set -a
        source "$envfile"
        set +a
    fi

    # Something of a hack, but works for --no-tls so long as that's enabled via TLS_OPTS
    if [ -z "${DOCKER_TLS_VERIFY+x}" ] && [ -z "${TLS_OPTS+x}" ]; then
        alias docker='docker --tls'
    fi

    cd "$base" || exit
}

vic-delete () {
    "$(vic-path)"/bin/vic-machine-"$OS" delete --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --thumbprint="$THUMBPRINT" --force "$@"
}

vic-inspect () {
    "$(vic-path)"/bin/vic-machine-"$OS" inspect --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --thumbprint="$THUMBPRINT" "$@"
}

vic-upgrade () {
    "$(vic-path)"/bin/vic-machine-"$OS" upgrade --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --thumbprint="$THUMBPRINT" "$@"
}

vic-ls () {
    "$(vic-path)"/bin/vic-machine-"$OS" ls --target="$GOVC_URL" --thumbprint="$THUMBPRINT" "$@"
}

vic-ssh () {
    unset keyarg
    if [ -e "$HOME"/.ssh/authorized_keys ]; then
        keyarg="--authorized-key=$HOME/.ssh/authorized_keys"
    fi

    out=$("$(vic-path)"/bin/vic-machine-"$OS" debug --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --enable-ssh "$keyarg" --rootpw=password --thumbprint="$THUMBPRINT" "$@")
    host=$(echo "$out" | grep DOCKER_HOST | awk -F"DOCKER_HOST=" '{print $2}' | cut -d ":" -f1 | cut -d "=" -f2)

    echo "SSH to ${host}"
    sshpass -ppassword ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no root@"${host}"
}

vic-admin () {
    out=$("$(vic-path)"/bin/vic-machine-"$OS" debug --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --enable-ssh "$keyarg" --rootpw=password --thumbprint="$THUMBPRINT" "$@")
    host=$(echo "$out" | grep DOCKER_HOST | sed -n 's/.*DOCKER_HOST=\([^:\s*\).*/\1/p')

    open http://"${host}":2378
}

addr-from-dockerhost () {
    echo "$DOCKER_HOST" | sed -e 's/:[0-9]*$//'
}

vic-tail-portlayer() {
    unset keyarg
    if [ -e "$HOME"/.ssh/authorized_keys ]; then
        keyarg="--authorized-key=$HOME/.ssh/authorized_keys"
    fi

    out=$("$(vic-path)"/bin/vic-machine-"$OS" debug --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --enable-ssh "$keyarg" --rootpw=password --thumbprint="$THUMBPRINT" "$@")
    host=$(echo "$out" | grep DOCKER_HOST | awk -F"DOCKER_HOST=" '{print $2}' | cut -d ":" -f1 | cut -d "=" -f2)

    sshpass -ppassword ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no root@"${host}" tail -f /var/log/vic/port-layer.log
}

vic-tail-docker() {
    unset keyarg
    if [ -e "$HOME"/.ssh/authorized_keys ]; then
        keyarg="--authorized-key=$HOME/.ssh/authorized_keys"
    fi

    out=$("$(vic-path)"/bin/vic-machine-"$OS" debug --target="$GOVC_URL" --compute-resource="$COMPUTE" --name="${VIC_NAME:-${USER}test}" --enable-ssh "$keyarg" --rootpw=password --thumbprint="$THUMBPRINT" "$@")
    host=$(echo "$out" | grep DOCKER_HOST | awk -F"DOCKER_HOST=" '{print $2}' | cut -d ":" -f1 | cut -d "=" -f2)

    sshpass -ppassword ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no root@"${host}" tail -f /var/log/vic/docker-personality.log
}

# import the custom sites
# example entry, actived by typing "example"
# those variales that hold multiple arguments which may contain spaces are arrays to allow for proper quoting
#example () {
#    target='https://user:password@host.domain.com/datacenter'
#    unset-vic
#
#    export GOVC_URL=$target
#
#    eval "export THUMBPRINT=$(govc about.cert -k -json | jq -r .ThumbprintSHA1)"
#    export COMPUTE=cluster/pool
#    export DATASTORE=datastore1
#    export IMAGE_STORE=$DATASTORE/image/path
#    export TIMEOUT="--timeout=10m"
#    export IPADDR="--client-network-ip=vch-hostname.domain.com --client-network-gateway=x.x.x.x/22 --dns-server=y.y.y.y --dns-server=z.z.z.z"
#    export VIC_NAME="MyVCH"
#
#    TLS=("--tls-cname=vch-hostname.domain.com" "--organization=MyCompany")
#    OPS_CREDS=("--ops-user=<user>" "--ops-password=<password>")
#    NETWORKS=("--bridge-network=private-dpg-vlan" "--public-network=extern-dpg")
#    MAPPED_NETWORKS=("--container-network=VM Network:external" "--container-network=SomeOtherNet:elsewhere")
#    VOLUME_STORES=("--volume-store=$DATASTORE:default")
#
#    export NETWORKS MAPPED_NETWORKS VOLUME_STORES OPS_CREDS TLS
#}

. ~/.vic
