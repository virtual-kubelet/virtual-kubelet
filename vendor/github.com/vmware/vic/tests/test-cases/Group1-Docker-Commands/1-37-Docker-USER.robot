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
Documentation   Test 1-37 - Docker Run As USER
Resource        ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Run Image Specifying NewUser in NewGroup
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run gigawhitlocks/1-37-docker-user-newuser-newgroup:latest
    Should Be Equal As Integers    ${rc}       0
    Should Match Regexp            ${output}   uid=\\d+\\\(newuser\\\)\\s+gid=\\d+\\\(newuser\\\)\\s+groups=\\d+\\\(newuser\\\)

Run Image Specifying UID 2000
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run gigawhitlocks/1-37-docker-user-uid-2000:latest
    Should Be Equal As Integers    ${rc}       0
    Should Contain                 ${output}   uid=2000 gid=0(root)

Run Specifying UID 2000 With -u
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run -u 2000 busybox id
    Should Be Equal As Integers    ${rc}       0
    Should Contain                 ${output}   uid=2000 gid=0(root)

Run Image Specifying UID:GID 2000:2000
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run gigawhitlocks/1-37-docker-user-uid-gid-2000-2000:latest
    Should Be Equal As Integers    ${rc}       0
    Should Contain                 ${output}   uid=2000 gid=2000

Run Specifying UID:GID 2000:2000 With -u
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run -u 2000:2000 busybox id
    Should Be Equal As Integers    ${rc}       0
    Should Contain                 ${output}   uid=2000 gid=2000

Run as Nonexistent User With -u
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run -u nonexistent busybox whoami
    Should Be Equal As Integers    ${rc}       125
    Should Contain                 ${output}   Unable to find user nonexistent

Run as Root with Nonexistent User With -u
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run -u root:nonexistent busybox whoami
    Should Be Equal As Integers    ${rc}       125
    Should Contain                 ${output}   Unable to find group nonexistent

Run as uid 0 group 0 With -u
    ${rc}    ${output}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run -u 0:0 busybox whoami
    Should Be Equal As Integers    ${rc}       0
    Should Contain                 ${output}   root
