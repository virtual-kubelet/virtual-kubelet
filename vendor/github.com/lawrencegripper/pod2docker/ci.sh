#!/bin/bash
set -e
trap cleanup EXIT
function cleanup(){
    docker rm -f pod2dockerci
}

docker build -t ci -f ci.Dockerfile . 
docker run --name pod2dockerci -e HOSTDIR=$PWD -v $PWD:$PWD -v /var/lib/docker/containers:/var/lib/docker/containers -v /var/run/docker.sock:/var/run/docker.sock ci 

