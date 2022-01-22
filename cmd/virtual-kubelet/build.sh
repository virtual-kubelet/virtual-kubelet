#! /bin/bash

set -ex

CG_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o vk

docker build -t vk:latest .

rm -rf vk
