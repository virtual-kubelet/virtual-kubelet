#!/bin/bash

if [ "${REGION}" == "" ];then
    REGION="us-west-1"
fi

if [ "$@" != "./build.sh" ];then
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

    #show example
    cat <<EOF

Run in container(example):
  ./build.sh                                # build hyper cli
  -----------------------------------------------------------
  hypercli info | grep "ID"                 # get tennat id
  hypercli pull busybox                     # pull image
  hypercli images                           # list images
  -----------------------------------------------------------
  cd integration-cli && go test             # start autotest

# 'hypercli' is the alias of 'hyper --region \${DOCKER_HOST}'

EOF
fi

#execute command
if [ $# -ne 0 ];then
    eval $@
    if [ "$@" == "./build.sh" ];then
    #show make result
        if [ $? -eq 0 ];then
            echo "OK:)"
        else
            echo "Failed:("
        fi
    fi
fi
