#!/bin/bash
# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

SSH="ssh -o StrictHostKeyChecking=no"
SCP="scp -o StrictHostKeyChecking=no"

REMOTE_DLV_ATTACH=/usr/local/bin/dlv-attach-headless.sh
REMOTE_DLV_DETACH=/usr/local/bin/dlv-detach-headless.sh

function usage() {
    echo "Usage: $0 -h vch-address [-a/-d] -p password [attach/detach] target" >&2
    echo "Valid targets are: "
    echo "    vic-init"
    echo "    vic-admin"
    echo "    docker-engine"
    echo "    port-layer"
    echo "    vic-machine"
    echo "    virtual-kubelet"
    exit 1
}

while getopts "h:p:ad" flag
do
    case $flag in

        h)
            # Optional
            export DLV_TARGET_HOST="$OPTARG"
            ;;

        p)
            export SSHPASS="$OPTARG"
            ;;

        a)
            export COMMAND="attach"
            ;;

        d)
            export COMMAND="detach"
            ;;

        *)
            usage
            ;;
    esac
done

shift $((OPTIND-1))

if [[ -z "${COMMAND}" &&  $# != 2 ]]; then
    usage
elif [[ -n "${COMMAND}" && $# != 1 ]]; then
    usage
fi

if [ -z "${COMMAND}" ]; then
    COMMAND=$1
    TARGET=$2
else
    TARGET=$1
fi

case ${TARGET} in

    vic-init)
        PORT=2345
        ;;

    vic-admin)
        PORT=2346
        ;;

    docker-engine)
        PORT=2347
        ;;

    port-layer)
        PORT=2348
        ;;

    vic-machine)
        PORT=2349
        ;;

    virtual-kubelet)
        PORT=2350
        ;;

    *)
        usage
        ;;
esac

if [ -z "${DLV_TARGET_HOST}" ]; then
    usage
fi

if [ -n "${SSHPASS}" ]; then
    SSH="sshpass -e ${SSH}"
    SCP="sshpass -e ${SCP}"

    if [ ! -f /usr/bin/sshpass ]; then
        echo sshpass must be installed. Run \"apt-get install sshpass\"
        exit 1
    fi
fi

if [ ${COMMAND} == "attach" ]; then
    ${SSH} root@${DLV_TARGET_HOST} "nohup /usr/local/bin/dlv-attach-headless.sh $TARGET $PORT >  /var/tmp/${TARGET}.log 2>&1 &"
elif [ ${COMMAND} == "detach" ]; then
    ${SSH} root@${DLV_TARGET_HOST} "/usr/local/bin/dlv-detach-headless.sh $PORT"
else
    usage
fi

echo $DLV_TARGET_HOST:$PORT
