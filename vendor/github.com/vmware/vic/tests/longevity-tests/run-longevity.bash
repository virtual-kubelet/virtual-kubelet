# Copyright 2017-2018 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License
#!/bin/bash
set -e

while getopts ":d:s:h:" opt; do
  case $opt in
    s)
      syslogAddress=$OPTARG
      ;;
    h)
      harborVersion=$OPTARG
      ;;
    d)
      debugLevel=$OPTARG
      ;;
    \?)
      echo "Usage: $0 [-d <debug level>] [-s <syslog endpoint>] [-h <harbor version>] target-cluster"
      exit 1
      ;;
    :)
      echo "Usage: $0 [-d <debug level>] [-s <syslog endpoint>] [-h <harbor version>] target-cluster"
      exit 1
      ;;
  esac
done

shift $((OPTIND-1))
if [ $# -ne 1 ]; then
    echo "Usage: $0 [-s <syslog endpoint>] [-h <harbor version>] target-cluster"
    exit 1
fi

if [[ $1 != "6.0" && $1 != "6.5" && $1 != "6.7" ]]; then
    echo "Please specify a target cluster. One of: 6.0, 6.5, 6.7"
    exit 1
fi

if [[ ! $(grep dns /etc/docker/daemon.json) || ! $(grep insecure-registries /etc/docker/daemon.json) ]]; then
    echo "NOTE: /etc/docker/daemon.json should contain
{
 \"dns\": [\"10.118.81.1\", \"10.16.188.210\"],
 \"insecure-registries\" : [\"vic-executor1.eng.vmware.com\"]
}

 in order for this script to function behind VMW's firewall.

 If the file does not exist, create it & restart the docker daemon before
 attempting to run this script
"
    exit 1
fi

target="$1"

# set up harbor if necessary
if [[ $(docker ps | grep harbor) == "" ]]; then
    if [[ ${harborVersion} != "" ]]; then
        hversion=${harborVersion}
    else
        hversion="1.3.0"
        echo "No Harbor version specified. Using default $hversion"
    fi
    DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    $DIR/get-and-start-harbor.bash $hversion
fi

if [[ ${debugLevel} != "" ]]; then
    debugVchLevel="${debugLevel}"
else
    debugVchLevel="1"
fi

if [[ ${syslogAddress} != "" ]]; then
    syslogVchOption="--syslog-address ${syslogAddress}"
fi

input=$(gsutil ls -l gs://vic-engine-builds/vic_* | grep -v TOTAL | sort -k2 -r | head -n1 | xargs | cut -d ' ' -f 3 | cut -d '/' -f 4)
echo "Downloading VIC build $input..."
wget https://storage.googleapis.com/vic-engine-builds/${input} -qO - | tar xz -C vic && mv vic/vic vic/bin

docker run -w /go/vic -i \
    --env-file vic-internal/longevity-${target}-secrets.list \
    -e SYSLOG_VCH_OPTION="${syslogVchOption}" \
    -e DEBUG_VCH_LEVEL="${debugVchLevel}" \
    -v $(pwd)/vic:/go/vic gcr.io/eminent-nation-87317/vic-integration-test:1.48 \
    pybot tests/manual-test-cases/Group14-Longevity/14-1-Longevity.robot

