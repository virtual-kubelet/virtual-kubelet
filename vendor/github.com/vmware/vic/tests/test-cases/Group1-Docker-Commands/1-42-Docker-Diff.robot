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
Documentation  Test 1-42 - Docker Diff
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Make changes to busybox image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create busybox /bin/sh -c "touch a b c; rm -rf /tmp; adduser -D krusty"
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} diff ${id}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  A /a
    Should Contain  ${output}  A /b
    Should Contain  ${output}  A /c
    Should Contain  ${output}  D /tmp
    Should Contain  ${output}  C /etc/passwd
    Should Contain  ${output}  C /etc
    Should Contain  ${output}  C /etc/group
    Should Contain  ${output}  A /etc/group-
    Should Contain  ${output}  C /etc/passwd
    Should Contain  ${output}  A /etc/passwd-
    Should Contain  ${output}  C /etc/shadow
    Should Contain  ${output}  A /etc/shadow-
    Should Contain  ${output}  C /home
    Should Contain  ${output}  A /home/krusty
    Should Not Contain  ${output}  hostname
    Should Not Contain  ${output}  hosts
    Should Not Contain  ${output}  resolv.conf
    Should Not Contain  ${output}  .tether