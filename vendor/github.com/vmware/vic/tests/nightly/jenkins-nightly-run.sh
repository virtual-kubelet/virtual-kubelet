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
ESX_60_VERSION="ob-5251623"
VC_60_VERSION="ob-5112509"

ESX_65_VERSION="ob-7867845"
VC_65_VERSION="ob-7867539"

ESX_67_VERSION="ob-8169922"
VC_67_VERSION="ob-8217866"

DEFAULT_LOG_UPLOAD_DEST="vic-ci-logs"
DEFAULT_VCH_BRANCH=""
DEFAULT_VCH_BUILD="*"
DEFAULT_TESTCASES=("tests/manual-test-cases/Group5-Functional-Tests" "tests/manual-test-cases/Group13-vMotion" "tests/manual-test-cases/Group21-Registries" "tests/manual-test-cases/Group23-Future-Tests")

DEFAULT_PARALLEL_JOBS=4
DEFAULT_RUN_AS_OPS_USER=0

VIC_BINARY_PREFIX="vic_"

# This is exported to propagate into the pybot processes launched by pabot
export RUN_AS_OPS_USER=${RUN_AS_OPS_USER:-${DEFAULT_RUN_AS_OPS_USER}}

PARALLEL_JOBS=${PARLLEL_JOBS:-${DEFAULT_PARALLEL_JOBS}}


if [[ $1 != "6.0" && $1 != "6.5" && $1 != "6.7" ]]; then
    echo "Please specify a target version. One of: 6.0, 6.5, 6.7"
    exit 1
fi

# process the CLI arguments
target="$1"
echo "Target version: ${target}"
shift
# Take the remaining CLI arguments as a test case list - this is treated as an array to preserve quoting when passing to pabot
testcases=("${@:-${DEFAULT_TESTCASES[@]}}")

# TODO: the version downloaded by this logic is not coupled with the tests that will be run against it. This should be altered to pull a version that matches the commit SHA of the tests
# we will be running or similar mechanism.
VCH_BUILD=${VCH_BUILD:-${DEFAULT_VCH_BUILD}}
VCH_BRANCH=${VCH_BRANCH:-${DEFAULT_VCH_BRANCH}}
input=$(gsutil ls -l gs://vic-engine-builds/${VCH_BRANCH}${VCH_BRANCH:+/}${VIC_BINARY_PREFIX}${VCH_BUILD} | grep -v TOTAL | sort -k2 -r | head -n1 | xargs | cut -d ' ' -f 3 | cut -d '/' -f 4)

# strip prefix and suffix from archive filename
VCH_BUILD=${input#${VIC_BINARY_PREFIX}}
VCH_BUILD=${VCH_BUILD%%.*}

# Enforce short SHA
GIT_COMMIT=${GIT_COMMIT:0:7}

case "$target" in
    "6.0")
        excludes="--exclude nsx"
        ESX_BUILD=${ESX_BUILD:-$ESX_60_VERSION}
        VC_BUILD=${VC_BUILD:-$VC_60_VERSION}
        ;;
    "6.5")
        ESX_BUILD=${ESX_BUILD:-$ESX_65_VERSION}
        VC_BUILD=${VC_BUILD:-$VC_65_VERSION}
        ;;
    "6.7")
        excludes="--exclude nsx --exclude hetero"
        ESX_BUILD=${ESX_BUILD:-$ESX_67_VERSION}
        VC_BUILD=${VC_BUILD:-$VC_67_VERSION}
        ;;
esac

LOG_UPLOAD_DEST="${LOG_UPLOAD_DEST:-${DEFAULT_LOG_UPLOAD_DEST}}"

n=0 && rm -f "${input}"
until [ $n -ge 5 -o -f "${input}" ]; do
    echo "Retry.. $n"
    echo "Downloading gcp file ${input}"
    wget -nv https://storage.googleapis.com/vic-engine-builds/${input}

    ((n++))
    sleep 15
done

echo "Extracting .tar.gz"
mkdir bin && tar xvzf ${input} -C bin/ --strip 1

if [ ! -f  "bin/vic-machine-linux" ]; then
    echo "Tarball extraction failed..quitting the run"
    rm -rf bin
    exit
else
    VCH_COMMIT=$(bin/vic-machine-linux version | awk -F '-' '{print $NF}')
    echo "Tarball extraction passed, Running nightlies test.."
fi

pabot --processes ${PARALLEL_JOBS} --removekeywords TAG:secret ${excludes} --variable ESX_VERSION:${ESX_BUILD} --variable VC_VERSION:${VC_BUILD} -d ${target} "${testcases[@]}"
cat ${target}/pabot_results/*/stdout.txt | grep '::' | grep -E 'PASS|FAIL' > console.log

# See if any VMs leaked
# TODO: should be a warning until clean, then changed to a failure if any leak
echo "There should not be any VMs listed here"
echo "======================================="
timeout 60s sshpass -p ${NIMBUS_PASSWORD} ssh -o StrictHostKeyChecking\=no ${NIMBUS_USER}@${NIMBUS_GW} nimbus-ctl list
echo "======================================="
echo "If VMs are listed we should investigate why they are leaking"

# archive the logs
logarchive="logs_vch-${VCH_BUILD}-${VCH_COMMIT}_test-${BUILD_ID}-${GIT_COMMIT}_${BUILD_TIMESTAMP}.zip"
/usr/bin/zip -9 -r "${logarchive}" "${target}" *.zip *.log *.debug *.tgz
if [ $? -eq 0 ]; then
    tests/nightly/upload-logs.sh ${logarchive} ${LOG_UPLOAD_DEST}
fi

# Pretty up the email results
sed -i -e 's/^/<br>/g' console.log
sed -i -e 's|PASS|<font color="green">PASS</font>|g' console.log
sed -i -e 's|FAIL|<font color="red">FAIL</font>|g' console.log

