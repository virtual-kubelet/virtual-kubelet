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
Documentation  Test 1-25 - Docker Port Map
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Create container with port mappings
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 10000:80 -p 10001:80 --name webserver ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start webserver
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{EXT-IP}  10000
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{EXT-IP}  10001

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop webserver
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  curl %{EXT-IP}:10000 --connect-timeout 5
    Should Not Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  curl %{EXT-IP}:10001 --connect-timeout 5
    Should Not Be Equal As Integers  ${rc}  0

Create container with conflicting port mapping
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 8083:80 --name webserver2 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 8083:80 --name webserver3 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start webserver2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start webserver3
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  port 8083 is not available

Create container with port range
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 8081-8088:80 --name webserver5 ${nginx}
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  host port ranges are not supported for port bindings

Create container with host ip
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 10.10.10.10:8088:80 --name webserver5 ${nginx}
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  host IP for port bindings is only supported for 0.0.0.0 and the public interface IP address

Create container with host ip equal to 0.0.0.0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 0.0.0.0:8088:80 --name webserver5 ${nginx}
    Should Be Equal As Integers  ${rc}  0

Create container with host ip equal to public IP
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p %{EXT-IP}:8089:80 --name webserver6 ${nginx}
    Should Be Equal As Integers  ${rc}  0

Create container without specifying host port
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 6379 --name test-redis redis:alpine
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start test-redis
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop test-redis
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Run after exit remapping mapped ports
    Pass Execution  Disabled until we can figure out how to do attach in Robot tests
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f $(docker %{VCH-PARAMS} ps -aq)

    ${rc}  ${output}=  Run And Return Rc And Output  mkfifo /tmp/fifo1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -id --name ctr1 -p 1900:9999 -p 2200:2222 busybox /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} attach ctr1 < /tmp/fifo1
    Should Be Equal As Integers  ${rc}  0
    Sleep  5
    ${rc}  ${output}=  Run And Return Rc And Output  echo q > /tmp/fifo1
    ${result}=  Wait for process  sh1
    Log  ${result.stdout}
    Log  ${result.stderr}
    Should Be Equal As Integers  ${result.rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Log  ${output}
    Should Not Contain  ${output}  Running

    ${rc}  ${output}=  Run And Return Rc And Output  mkfifo /tmp/fifo2
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -id --name ctr2 -p 1900:9999 -p 3300:3333 busybox /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} attach ctr2 < /tmp/fifo2
    Should Be Equal As Integers  ${rc}  0
    Sleep  5
    ${rc}  ${output}=  Run And Return Rc And Output  echo q > /tmp/fifo2
    ${result}=  Wait for process  sh2
    Log  ${result.stdout}
    Log  ${result.stderr}
    Should Be Equal As Integers  ${result.rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Log  ${output}
    Should Not Contain  ${output}  Running

Remap mapped ports after OOB Stop
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f $(docker %{VCH-PARAMS} ps -aq)

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 10000:80 -p 10001:80 --name ctr3 busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ctr3
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

    Power Off VM OOB  ctr3*

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 10000:80 -p 20000:22222 --name ctr4 busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ctr4
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Remap mapped ports after OOB Stop and Remove
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f $(docker %{VCH-PARAMS} ps -aq)

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd -p 5001:80 --name nginx1 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  5001

    Power Off VM OOB  nginx1*
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm nginx1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd -p 5001:80 --name nginx2 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  5001

Container to container traffic via VCH public interface
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f $(docker %{VCH-PARAMS} ps -aq)

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${nginx}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --net bridge -p 8085:80 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${containerID}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

    Sleep  10

    ${rc}  ${ip}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect bridge | jq '.[0].Containers."${containerID}".IPv4Address'
    ${ip}=  Split String  ${ip}  /
    ${nginx-ip}=  Set Variable  @{ip}[0]
    ${nginx-ip}=  Strip String  ${nginx-ip}  characters="

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name anjunabeats busybox /bin/ash -c "wget -O index.html %{EXT-IP}:8085; md5sum index.html"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    # Verify hash of nginx default index.html
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs anjunabeats
    Log  ${output}
    Should Contain  ${output}  e3eb0a1df437f3f97a64aca5952c8ea0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name abgt250 busybox /bin/ash -c "wget -O index.html ${nginx-ip}:80; md5sum index.html"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    # Verify hash of nginx default index.html
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs abgt250
    Log  ${output}
    Should Contain  ${output}  e3eb0a1df437f3f97a64aca5952c8ea0

Remap mapped port after stop container, and then remove stopped container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f $(docker %{VCH-PARAMS} ps -aq)

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd -p 6001:80 --name remap1 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  6001

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop remap1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd -p 6001:80 --name remap2 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  6001

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm remap1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  6001
