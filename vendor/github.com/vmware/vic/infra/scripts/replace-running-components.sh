#!/bin/bash
# Copyright 2018 VMware, Inc. All Rights Reserved.
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

set -eo pipefail

if [ -z "$PS1" ]; then
    interactive=1
else
    interactive=0
fi

[ -n "$DEBUG" ] && set -x
BASE_DIR=$(dirname $(readlink -f "$BASH_SOURCE"))
VIC_DIR=$(dirname $(readlink -f $BASE_DIR/..))

# Run the command given on the VCH instead of locally
function on-vch() {
    ssh -oUserKnownHostsFile=/dev/null -oStrictHostKeyChecking=no -i"${VIC_KEY%.*}" root@$VCH_IP -C $@ 2>/dev/null
}

function get-thumbprint() {
    govc about.cert -k -thumbprint | awk '{print $NF}'
}

# Determines whether the human VCH name is adequate to continue
function vch-name-is-ambiguous() {
    [ $($VIC_DIR/bin/vic-machine-linux \
            ls \
            --target=$target/${GOVC_DATACENTER} \
            --user=$username \
            --password=$password \
            --thumbprint=$(get-thumbprint) \
            | grep $VIC_NAME | wc -l) -ne 1 ] \
        && return 0 || return 1
}

# Check GOVC vars & print help text if not
function check-govc-vars() {
    if [[ ! $(govc ls 2>/dev/null) ]]; then
        echo "ERROR:"
        echo "GOVC environment variables are required to use this command. Set the necessary variables to allow govc to connect to your vSphere deployment:";
        echo "GOVC_USERNAME: username on vSphere target"
        echo "GOVC_PASSWORD: password on vSphere target"
        echo "GOVC_URL: IP or FQDN of your vSphere target"
        echo "GOVC_INSECURE: set to 1 to disable tls verify when using govc to talk to vSphere"
        exit 1
    fi

}

# Make sure we have at least one of VIC_NAME or VIC_ID
function check-vch-name-or-id () {
    if [[ ! -v VIC_NAME ]] && [[ ! -v VIC_ID ]]; then
        echo "Please set one of the following environment variables to specify the VCH which you would like to reconfigure:"
        echo "VIC_NAME: name of VCH; matches --name argument for vic-machine"
        echo "VIC_ID: ID of VCH, as displayed in output of vic-machine ls"
        if [[ $interactive -eq 0 ]]; then
            exit 1
        fi
        read -p "Or enter VCH name to continue: " VIC_NAME
    fi
}

# Falls back on VIC_ID if possible, bails if not, assumes the VCH name is ambiguous
function check-name-isnt-ambiguous ()  {
    username=$(govc env | grep GOVC_USERNAME | cut -d= -f2)
    password=$(govc env | grep GOVC_PASSWORD | cut -d= -f2)
    target=$(govc env | grep GOVC_URL | cut -d= -f2)

    if [[ ! -v VIC_ID ]] && [[ $(vch-name-is-ambiguous) ]]; then
        echo "The provided VIC name is ambiguous; please choose the correct VCH ID from the output below and assign it to the environment variable VIC_ID, e.g., export VIC_ID=12"

        $VIC_DIR/bin/vic-machine-linux\
            ls \
            --target $target/${GOVC_DATACENTER} \
            --user "$username" \
            --password="$password" \
            --thumbprint=$(get-thumbprint)

        if [[ $interactive -eq 0 ]]; then
            exit 1
        fi
        read -p "Enter unique VIC ID from above: " VIC_ID
    fi
}

# Translates VCH name to ID if necessary
function get-vic-id () {
    if [[ -z $VIC_ID ]]; then
        export VIC_ID="$($VIC_DIR/bin/vic-machine-linux ls --target=$target/${GOVC_DATACENTER} --user="$username" --password="$password" --thumbprint=$(get-thumbprint) | grep $VIC_NAME | awk '{print $1}')"
    fi
}

function get-ssh-keys() {
    if [[ -z $VIC_KEY ]]; then
        key="/home/$USER/.ssh/id_rsa.pub"
        if [ -r "${key}" -a -r "${key%.*}" ]; then
            echo "Using default key $key - use VIC_KEY to override"
            export VIC_KEY=${key:-/home/$USER/.ssh/id_rsa.pub}
            return
        fi

        echo "Variable VIC_KEY not set. Provide the path to your public SSH key below."
        if [[ $interactive -eq 0 ]]; then
            exit 1
        fi
        read -p "Path to your public SSH key for access to VCH [/home/$USER/.ssh/id_rsa.pub]: " key
        export VIC_KEY=${key:-/home/$USER/.ssh/id_rsa.pub}
    fi
}

# Checks environment for required inputs
function sanity-checks () {
    check-govc-vars
    check-vch-name-or-id
    check-name-isnt-ambiguous
    get-vic-id
    get-ssh-keys
}

# Enables SSH and saves off the VCH IP address
function enable-debug () {
    VCH_IP=$($VIC_DIR/bin/vic-machine-linux debug \
                                   --target=$target/${GOVC_DATACENTER} \
                                   --id=$VIC_ID \
                                   --user="$username" \
                                   --password="$password" \
                                   --authorized-key="$VIC_KEY" \
                                   --thumbprint=$(get-thumbprint) \
                 | grep -A1 "Published ports" | tail -n1 | awk '{print $NF}')
}

# SCPs the component in $1 to the VCH, plops it in place, and brutally kills the previous running process
function replace-component() {
    scp -oUserKnownHostsFile=/dev/null -oStrictHostKeyChecking=no -i"${VIC_KEY%.*}" $VIC_DIR/bin/$1 root@$VCH_IP:/tmp/$1 2>/dev/null
    pid=$(on-vch ps -e --format='pid,args' \
                 | grep $1 | grep -v grep | awk '{print $1}')
    on-vch chmod 755 /tmp/$1
    on-vch mv /tmp/$1 /sbin/$1
    if [[ $1 == "vic-init" ]]; then
        on-vch systemctl restart vic-init
    else
        on-vch kill -9 $pid
    fi
}

function replace-components () {
    if [[ $1 == "" ]]; then # replace everything
        services="port-layer-server docker-engine-server vicadmin vic-init"
    else
        services="$@"
    fi

    for x in $services; do
        echo "Replacing component $x..."
        replace-component $x
    done
}

sanity-checks
enable-debug
replace-components $@

echo "If you ran make push with no arguments or replaced vic-init, wait a few moments and run this to get the new IP of your VCH:"
echo "$VIC_DIR/bin/vic-machine-linux inspect --target=$target --id=$VIC_ID --user=$username --password=$password --thumbprint=$(get-thumbprint)"
