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
Documentation  Test 1-05 - Docker Start
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Simple start
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error:
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error:

Start from image that has no PATH
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull vmware/photon
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error:
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it vmware/photon
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error:

Start non-existent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start fakeContainer
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: No such container: fakeContainer
    Should Contain  ${output}  Error: failed to start containers: fakeContainer

Start with no ethernet card
    # Testing that port layer doesn't hang forever if tether fails to initialize (see issue #2327)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${name}=  Generate Random String  15
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} ${busybox} date
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  govc device.remove -vm ${name}-* ethernet-0
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${name}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  unable to wait for process launch status
    Should Not Contain  ${output}  context deadline exceeded

Serially start 5 long running containers
    # Perf testing reported (see issue #2496)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    :FOR  ${idx}  IN RANGE  0  5
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -t ${busybox} /bin/top
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Not Contain  ${output}  Error:
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${output}
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Not Contain  ${output}  Error:
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs -n1 docker %{VCH-PARAMS} rm -f
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${ubuntu}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    :FOR  ${idx}  IN RANGE  0  5
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -t ${ubuntu} top
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Not Contain  ${output}  Error:
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${output}
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Not Contain  ${output}  Error:
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs -n1 docker %{VCH-PARAMS} rm -f
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Parallel start 5 long running containers
    ${pids}=  Create List
    ${containers}=  Create List
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    :FOR  ${idx}  IN RANGE  0  5
    \   ${output}=  Run  docker %{VCH-PARAMS} create -t ${busybox} /bin/top
    \   Should Not Contain  ${output}  Error
    \   Append To List  ${containers}  ${output}

    :FOR  ${container}  IN  @{containers}
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} start ${container}  shell=True
    \   Append To List  ${pids}  ${pid}

    # Wait for them to finish and check their RC
    :FOR  ${pid}  IN  @{pids}
    \   ${res}=  Wait For Process  ${pid}
    \   Should Be Equal As Integers  ${res.rc}  0

Start a container with removed network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --net test-network ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network rm test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  network test-network not found

Simple start with attach
    Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start -a ${container}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  bin
    Should Contain  ${output}  root
    Should Contain  ${output}  var
