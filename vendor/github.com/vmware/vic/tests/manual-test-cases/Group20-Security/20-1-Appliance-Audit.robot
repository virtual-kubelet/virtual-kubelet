# Copyright 2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

*** Settings ***
Documentation  Test 20-01 - Appliance-Audit
Suite Setup  Setup Test Environment
Suite Teardown  Clean up VIC Appliance And Local Binary
Resource  ../../resources/Util.robot

*** Keywords ***
Setup Test Environment
    [Arguments]  ${version}=${EMPTY}
    Run Keyword If  '${version}' == '${EMPTY}'  Install VIC Appliance To Test Server
    Run Keyword Unless  '${version}' == '${EMPTY}'  Install VIC with version to Test Server  ${version}
    Run Keyword If  '${version}' == '${EMPTY}'  Enable VCH SSH
    Run Keyword Unless  '${version}' == '${EMPTY}'  Enable VCH SSH  vic/vic-machine-linux

Provision lynis
    [Arguments]  ${target}=%{VCH-IP}  ${user}=root  ${password}=%{TEST_PASSWORD}
    Log To Console  \nProvision lynis to vch appliance...
    Open Connection  ${target}
    Login  ${user}  ${password}
    ${output}  ${rc}=  Execute Command  rpm --rebuilddb  return_stdout=True  return_rc=True
    Should Be Equal As Integers  ${rc}  0
    ${output}  ${rc}=  Execute Command  tdnf install -y yum  return_stdout=True  return_rc=True
    Should Be Equal As Integers  ${rc}  0
    ${output}  ${rc}=  Execute Command  yum install -y awk sed git  return_stdout=True  return_rc=True
    Should Be Equal As Integers  ${rc}  0
    ${output}  ${rc}=  Execute Command  cd /usr/local && git clone https://github.com/CISOfy/lynis  return_stdout=True  return_rc=True
    Should Be Equal As Integers  ${rc}  0

Lynis Audit System
    [Arguments]  ${target}=%{VCH-IP}  ${user}=root  ${password}=%{TEST_PASSWORD}  ${lynis_path}=/usr/local/lynis
    Log To Console  \nBegin auditing appliance...
    Open Connection  ${target}
    Login  ${user}  ${password}
    ${output}  ${rc}=  Execute Command  cd ${lynis_path} && ./lynis audit system  return_stdout=True  return_rc=True
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -p '${password}' scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ${user}@${target}:/var/log/lynis* .
    Should Be Equal As Integers  ${rc}  0
    
*** Test Cases ***
Appliance Audit
    Provision lynis
    Lynis Audit System
