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
Documentation   Test 1-39 - Docker Stats
Resource        ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Get Average Active Memory
    [Arguments]  ${vm}  ${samples}=6

    ${rc}  ${memValues}=  Run And Return Rc And Output  govc metric.sample -n ${samples} -json ${vm} mem.active.average | jq -r .Sample[].Value[].Value[]
    Should Be Equal As Integers  ${rc}  0
    @{memList}=  Split To Lines  ${memValues}
    :FOR  ${mem}  IN  @{memList}
    \    ${num}=  Convert To Integer  ${mem}
    \    ${vmomiMemory}=  Set Variable If  ${num} > 0  ${num}  0

    [Return]  ${vmomiMemory}

Create test containers
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name stresser ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  STRESSED  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name stopper ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  STOPPER  ${output}
    ${stress}=  Get Container ShortID  %{STRESSED}
    Set Environment Variable  VM-PATH  vm/%{VCH-NAME}/*${stress}

Check Memory Usage
    ${vmomiMemory}=  Get Average Active Memory  %{VM-PATH}
    Should Be True  ${vmomiMemory} > 0
    [Return]  ${vmomiMemory}

Get Memory From Stats
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stats --no-stream %{STRESSED}
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Get Line  ${output}  -1
    ${short}=  Get Container ShortID  %{STRESSED}
    Should Contain  ${output}  ${short}
    ${vals}=  Split String  ${output}
    ${vicMemory}=  Get From List  ${vals}  7
    # only care about the integer value of memory usage
    ${vicMemory}=  Fetch From Left  ${vicMemory}  .
    [Return]  ${vicMemory}

Check Memory From Stats
    ${vicMemory}=  Get Memory From Stats
    Should Be True  ${vicMemory} > 0
    [Return]  ${vicMemory}

*** Test Cases ***
Stats No Stream
    Create test containers
    ${vicMemory}=  Wait Until Keyword Succeeds  5x  20s  Check Memory From Stats

    # get the latest memory value for the "stresser" vm
    ${vmomiMemory}=  Wait Until Keyword Succeeds  5x  20s  Check Memory Usage
    # convert to percent and move decimal
    ${percent}=  Evaluate  ${vmomiMemory}/20480
    ${diff}=  Evaluate  abs(${percent}-${vicMemory})
    # due to timing we could see some variation, but shouldn't exceed 5%
    Should Be True  ${diff} < 5

Stats No Stream All Containers
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stats --no-stream -a
    Should Be Equal As Integers  ${rc}  0
    ${stress}=  Get Container ShortID  %{STRESSED}
    ${stop}=  Get Container ShortID  %{STOPPER}
    Should Contain  ${output}  ${stress}
    Should Contain  ${output}  ${stop}

Stats API Memory Validation
    ${status}=  Run Keyword And Return Status  Environment Variable Should Be Set  DOCKER_CERT_PATH
    ${certs}=  Set Variable If  ${status}  --cert %{DOCKER_CERT_PATH}/cert.pem --key %{DOCKER_CERT_PATH}/key.pem  ${EMPTY}
    ${rc}  ${apiMem}=  Run And Return Rc And Output  curl -sk ${certs} -H "Accept: application/json" -H "Content-Type: application/json" -X GET https://%{VCH-IP}:%{VCH-PORT}/containers/%{STRESSED}/stats?stream=false | jq -r .memory_stats.usage
    Should Be Equal As Integers  ${rc}  0
    ${stress}=  Get Container ShortID  %{STRESSED}
    ${vmomiMemory}=  Get Average Active Memory  %{VM-PATH}
    Should Be Equal As Integers  ${rc}  0
    ${vmomiMemory}=  Evaluate  ${vmomiMemory}*1024
    ${diff}=  Evaluate  ${apiMem}-${vmomiMemory}
    ${diff}=  Set Variable  abs(${diff})
    Should Be True  ${diff} < 1000

Stats API CPU Validation
    ${status}=  Run Keyword And Return Status  Environment Variable Should Be Set  DOCKER_CERT_PATH
    ${certs}=  Set Variable If  ${status}  --cert %{DOCKER_CERT_PATH}/cert.pem --key %{DOCKER_CERT_PATH}/key.pem  ${EMPTY}
    ${rc}  ${apiCPU}=  Run And Return Rc And Output  curl -sk ${certs} -H "Accept: application/json" -H "Content-Type: application/json" -X GET https://%{VCH-IP}:%{VCH-PORT}/containers/%{STRESSED}/stats?stream=false
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${apiCPU}  cpu_stats
    Should Contain  ${apiCPU}  cpu_usage
    Should Contain  ${apiCPU}  total_usage

Stats No Stream Non-Existent Container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stats --no-stream fake
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such container: fake

Stats No Stream Specific Stopped Container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stats --no-stream %{STOPPER}
    Should Be Equal As Integers  ${rc}  0
    ${stop}=  Get Container ShortID  %{STOPPER}
    Should Contain  ${output}  ${stop}

Stats API Disk and Network Validation
    ${status}=  Run Keyword And Return Status  Environment Variable Should Be Set  DOCKER_CERT_PATH
    ${certs}=  Set Variable If  ${status}  --cert %{DOCKER_CERT_PATH}/cert.pem --key %{DOCKER_CERT_PATH}/key.pem  ${EMPTY}
    ${rc}  ${api}=  Run And Return Rc And Output  curl -sk ${certs} -H "Accept: application/json" -H "Content-Type: application/json" -X GET https://%{VCH-IP}:%{VCH-PORT}/containers/%{STRESSED}/stats?stream=false
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${api}  ethernet
    Should Contain  ${api}  Read
    Should Contain  ${api}  Write
    Should Contain  ${api}  op
    Should Contain  ${api}  major
    Should Contain  ${api}  minor
