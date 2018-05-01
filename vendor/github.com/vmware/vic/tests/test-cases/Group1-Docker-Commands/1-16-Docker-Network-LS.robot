# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
Documentation  Test 1-16 - Docker Network LS
Resource  ../../resources/Util.robot
Suite Setup  Run Keywords  Conditional Install VIC Appliance To Test Server  Remove All Container Networks
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Basic network ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  bridge

Docker network ls -q
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls -q
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  NAME
    Should Not Contain  ${output}  DRIVER
    Should Not Contain  ${output}  bridge

Docker network ls filter by name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls -f name=bridge
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  bridge
    @{lines}=  Split To Lines  ${output}
    Length Should Be  ${lines}  2

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls -f name=fakeName
    Should Be Equal As Integers  ${rc}  0
    @{lines}=  Split To Lines  ${output}
    Length Should Be  ${lines}  1
    Should Contain  @{lines}[0]  NAME

Docker network ls filter by label
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create --label=foo foo-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls -f label=foo
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  foo-network
    @{lines}=  Split To Lines  ${output}
    Length Should Be  ${lines}  2

Docker network ls --no-trunc
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls --no-trunc
    Should Be Equal As Integers  ${rc}  0
    @{lines}=  Split To Lines  ${output}
    @{line}=  Split String  @{lines}[1]
    Length Should Be  @{line}[0]  64

