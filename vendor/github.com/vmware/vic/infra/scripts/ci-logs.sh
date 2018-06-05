#!/bin/bash -e
# Copyright 2016 VMware, Inc. All Rights Reserved.
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
#
# Requirements
# ############
# - gcloud SDK: https://cloud.google.com/sdk/docs/
# - drone cli 0.5: https://github.com/drone/drone#from-source
#
# Examples
# ########
# Grab the logs for your current branch:
# $ ./ci-logs.sh
#
# If the build is running you can stream the logs using the '-s' option:
# $ ./ci-logs.sh -s
#
# Grab logs for a specific build:
# $ ./ci-logs.sh 4835
#
# Grab logs for recent failures on the master branch:
# $ drone build list --format {{.Number}}-{{.Branch}}-{{.Status}} vmware/vic | grep master-failure | cut -d- -f1 | xargs -n1 ./ci-logs.sh
#
# Find container log zip files for failed tests:
# $ grep FAIL ci.log | grep Test-Cases.Group | grep :: | awk '{print $1}' | xargs -n1 -I% bash -c "ls %*.zip"

top=$(git rev-parse --show-toplevel)
repo="vmware/$(basename "$top")"
dir="$top/ci-logs"
drone=${DRONE_CLI:-drone}

while getopts s flag
do
    case $flag in
        d)
            dir="$OPTARG"
            ;;
        s)
            stream=true
            ;;
        *)
            echo "invalid option '$flag'"
            exit 1
            ;;
    esac
done

shift $((OPTIND-1))

build="$1"
job="$2"

export DRONE_SERVER=${DRONE_SERVER:-https://ci-vic.vmware.com}

if [ -z "$DRONE_TOKEN" ] ; then
    echo "DRONE_TOKEN not set (available at $DRONE_SERVER/settings/profile)"
fi

if [ -z "$build" ] ; then
    commit=$(git rev-parse HEAD)
    builds=$($drone build list --format "{{.Number}}-{{.Commit}}" "$repo" | grep "$commit")
    build=$(cut -d- -f1 <<<"$builds")
fi

if [ -z "$job" ] ; then
    job=1
fi

state=$($drone build info "$repo" "$build" --format {{.Status}})


echo "$state"

case "$state" in
    running)
        if [ -z "$stream" ] ; then
            exit
        fi
        ;;
    success|failure|killed)
        stream=""
        ;;
    pending|error)
        exit
        ;;
esac

if [ ! -d "$dir" ] ; then
    mkdir "$dir"
fi

logs="$dir/$build"
mkdir "$logs"

if [ -n "$stream" ] ; then
    echo "Streaming CI log..."

    curl --silent "$DRONE_SERVER/api/stream/$repo/$build/$job?access_token=$DRONE_TOKEN" | \
        grep data: | cut -d: -f2- | grep -v '^$' | tee "$logs/ci.log"
else
    echo "Downloading CI log..."
    curl --silent "$DRONE_SERVER/api/repos/$repo/logs/$build/$job?access_token=$DRONE_TOKEN" > "$logs/ci.log"
fi

url=$(grep https://console.cloud.google.com/ "$logs/ci.log")
if [ -z "$url" ] ; then
    echo "No integration logs link found for build ${build}"
    exit
fi

name=$(basename "$url" | cut -d? -f1)
bucket=$(basename "$(dirname "$(dirname "$url")")")

echo "Downloading integration logs ($name)..."

gsutil cp "gs://$bucket/$name" "$logs/$name"

unzip -d "$logs" "$logs/$name" >/dev/null

if [ "$state" = "failure" ] ; then
    echo "Container logs for failed tests:"
    grep FAIL "$logs/ci.log" | grep Test-Cases.Group | grep :: | awk '{print $1}' | xargs -n1 -I% bash -c "ls $logs/%*.zip" 2>/dev/null
fi
