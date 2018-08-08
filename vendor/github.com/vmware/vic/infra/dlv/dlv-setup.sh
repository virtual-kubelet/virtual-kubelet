#!/bin/bash
# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#50    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.!/bin/bash

SSH="ssh -o StrictHostKeyChecking=no"
SCP="scp -o StrictHostKeyChecking=no"

function usage() {
    echo "Usage: $0 -h vch-address -p password -s -a" >&2
    exit 1
}

while getopts "h:p:sa" flag
do
    case $flag in

        h)
            # Optional
            export DLV_TARGET_HOST="$OPTARG"
            ;;

        p)
            export SSHPASS="$OPTARG"
            ;;

        s)
            COPY_SSH_KEY=true
            ;;
        a)
            SSH_AUTHORIZED=true
            ;;
        *)
            usage
            ;;
    esac
done

shift $((OPTIND-1))

if [ -z "${DLV_TARGET_HOST}" ]; then
    usage
fi

if [ -z "${SSHPASS}" -a -z "${SSH_AUTHORIZED}" ]; then
    usage
fi

if [ -n "${SSHPASS}" -a -z "${SSH_AUTHORIZED}" ]; then
    SSH="sshpass -e ${SSH}"
    SCP="sshpass -e ${SCP}"

    if [ ! -f /usr/bin/sshpass ]; then
        echo sshpass must be installed. Run \"apt-get install sshpass\"
        exit 1
    fi
fi

DLV_BIN="$GOPATH/bin/dlv"

if [ -z "${GOPATH}" -o -z "${GOROOT}" ]; then
    echo GOROOT and GOPATH should be set to point to the current GOLANG enironment
    exit 1
fi

# copy dlv binary
echo -n copying dlv binary..
if [ -f ${DLV_BIN} ]; then
    ${SCP} ${DLV_BIN} root@${DLV_TARGET_HOST}:/usr/local/bin
else
    echo $DLV_BIN does not exist. Run \"go get github.com/derekparker/delve/cmd/dlv\"
    exit 1
fi
echo done

# copy GOROOT env
echo -n copying GOROOT environment..
${SSH} root@${DLV_TARGET_HOST} "mkdir -p /usr/local/go"
${SCP} -r ${GOROOT}/bin root@${DLV_TARGET_HOST}:/usr/local/go
${SCP} -r ${GOROOT}/api root@${DLV_TARGET_HOST}:/usr/local/go
${SCP} ${GOROOT}/VERSION root@${DLV_TARGET_HOST}:/usr/local/go
${SSH} root@${DLV_TARGET_HOST} "ln -f -s /usr/local/go/bin/go /usr/local/bin/go"
echo done

# open IPTABLES
echo -n fixing ipatables..
${SSH} root@${DLV_TARGET_HOST} "iptables -I INPUT -p tcp -m tcp --dport 2345:2350 -j ACCEPT"
echo done
echo "Iptables changed: run \"iptables -D INPUT 1\" when finished debugging"

# write remote dlv attach script
TEMPFILE=$(mktemp)
cat > ${TEMPFILE} <<EOF
#/bin/bash
if [ \$# != 2 ]; then
    echo "\$0 vic-init|vic-admin|docker-engine|port-layer|vic-machine|virtual-kubelet port"
    exit 1
fi

NAME=\$1
PORT=\$2

if [ -z "\${NAME}" -o -z "\${PORT}" ]; then
    echo "\$0 vic-init|vic-admin|docker-engine|port-layer|vic-machine|virtual-kubelet port"
    exit 1
fi

PID=\$(ps -e | grep \${NAME} | grep -v grep | tr -s ' ' | cut -d " " -f 2)

if [ -z "\${PID}" ]; then
    echo "\$0: cannot find process \${NAME}"
    exit 1
fi

dlv attach \${PID} --api-version 2 --headless --listen=:\${PORT} 
EOF

${SCP} ${TEMPFILE} root@${DLV_TARGET_HOST}:/usr/local/bin/dlv-attach-headless.sh

# write dlv detach script
cat > ${TEMPFILE} <<EOF
#/bin/bash
if [ \$# != 1 ]; then
    echo "\$0 port-number"
    exit 1
fi

PORT=\$1

if [ -z "\${PORT}" ]; then
    echo "\$0 port-number"
    exit 1
fi

# Find appropriate dlv instance
PID=\$(ps -ef | grep "dlv" |  grep "api-version" | grep \${PORT} | grep -v grep |  tr -s ' ' | cut -d " " -f 2)

if [ -z "\${PID}" ]; then
    echo "\$0: cannot find dlv listening on \${PORT}"
    exit 1
fi

kill \${PID}
EOF

${SCP} ${TEMPFILE} root@${DLV_TARGET_HOST}:/usr/local/bin/dlv-detach-headless.sh

${SSH} root@${DLV_TARGET_HOST} 'chmod +x /usr/local/bin/*'

${SSH} root@${DLV_TARGET_HOST} 'passwd -x 1000000 root'

rm ${TEMPFILE}

if [ -n "${COPY_SSH_KEY}" -a -n "${SSHPASS}" ]; then
    # setup authorized_keys
    ${SSH} root@${DLV_TARGET_HOST} "mkdir -p .ssh"
    ${SCP} ${HOME}/.ssh/*.pub root@${DLV_TARGET_HOST}:.ssh
    ${SSH} root@${DLV_TARGET_HOST} 'cat ~/.ssh/*.pub > ~/.ssh/authorized_keys'
    ${SSH} root@${DLV_TARGET_HOST} 'rm ~/.ssh/*.pub'
    ${SSH} root@${DLV_TARGET_HOST} 'chmod 600 ~/.ssh/authorized_keys'
fi
