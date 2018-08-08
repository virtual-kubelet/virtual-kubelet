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
Documentation  Test 5-9 - Private Registry
Resource  ../../resources/Util.robot
#Suite Setup  Private Registry Setup
#Suite Teardown  Private Registry Cleanup

*** Keywords ***
Private Registry Setup
    [Timeout]    110 minutes
    ${dockerHost}=  Get Environment Variable  DOCKER_HOST  ${SPACE}
    Remove Environment Variable  DOCKER_HOST
    ${rc}  ${output}=  Run And Return Rc And Output  docker run -d -p 5000:5000 --name registry registry
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker pull busybox
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker tag busybox localhost:5000/busybox:latest
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker push localhost:5000/busybox
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  DOCKER_HOST  ${dockerHost}

Private Registry Cleanup
    ${dockerHost}=  Get Environment Variable  DOCKER_HOST  ${SPACE}
    Remove Environment Variable  DOCKER_HOST
    ${rc}  ${output}=  Run And Return Rc And Output  docker rm -f registry
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  DOCKER_HOST  ${dockerHost}

Pull image
    [Arguments]  ${image}
    Log To Console  \nRunning docker pull ${image}...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Digest:
    Should Contain  ${output}  Status:
    Should Not Contain  ${output}  No such image:

*** Test Cases ***
Pull an image from non-default repo
    Pass Execution  This test needs to be re-written
    Install VIC Appliance To Test Server  vol=default --insecure-registry 172.17.0.1:5000
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  172.17.0.1:5000/busybox
    Cleanup VIC Appliance On Test Server

Pull image from non-whitelisted repo
    Pass Execution  This test needs to be re-written
    Install VIC Appliance To Test Server  vol=default
    ${rc}  ${output}=  Run And Return Rc And Output  docker ${params} pull 172.17.0.1:5000/busybox
    Should Contain  ${output}  Error response from daemon: Head https://172.17.0.1:5000/v2/: http: server gave HTTP response to HTTPS client
