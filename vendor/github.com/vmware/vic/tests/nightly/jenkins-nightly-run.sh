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

if [[ $1 != "6.0" && $1 != "6.5" ]]; then
    echo "Please specify a target cluster. One of: 6.0, 6.5"
    exit 1
fi
target="$1"
echo "Target cluster: "$target

input=$(gsutil ls -l gs://vic-engine-builds/vic_* | grep -v TOTAL | sort -k2 -r | head -n1 | xargs | cut -d ' ' -f 3 | cut -d '/' -f 4)
buildNumber=${input:4}

n=0
   until [ $n -ge 5 ]
   do
      echo "Retry.. $n"
      echo "Downloading gcp file $input"
      wget https://storage.googleapis.com/vic-engine-builds/$input
      if [ -f "$input" ]
      then
      echo "File found.."
      break
      else
      echo "File NOT found"
      fi
      n=$[$n+1]
      sleep 15
   done

n=0
   until [ $n -ge 5 ]
   do
      mkdir bin
      echo "Extracting .tar.gz"
      tar xvzf $input -C bin/ --strip 1
      if [ -f "bin/vic-machine-linux" ]
      then
      echo "tar extraction complete.."
      canContinue="Yes"
      break
      else
      echo "tar extraction failed"
      canContinue="No"
      rm -rf bin
      fi
      n=$[$n+1]
      sleep 15
   done

if [[ $canContinue = "No" ]]; then
    echo "Tarball extraction failed..quitting the run"
    break
else
    echo "Tarball extraction passed, Running nightlies test.."
fi

if [[ $target == "6.0" ]]; then
    echo "Executing nightly tests on vSphere 6.0"
    pabot --processes 4 --removekeywords TAG:secret --exclude nsx --variable ESX_VERSION:$ESX_60_VERSION --variable VC_VERSION:$VC_60_VERSION -d 60/$i tests/manual-test-cases/Group5-Functional-Tests tests/manual-test-cases/Group13-vMotion tests/manual-test-cases/Group21-Registries tests/manual-test-cases/Group23-Future-Tests
    cat 60/pabot_results/*/stdout.txt | grep '::' | grep -E 'PASS|FAIL' > console.log
elif [[ $target == "6.5" ]]; then
    echo "Executing nightly tests on vSphere 6.5"
    pabot --processes 4 --removekeywords TAG:secret -d 65/$i tests/manual-test-cases/Group5-Functional-Tests tests/manual-test-cases/Group13-vMotion tests/manual-test-cases/Group21-Registries tests/manual-test-cases/Group23-Future-Tests
    cat 65/pabot_results/*/stdout.txt | grep '::' | grep -E 'PASS|FAIL' > console.log
fi
# Pretty up the email results
sed -i -e 's/^/<br>/g' console.log
sed -i -e 's|PASS|<font color="green">PASS</font>|g' console.log
sed -i -e 's|FAIL|<font color="red">FAIL</font>|g' console.log

# See if any VMs leaked
timeout 60s sshpass -p $NIMBUS_PASSWORD ssh -o StrictHostKeyChecking\=no $NIMBUS_USER@$NIMBUS_GW nimbus-ctl list

tests/nightly/upload-logs.sh $target_$BUILD_TIMESTAMP
