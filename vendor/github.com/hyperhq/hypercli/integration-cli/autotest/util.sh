#!/bin/bash

set -e

if [ ! -f etc/config ];then
  cp etc/config.template etc/config
  echo "please config etc/config first"
  exit 1
fi

. etc/config

repo="hyperhq/hypercli-auto-test"
tag="auto"
image=${repo}:${tag}


function build(){
    echo "starting build..."
    echo "=============================================================="
    docker build --build-arg http_proxy=${http_proxy} --build-arg https_proxy=${https_proxy} -t ${image} .
}

function push(){

    echo -e "\nstarting push [${image}] ..."
    echo "=============================================================="
    docker push ${image}
}

function run(){
    CONFIG_FILE=$1
    if [ "${CONFIG_FILE}" != "" ];then
        if [ ! -f "${CONFIG_FILE}" ];then
            echo "${CONFIG_FILE} not found"
            exit 1
        else
            echo "use config: ${CONFIG_FILE}"
            . ${CONFIG_FILE}
        fi
    else
        echo "use default config: etc/config"
    fi
    docker run -it --rm \
	-e http_proxy="$http_proxy" \
	-e https_proxy="$https_proxy" \
	-e TARGET_REGION="$TARGET_REGION" \
	-e BRANCH="$BRANCH" \
	-e TEST_CASE_REG="$TEST_CASE_REG" \
	-e ACCESS_KEY="${ACCESS_KEY}" \
	-e SECRET_KEY="${SECRET_KEY}" \
	-e SLACK_TOKEN="${SLACK_TOKEN}" \
	-e SLACK_CHANNEL_ID="${SLACK_CHANNEL_ID}" \
	${image}
}

case "$1" in
    "push")
        build
        push
        ;;
    "build")
        build
        ;;
    "run")
	    run "$2"
	;;
    *)
        cat <<EOF
usage:
    ./util.sh               # show usage
    ./util.sh build         # build only
    ./util.sh push          # build and push
    ./util.sh run <config>  # run docker container

example:
    ./util.sh run
    ./util.sh run etc/config
    ./util.sh run etc/config.zl2
    ./util.sh run etc/config.eu1
    ./util.sh run etc/config.pkt
EOF
    exit 1
        ;;
esac



echo -e "\n=============================================================="
echo "Done!"

