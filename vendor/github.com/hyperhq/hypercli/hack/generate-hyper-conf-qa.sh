#!/bin/bash

env

if [ "${REGION}" == "" ];then
    REGION="us-west-1"
fi

# set default value of DOCKER_HOST and BRANCH
if [[ "$DOCKER_HOST" == "" ]];then
  DOCKER_HOST="tcp://us-west-1.hyper.sh:443"
fi

PR=""
if [[ "${BRANCH:0:1}" == "#" ]];then
  PR=${BRANCH:1}
  BRANCH=""
  echo "========== Task: test PR #${PR} =========="
else
  if [[ "${BRANCH}" == "" ]];then
    BRANCH="master"
  fi
  echo "========== Task: test BRANCH ${BRANCH} =========="
fi

if [[ "${ACCESS_KEY}" == "" ]] || [[ "${SECRET_KEY}" == "" ]];then
  echo "Error: Please set ACCESS_KEY and SECRET_KEY"
  exit 1
fi

if [[ "$@" != "./build.sh" ]];then
    #ensure config for hyper cli
    mkdir -p ~/.hyper
    cat > ~/.hyper/config.json <<EOF
{
    "clouds": {
        "${DOCKER_HOST}": {
            "accesskey": "${ACCESS_KEY}",
            "secretkey": "${SECRET_KEY}"
        },
        "tcp://*.hyper.sh:443": {
            "accesskey": "${ACCESS_KEY}",
            "secretkey": "${SECRET_KEY}",
            "region": "${REGION}"
        }
    }
}
EOF

echo "========== config git proxy =========="
if [ "${http_proxy}" != "" ];then
  git config --global http.proxy ${http_proxy}
fi
if [ "${https_proxy}" != "" ];then
  git config --global https.proxy ${https_proxy}
fi
git config --list | grep proxy

echo "========== ping github.com =========="
ping -c 6 -W 10 github.com

echo "========== Clone hypercli repo =========="
mkdir -p /go/src/github.com/{hyperhq,docker}
cd /go/src/github.com/hyperhq
git clone https://github.com/hyperhq/hypercli.git

echo "========== Build hypercli =========="
cd /go/src/github.com/hyperhq/hypercli
if [[ "${BRANCH}" != "" ]];then
  echo "checkout branch :${BRANCH}"
  git checkout ${BRANCH}
elif [[ "${PR}" != "" ]];then
  echo "checkout pr :#$PR"
  git fetch origin pull/${PR}/head:pr-${PR}
  git checkout pr-${PR}
fi

if [[ $? -ne 0 ]];then
  echo "Branch ${BRANCH} not exist!"
  exit 1
fi
./build.sh
ln -s /go/src/github.com/hyperhq/hypercli /go/src/github.com/docker/docker
ln -s /go/src/github.com/hyperhq/hypercli/hyper/hyper /usr/bin/hyper
echo alias hypercli=\"hyper --region \${DOCKER_HOST}\" >> /root/.bashrc
source /root/.bashrc

echo "##############################################################################################"
echo "##                               Welcome to integration test env                            ##"
echo "##############################################################################################"
#show config for hyper cli
echo "Current hyper config: ~/.hyper/config.json"
echo "----------------------------------------------------------------------------------------------"
cat ~/.hyper/config.json \
  | sed 's/"secretkey":.*/"secretkey": "******************************",/g' \
  | sed 's/"auth":.*/"auth": "******************************"/g'
echo "----------------------------------------------------------------------------------------------"

fi

#execute command
if [[ $# -ne 0 ]];then
    echo "========== Test Cmd: $@ =========="
    cd /go/src/github.com/hyperhq/hypercli/integration-cli && $@
    if [[ "$@" == "./build.sh" ]];then
    #show make result
        if [[ $? -eq 0 ]];then
            echo "OK:)"
        else
            echo "Failed:("
        fi
    fi
fi
