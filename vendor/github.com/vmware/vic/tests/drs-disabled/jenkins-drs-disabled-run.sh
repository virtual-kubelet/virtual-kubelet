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
input=$(gsutil ls -l gs://vic-engine-builds/vic_* | grep -v TOTAL | sort -k2 -r | head -n1 | xargs | cut -d ' ' -f 3 | cut -d '/' -f 4)

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
      mkdir vic/bin
      echo "Extracting .tar.gz"
      tar xvzf $input -C vic/bin --strip 1
      if [ -f "vic/bin/vic-machine-linux" ]
      then
      echo "tar extraction complete.."
      canContinue="Yes"
      break
      else
      echo "tar extraction failed"
      canContinue="No"
      rm -rf vic/bin
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

# Run the Robot tests in a container
envfile="vic-internal/drs-disabled-secrets.list"
image="gcr.io/eminent-nation-87317/vic-integration-test:1.48"
cmd="pabot --processes 1 --removekeywords TAG:secret -d drs-disabled tests/manual-test-cases/Group19-DRS-Disabled"
docker run --rm -v $PWD/vic:/go --env-file $envfile $image $cmd

# Remove the VIC binary tar file
rm $input

cat vic/drs-disabled/pabot_results/*/stdout.txt | grep -E '::|\.\.\.' | grep -E 'PASS|FAIL' > console.log

# Format the email output for Jenkins
sed -i -e 's/^/<br>/g' console.log
sed -i -e 's|PASS|<font color="green">PASS</font>|g' console.log
sed -i -e 's|FAIL|<font color="red">FAIL</font>|g' console.log

# Run the log upload script in a container
cmd="tests/drs-disabled/upload-logs.sh"
docker run --rm -e BUILD_TIMESTAMP -v $PWD/vic:/go --env-file $envfile $image $cmd

