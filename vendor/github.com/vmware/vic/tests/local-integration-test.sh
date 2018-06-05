#!/bin/bash -e
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
# limitations under the License.

# Run robot integration tests locally, no .yml files required.
# Set GITHUB_TOKEN once and switch environments just by changing GOVC_URL

cmd="pybot -L DEBUG --debugfile debug.log"

while getopts t: flag
do
    case $flag in
        # run a specific test
        t)
            cmd="$cmd --exclude skip -t \"$OPTARG\""
            ;;
    esac
done

shift $((OPTIND-1))

if [ -z "$GITHUB_TOKEN" ] || [ -z "$GOVC_URL" ]; then
    echo "usage: GITHUB_TOKEN=... GOVC_URL=... $0 test.robot..."
    exit 1
fi

# check if govc env command works as expected
if ! govc version -require 0.9.0; then
    echo "govc version must be updated"
    exit 1
fi

if [ -z "$DOMAIN" ]; then
  echo "DOMAIN not set, using --no-tlsverify for all tests"
fi

cd "$(git rev-parse --show-toplevel)"

tests=${*#${PWD}/}

# Create a temp drone.yml file
tempfile() {
    # tempprefix=$(basename "$0")
    # mktemp /tmp/${tempprefix}.yml
    mktemp .tmp.drone.XXXXX.yml
    # $(mktemp .tmp.drone.`date +%s`.yml)
}

cleanup() {
    echo "removing temp file $tmpYml"
    rm -f $tmpYml
}
tmpYml=$(tempfile)
#cleanup temp file on exit
trap cleanup EXIT

cat > $tmpYml <<CONFIG
---
workspace:
  base: /go
  path: src/github.com/vmware/vic

pipeline:
  vic-integration-test-on-pr:
    image: gcr.io/eminent-nation-87317/vic-integration-test:1.48
    pull: true
    environment:
      GITHUB_AUTOMATION_API_KEY: $GITHUB_TOKEN
      TEST_URL_ARRAY:   "$(govc env -x GOVC_URL_HOST)"
      TEST_USERNAME:    "$(govc env GOVC_USERNAME)"
      TEST_PASSWORD:    "$(govc env GOVC_PASSWORD)"
      TEST_DATASTORE:   "${GOVC_DATASTORE:-$(basename "$(govc ls datastore | head -1)")}"
      TEST_RESOURCE:    "${GOVC_RESOURCE_POOL:-$(govc ls host/*/Resources)}"
      BRIDGE_NETWORK:   "$BRIDGE_NETWORK"
      PUBLIC_NETWORK:   "$PUBLIC_NETWORK"
      DOMAIN:           "$DOMAIN"
      TARGET_VCH:       "$TARGET_VCH"
      BIN: bin
      GOPATH: /go
      SHELL: /bin/bash
      TEST_TIMEOUT: 60s
      GOVC_INSECURE: "true"
    commands:
      - $cmd ${tests:-tests/test-cases}
CONFIG

drone exec --local $tmpYml
