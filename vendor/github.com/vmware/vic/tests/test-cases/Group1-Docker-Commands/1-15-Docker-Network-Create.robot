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
Documentation  Test 1-15 - Docker Network Create
Resource  ../../resources/Util.robot
Suite Setup  Run Keywords  Conditional Install VIC Appliance To Test Server  Remove All Container Networks
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Basic network create
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  test-network

Network create with label
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create --label=foo=bar label-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  label-network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect -f '{{.Labels}}' label-network
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  map[foo:bar]

Create already created network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  already exists

Create overlay network
    ${status}=  Get State Of Github Issue  1222
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-15-Docker-Network-Create.robot needs to be updated now that Issue #1222 has been resolved
    Log  Issue \#1222 is blocking implementation  WARN
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create -d overlay test-network2
    #Should Be Equal As Integers  ${rc}  1
    #Should Contain  ${output}  Error response from daemon: failed to parse pool request for address space "GlobalDefault" pool "" subpool "": cannot find address space GlobalDefault (most likely the backing datastore is not configured)

Create internal network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create --internal internal-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect -f '{{.Internal}}' internal-network
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${output}  true
