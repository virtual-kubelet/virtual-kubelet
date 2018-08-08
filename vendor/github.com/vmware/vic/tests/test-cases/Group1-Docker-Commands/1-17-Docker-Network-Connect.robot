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
Documentation  Test 1-17 - Docker Network Connect
Resource  ../../resources/Util.robot
Suite Setup  Run Keywords  Conditional Install VIC Appliance To Test Server  Remove All Container Networks
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Connect containers to multiple bridge networks overlapping
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create cross1-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create cross1-network2
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${debian}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --net cross1-network --name cross1-container ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect cross1-network2 ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${containerID}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --net cross1-network --name cross1-container2 ${debian} ping -c2 cross1-container
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect cross1-network2 ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow cross1-container2
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name cross1-container3 --net cross1-network ${busybox} ping -c2 cross1-container3
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect cross1-network2 cross1-container3
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start cross1-container3
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow cross1-container3
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

Connect container to a new network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create test-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} ip -4 addr show eth0
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect test-network ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${ips}=  Get Lines Containing String  ${output}  inet
    @{lines}=  Split To Lines  ${ips}
    Length Should Be  ${lines}  2

Connect to non-existent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect test-network fakeContainer
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  not found

Connect to non-existent network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name connectTest3 ${busybox} ifconfig
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect fakeNetwork connectTest3
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  not found

Connect containers to multiple networks non-overlapping
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create cross2-network
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create cross2-network2
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${debian}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${nginx}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --net cross2-network --name cross2-container ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get Container IP  %{VCH-PARAMS}  ${containerID}  cross2-network

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --net cross2-network2 --name cross2-container2 ${debian} ping -c2 ${ip}
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 0 packets received, 100% packet loss

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow cross2-container2
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 0 packets received, 100% packet loss

    # verify that an exposed port on the container does not break down bridge isolation
    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --net cross2-network -p 8080:80 ${nginx}
    Should Be Equal As Integers  ${rc}  0

    ${ip}=  Get Container IP  %{VCH-PARAMS}  ${containerID}  cross2-network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --net cross2-network2 --name cross2-container3 ${debian} ping -c2 ${ip}
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 0 packets received, 100% packet loss

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow cross2-container3
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 0 packets received, 100% packet loss

Connect containers to an internal network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create --internal internal-net
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --net internal-net ${busybox} ping -c1 www.google.com
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Network is unreachable

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create public-net
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --net internal-net --net public-net ${busybox} ping -c2 www.google.com
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --net internal-net ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get Container IP  %{VCH-PARAMS}  ${containerID}  internal-net
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --net internal-net ${busybox} ping -c2 ${ip}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

Check Name Resolution Between Containers On Internal Network
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create --internal mynet
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create pubnet
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name foo --net mynet alpine:latest sleep 10000
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i --name baz --net pubnet -p 80 alpine:latest ping -c3 foo
    Log  ${output}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network connect mynet baz
    Log  ${output}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start -i baz
    Log  ${output}

    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  PING foo
    Should Contain  ${output}  3 packets transmitted, 3 packets received

Connect container to multiple networks concurrently
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create foonet
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create barnet
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create baznet
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${c1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${pid1}=  Start Process  docker %{VCH-PARAMS} network connect foonet ${c1}  shell=True
    ${pid2}=  Start Process  docker %{VCH-PARAMS} network connect barnet ${c1}  shell=True
    ${pid3}=  Start Process  docker %{VCH-PARAMS} network connect baznet ${c1}  shell=True
    ${res1}=  Wait For Process  ${pid1}
    ${res2}=  Wait For Process  ${pid2}
    ${res3}=  Wait For Process  ${pid3}
    Should Be Equal As Integers  ${res1.rc}  0
    Should Be Equal As Integers  ${res2.rc}  0
    Should Be Equal As Integers  ${res3.rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${c1} | jq -c '.[0].NetworkSettings.Networks'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  foonet
    Should Contain  ${output}  barnet
    Should Contain  ${output}  baznet

    ${rc}  ${c1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${c1}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${c1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${c1}
    Should Be Equal As Integers  ${rc}  0
