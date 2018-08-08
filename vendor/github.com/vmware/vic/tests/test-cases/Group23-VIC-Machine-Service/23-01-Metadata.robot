# Copyright 2017 VMware, Inc. All Rights Reserved.
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

*** Settings ***
Documentation     Test 23-01 - Version
Resource          ../../resources/Util.robot
Resource          ../../resources/Group23-VIC-Machine-Service-Util.robot
Suite Setup       Start VIC Machine Server
Suite Teardown    Stop VIC Machine Server
Default Tags


*** Keywords ***
Get Version
    ${out}=  Run  netstat -l | grep 1337
    Log  ${out}
    ${out}=  Run  ps aux | grep vic-machine-server
    Log  ${out}
    Get Path    version
    Verify Return Code

Get Hello
    Get Path    hello

Verify Version
    Output Should Match Regexp    v\\d+\\.\\d+\\.\\d+-(\\w+-)?\\d+-[a-f0-9]+
    Output Should Not Contain     "

Verify Hello
    Output Should Contain         success
    Output Should Not Contain     "


*** Test Cases ***
Get Version
    Wait Until Keyword Succeeds  5x  1s  Get Version

    Verify Status Ok
    Verify Version


Get Hello
    Get Hello

    Verify Return Code
    Verify Status Ok
    Verify Hello
