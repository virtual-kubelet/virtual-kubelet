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

import os
from enum import Enum


class TestEnvironment(Enum):
    LOCAL = 0
    DRONE = 1
    LONGEVITY = 2


def getEnvironment():
    if (os.environ.has_key("DRONE_BUILD_NUMBER") and (int(os.environ['DRONE_BUILD_NUMBER']) != 0)):
        return TestEnvironment.DRONE
    elif os.environ.has_key("LONGEVITY"):
        return TestEnvironment.LONGEVITY
    else:
        return TestEnvironment.LOCAL

def getName(image):
        return {TestEnvironment.DRONE: 'wdc-harbor-ci.eng.vmware.com/default-project/{}'.format(image),
            TestEnvironment.LONGEVITY: 'vic-executor1.vcna.io/library/{}'.format(image),
            TestEnvironment.LOCAL: image}[getEnvironment()]

# this global variable (images) is used by the Longevity scripts. If you change this, change those!
# and don't inline it!
images = ['busybox', 'alpine', 'nginx','debian', 'ubuntu', 'redis']
for image in images:
    exec("{} = '{}'".format(image.upper().replace(':', '_').replace('.', '_'), getName(image)))
