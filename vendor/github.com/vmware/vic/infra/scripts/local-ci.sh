#!/bin/bash
# Copyright 2018 VMware, Inc. All Rights Reserved.
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

##### defaults
secretsfile=""
docker_test="Group1-Docker-Commands"
target_vch=""
odir="ci-results"
ci_container="gcr.io/eminent-nation-87317/vic-integration-test:1.44"
github_api_key=""
test_url=""
test_datastore=""
test_username=""
test_password=""
BASE_DIR=$(dirname $(readlink -f "$BASH_SOURCE"))
vic_dir=${BASE_DIR}/../../

##### utility functions
function usage() {
  echo "Usage: $0 [options]" 1>&2
  echo
  echo "  Options can be provided by commandline argument, environment variable, or a secrets yaml file containing the"
  echo "  variables.  If a secrets file is not provided, this script will attempt to retrieve some info from govc, such as"
  echo "  TEST_URL, TEST_USERNAME, and TEST_PASSWORD."
  echo
  echo "  options:"
  echo "    -t DOCKER_TEST (or env var)"
  echo "    -v TARGET_VCH (or env var) name of VCH"
  echo "    -f SECRETS_FILE (or env var)"
  echo "    -g GITHUB_API_KEY (or env var)"
  echo "    -u TEST_URL (or env var or in secretsfile)"
  echo "    -s TEST_DATASTORE (or env var or in secretsfile)"
  echo "    -n TEST_USERNAME (or env var or in secretsfile)"
  echo "    -p TEST_PASSWORD (or env var or in secretsfile)"
  echo "    -d debug dumps out all the inputs and results of the resulting options"
  echo
  echo "  example:"
  echo "    $0 -t Group1-Docker-Commands -v my_vch -s test.secrets.nested -g xxxxxx"
  echo "    $0 -t 1-01-Docker-Info.robot -v my_vch -s test.secrets.nested -g xxxxxx"
  echo 
  echo "    DOCKER_TEST=Group1-Docker-Commands/1-01-Docker-Info.robot TARGET_VCH=my_vch SECRETS_FILE=test.secrets.nested $0"
  echo
  echo "    $0 -s test.secrets.nested (all params defined in secrets file)"
  exit 1
}

function GetGovcParamsFromEnv() {
  echo "Getting params from GOVC var"
  test_username=$(govc env | grep GOVC_USERNAME | cut -d= -f2)
  test_password=$(govc env | grep GOVC_PASSWORD | cut -d= -f2)
  test_url=$(govc env | grep GOVC_URL | cut -d= -f2)
}

function GetParamsFromSecrets() {
  echo "Getting params from the secrets file"
  secrets_api_key="$(grep 'GITHUB_AUTOMATION_API_KEY' ${secretsfile} | awk '{ print $2 }')"
  secrets_url="$(grep 'TEST_URL_ARRAY' ${secretsfile} | awk '{ print $2 }')"
  secrets_datastore="$(grep -E '\s+TEST_DATASTORE' ${secretsfile} | awk '{ print $2 }')"
  secrets_username="$(grep 'TEST_USERNAME' ${secretsfile} | awk '{ print $2 }')"
  secrets_password="$(grep 'TEST_PASSWORD' ${secretsfile} | awk '{ print $2 }')"
  secrets_docker_test="$(grep 'DOCKER_TEST' ${secretsfile} | awk '{ print $2 }')"
  secrets_target_vch="$(grep 'TARGET_VCH' ${secretsfile} | awk '{ print $2 }')"
}

function DoWork() {
  mkdir -p $odir

  testsContainer=$(docker create -it \
                        -w /vic \
                        -v "$vic_dir:/vic" \
                        -e GOVC_URL="$ip" \
                        -e GOVC_INSECURE=1 \
                        -e GITHUB_AUTOMATION_API_KEY=${github_api_key}\
                        -e TEST_URL_ARRAY=${test_url}\
                        -e TEST_DATASTORE=${test_datastore}\
                        -e TEST_USERNAME=${test_username}\
                        -e TEST_PASSWORD=${test_password}\
                        -e TARGET_VCH=${target_vch}\
                        -e DEBUG_VCH=1\
                        ${ci_container}\
                        bash -c "pybot -d /vic/${odir} /vic/tests/test-cases/"$docker_test"")

  docker start -ai $testsContainer
}

function DebugInputDump() {
  echo "Environment Variables"
  echo "---------------------"
  echo "SECRETS_FILE="${SECRETS_FILE}
  echo "TARGET_VCH="${TARGET_VCH}
  echo "DOCKER_TEST="${DOCKER_TEST}
  echo "GITHUB_API_KEY="${GITHUB_API_KEY}
  echo "TEST_URL="${TEST_URL}
  echo "TEST_DATASTORE"=${TEST_DATASTORE}
  echo "TEST_USERNAME"=${TEST_USERNAME}
  echo "TEST_PASSWORD"=${TEST_PASSWORD}
  echo
  echo "Arguments"
  echo "---------------------"
  echo "SECRETS_FILE="$secretsfile
  echo "TARGET_VCH="$target_vch
  echo "DOCKER_TEST="$docker_Test
  echo "GITHUB_API_KEY="$github_api_key
  echo "TEST_URL="$test_url
  echo "TEST_DATASTORE"=$test_datastore
  echo "TEST_USERNAME"=$test_username
  echo "TEST_PASSWORD"=$test_password
  echo
  echo "Secrets file"
  echo "---------------------"
  echo "TARGET_VCH="$secrets_target_vch
  echo "DOCKER_TEST="$secrets_docker_test
  echo "GITHUB_API_KEY="$secrets_api_key
  echo "TEST_URL="$secrets_url
  echo "TEST_DATASTORE"=$secrets_datastore
  echo "TEST_USERNAME"=$secrets_username
  echo "TEST_PASSWORD"=$secrets_password
}

function DebugDump() {
  echo
  echo "====================="
  echo "SECRETS_FILE="$secretsfile
  echo "TARGET_VCH="$target_vch
  echo "DOCKER_TEST="$docker_test
  echo "GITHUB_API_KEY="$github_api_key
  echo "TEST_URL="$test_url
  echo "TEST_DATASTORE"=$test_datastore
  echo "TEST_USERNAME"=$test_username
  echo "TEST_PASSWORD"=$test_password
  echo "vic-dir"=$vic_dir
}

##### Get command line arguments
while getopts f:t:v:g:u:s:n:p:d flag
do
  case $flag in
    f)
      secretsfile=$OPTARG
      ;;
    t)
      docker_test="$OPTARG"
      ;;
    v)
      target_vch=$OPTARG
      ;;
    g)
      github_api_key=$OPTARG
      ;;
    u)
     test_url=$OPTARG
      ;;
    s)
      test_datastore=$OPTARG
      ;;
    n)
      test_username=$OPTARG
      ;;
    p)
      test_password=$OPTARG
      ;;
    d)
      debug_enabled=1
      ;;
    *)
      usage
      ;;
  esac
done

##### Preconditions...

# There is a priority in the preconditions.  First, environment variable.  Second, secrets file.  Third, command line argument.

secretsfile=${SECRETS_FILE:-$secretsfile}
if [[ -z $secretsfile ]] ; then
  GetGovcParamsFromEnv
else
  GetParamsFromSecrets
fi

if [[ ! -z ${debug_enabled} ]] ; then
    DebugInputDump
fi

target_vch=${TARGET_VCH:-$secrets_target_vch}
docker_test=${DOCKER_TEST:-$secrets_docker_test}
github_api_key=${GITHUB_API_KEY:-$secrets_api_key}
test_url=${TEST_URL:-$secrets_url}
test_datastore=${TEST_DATASTORE:-$secrets_datastore}
test_username=${TEST_USERNAME:-$secrets_username}
test_password=${TEST_PASSWORD:-$secrets_password}
if [[ -z ${target_vch} ]] || [[ -z "${docker_test}" ]] || [[ -z $github_api_key ]] || [[ -z $test_url ]] || [[ -z $test_datastore ]] || [[ -z $test_password ]] ; then
  usage
fi

if [[ ! -z ${debug_enabled} ]] ; then
  DebugDump
fi

##### The actual work
DoWork
