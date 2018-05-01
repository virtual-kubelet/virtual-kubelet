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

cd ~/vic

git clean -fd
git fetch https://github.com/vmware/vic master
git pull

cp ~/secrets .
tests/longevity-tests/run-longevity.bash $1
id=`docker ps -lq`
echo $id

docker logs -f $id

docker cp $id:/tmp $id
tar -cvzf $id.tar.gz $id
gsutil cp $id.tar.gz gs://vic-longevity-results/

echo $id
rc=`docker inspect --format='{{.State.ExitCode}}' $id`
exit $rc
