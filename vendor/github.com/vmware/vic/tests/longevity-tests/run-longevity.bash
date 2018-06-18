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

if [[ $1 != "6.0" && $1 != "6.5" ]]; then
    echo "Please specify a target cluster. One of: 6.0, 6.5"
    exit 1
fi

if [[ ! $(grep dns /etc/docker/daemon.json) ]]; then
    echo "NOTE: /etc/docker/daemon.json should contain
{
 \"dns\": [\"10.118.81.1\", \"10.16.188.210\"]
}

 in order for this script to function behind VMW's firewall.

 If the file does not exist, create it & restart the docker daemon before
 attempting to run this script
"
    exit 1
fi

target="$1"

# set an output directory
odir=$PWD"-longevity-test-output-$(date -Iminute | sed 's/:/_/g')"


# set up harbor if necessary
if [[ $(docker ps | grep harbor) == "" ]]; then
    if [[ ${harborVersion} != "" ]]; then
        hversion=${harborVersion}
    else
        hversion="1.2.0"
        echo "No Harbor version specified. Using default $hversion"
    fi
    DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    $DIR/get-and-start-harbor.bash $hversion
fi

echo "Building container images...."
pushd tests/longevity-tests
docker build -q -t longevity-base -f Dockerfile.foundation .
docker build -q -t tests-"$target" -f Dockerfile."${target}" .
popd

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
rm -rf bin
mkdir -p bin
wget -P bin https://storage.googleapis.com/vic-engine-builds/$input -qO - | tar xz -C bin
mv bin/vic/* bin
rmdir bin/vic

echo "Creating container..."
testsContainer=$(docker create --rm -it\
                        -w /go/src/github.com/vmware/vic/ \
                        -v "$odir":/tmp/ -e SYSLOG_VCH_OPTION="${syslogVchOption}" -e DEBUG_VCH_LEVEL="${debugVchLevel}" \
                        tests-"$target" \
                        bash -c \
                        ". secrets && pybot -d /tmp/ /go/src/github.com/vmware/vic/tests/manual-test-cases/Group14-Longevity/14-1-Longevity.robot; rc=$?;\
                 mv *-container-logs.zip /tmp/ 2>/dev/null; \
                 mv VCH-*-vmware.log /tmp/ 2>/dev/null; \
                 mv vic-machine.log /tmp/ 2>/dev/null; \
                 mv index.html* /tmp/ 2>/dev/null; \
                 mv VCH-* /tmp/ 2>/dev/null; \
                 exit $rc")

echo "Copying code and binaries into container...."
cd ..
docker cp vic $testsContainer:/go/src/github.com/vmware/

echo "Running tests.."
echo "Run docker attach $testsContainer to interact with the container or use docker logs -f to simply view test output as the tests run"
docker start $testsContainer

echo "Output can be found in $odir"
