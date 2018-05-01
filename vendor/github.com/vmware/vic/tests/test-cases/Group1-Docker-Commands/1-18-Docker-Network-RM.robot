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
Documentation  Test 1-18 - Docker Network RM
Resource  ../../resources/Util.robot
Suite Setup  Run Keywords  Conditional Install VIC Appliance To Test Server  Remove All Container Networks
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Basic network remove
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network rm test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  test-network

Multiple network remove
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network2
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network3
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network rm test-network2 ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  test-network2
    Should Not Contain  ${output}  test-network3

Remove already removed network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network rm test-network
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: network test-network not found

Remove network with running container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect test-network ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network rm test-network
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: test-network has active endpoints

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network rm test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  test-network

Add and remove network multiple times
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0

    : FOR  ${INDEX}  IN RANGE  1  30
    \     ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} network create foo
    \     Should Be Equal As Integers  ${rc}  0
    \     ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} network rm foo
    \     Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  ${output2}
