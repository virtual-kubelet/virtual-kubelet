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

if [ $# -ne 1 ]; then
    echo "Usage: $0 harbor-version"
    exit 1
fi
version=$1
[ -e harbor ] \
    && echo "harbor exists. Delete it if you want to install a newer version and re-run $0" \
    && pushd harbor && docker-compose start && popd && exit 0

echo "Pulling down version ${version} of Harbor..."
wget https://storage.googleapis.com/harbor-releases/harbor-online-installer-v${version}.tgz -qO - | tar xz

echo "Configuring Harbor"
sed -i 's/hostname = reg.mydomain.com/hostname = vic-executor1.eng.vmware.com/g' harbor/harbor.cfg

echo "Installing & starting Harbor"
sudo ./harbor/install.sh

echo "Logging into Harbor"
docker login vic-executor1.eng.vmware.com --username=admin --password="Harbor12345"

echo "Pull required busybox image for longevity test and put it in Harbor"
docker pull busybox
docker tag busybox vic-executor1.eng.vmware.com/library/busybox
docker push vic-executor1.eng.vmware.com/library/busybox
