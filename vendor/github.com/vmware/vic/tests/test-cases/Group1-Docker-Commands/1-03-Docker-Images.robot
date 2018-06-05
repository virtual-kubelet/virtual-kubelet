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
Documentation  Test 1-03 - Docker Images
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Simple images
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox:1.27.1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox:1.27.0
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images | cut -d ' ' -f 1 | grep busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${count}=  Get Count  ${output}  busybox
    Should Be True  ${count} >= 3

All images
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -a | cut -d ' ' -f 1 | grep busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${count}=  Get Count  ${output}  busybox
    Should Be True  ${count} >= 3

Quiet images
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Should Not Contain  ${output}  busybox
    @{lines}=  Split To Lines  ${output}
    ${count}=  Get Length  ${lines}
    Should Be True  ${count} >= 3
    Length Should Be  @{lines}[1]  12

No-trunc images
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images --no-trunc
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${count}=  Get Count  ${output}  busybox
    Should Be True  ${count} >= 3
    @{lines}=  Split To Lines  ${output}
    @{line}=  Split String  @{lines}[2]
    Length Should Be  @{line}[2]  64

Filter images before
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -f before=busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    @{lines}=  Split To Lines  ${output}
    Should Contain  ${output}  1.27.0
    ${count}=  Get Length  ${lines}
    Should Be True  ${count} >= 3

Filter images since
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -f since=busybox:1.27.0
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    @{lines}=  Split To Lines  ${output}
    ${count}=  Get Length  ${lines}
    Should Be True  ${count} > 0
    Should Contain  ${output}  1.27.1

Tag images
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} tag busybox busybox:cdg
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images busybox
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Should Contain  ${output}  cdg

Specific images
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images busybox:1.27.0
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    @{lines}=  Split To Lines  ${output}
    Length Should Be  ${lines}  2
    Should Contain  ${output}  1.27.0

VIC/docker Image ID consistency
    @{tags}=  Create List  uclibc  glibc  musl

    :FOR  ${tag}  IN  @{tags}
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox:${tag}
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Not Contain  ${output}  Error
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker --tls pull busybox:${tag}
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Not Contain  ${output}  Error
    \   ${rc}  ${vic_id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images | grep -E busybox.*.${tag} |awk '{print $3}'
    \   Should Be Equal As Integers  ${rc}  0
    \   ${rc}  ${docker_id}=  Run And Return Rc And Output  docker --tls images | grep -E busybox.*.${tag} |awk '{print $3}'
    \   Should Be Equal  ${vic_id}  ${docker_id}
