#!/bin/bash
# tool for autotest
# please run this scrip in host os

#############################################################################
function show_usage() {
    cat <<EOF
Usage: ./util.sh <action>
<action>:
    build-dev          # build docker image 'hyperhq/hypercl' from Dockerfile.dev
    build-qa           # build docker image 'hyperhq/hypercl' from Dockerfile.qa
    make               # make hyper cli in container
    enter-dev          # enter container from hyperhq/hhypercli-auto-test:dev
    enter-qa <branch>  # enter container from hyperhq/hhypercli-auto-test:qa, default branch is 'master'
    test               # test on host
EOF
}

function show_test_usage() {
    cat <<EOF
---------------------------------------------------------------------------------------------------------------
Usage:
  ./util.sh test all                                     # run all test case
  ./util.sh test all -timeout 20m                        # run all test case with specified timeout(default 10m)
  ./util.sh test -check.f <case prefix>                  # run specified test case
  ./util.sh test -check.f ^<case name>$                  # run specified prefix of test case
  ./util.sh test -check.f <case prefix> -timeout 20m     # combined use
----------------------------------------------------------------------------------------------------------------
EOF
}

#############################################################################
IMAGE_NAME="hyperhq/hypercli-auto-test"
WORKDIR=$(cd `dirname $0`; pwd)
cd ${WORKDIR}

#############################################################################
# ensure util.conf
if [ ! -s ${WORKDIR}/util.conf ];then
  cp util.conf.template util.conf
fi


# load util.conf
source ${WORKDIR}/util.conf

# check util.conf
if [[ "${ACCESS_KEY}" == "" ]] || [[ "${SECRET_KEY}" == "" ]];then
    echo "please update 'ACCESS_KEY' and 'SECRET_KEY' in '${WORKDIR}/util.conf'"
    exit 1
fi

#############################################################################
# main
#############################################################################
case $1 in
  "build-dev")
    cd ${WORKDIR}/..
    docker build -t ${IMAGE_NAME}:dev -f Dockerfile.dev .
    ;;
  "build-qa")
    cd ${WORKDIR}/..
    docker build -t ${IMAGE_NAME}:qa -f Dockerfile.qa .
    ;;
  make)
    echo "Start compile hyper client, please wait..."
    docker run -it --rm \
        -v $(pwd)/../:/go/src/github.com/hyperhq/hypercli \
        ${IMAGE_NAME}:dev ./build.sh
    ;;
  enter-dev)
    docker run -it --rm \
        -e DOCKER_HOST=${HYPER_HOST} \
        -e REGION=${REGION} \
        -e ACCESS_KEY=${ACCESS_KEY} \
        -e SECRET_KEY=${SECRET_KEY} \
        -e AWS_ACCESS_KEY=${AWS_ACCESS_KEY} \
        -e AWS_SECRET_KEY=${AWS_SECRET_KEY} \
        -e URL_WITH_BASIC_AUTH=${URL_WITH_BASIC_AUTH} \
        -e MONGODB_URL=${MONGODB_URL} \
        -e DOCKERHUB_EMAIL=${DOCKERHUB_EMAIL} \
        -e DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME} \
        -e DOCKERHUB_PASSWD=${DOCKERHUB_PASSWD} \
        -v $(pwd)/../:/go/src/github.com/hyperhq/hypercli \
        ${IMAGE_NAME}:dev zsh
    ;;
  enter-qa)
    BRANCH=$2
    if [ "$BRANCH" == "" ];then
      BRANCH="master"
    fi
    docker run -it --rm \
        -e http_proxy=${http_proxy} \
        -e https_proxy=${https_proxy} \
        -e DOCKER_HOST=${HYPER_HOST} \
        -e REGION=${REGION} \
        -e BRANCH=${BRANCH} \
        -e ACCESS_KEY=${ACCESS_KEY} \
        -e SECRET_KEY=${SECRET_KEY} \
        -e DOCKERHUB_EMAIL=${DOCKERHUB_EMAIL} \
        -e DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME} \
        -e DOCKERHUB_PASSWD=${DOCKERHUB_PASSWD} \
        ${IMAGE_NAME}:qa /bin/bash
#        ${IMAGE_NAME}:qa go test -check.f TestCli -timeout 180m
    ;;
  test)
    export DOCKER_HOST=${HYPER_HOST}
    export GOPATH=$GOPATH:`pwd`/../vendor
    mkdir -p ${IMAGE_DIR}
    shift
    if [ $# -ne 0 ];then
      if [ $1 == "all" ];then
        shift
        go test $@
      elif [ $1 == "-check.f" ];then
        go test $@
      else
        show_test_usage
      fi
    else
      show_test_usage
    fi
    ;;
  *) show_usage
    ;;
esac
