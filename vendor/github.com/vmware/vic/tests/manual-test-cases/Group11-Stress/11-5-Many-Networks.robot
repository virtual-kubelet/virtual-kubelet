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
Documentation  Test 11-5-Many-Networks
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Many Networks
    Log To Console  Create 1000 networks
    :FOR  ${idx}  IN RANGE  0  1000
    \   Log To Console  \nCreate network ${idx}
    \   ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} network create net${idx}
    \   Should Be Equal As Integers  ${rc}  0

    ${out}=  Run  docker %{VCH-PARAMS} pull busybox
    ${container}=  Run  docker %{VCH-PARAMS} create --net=net999 busybox ping -C2 google.com
    ${out}=  Run  docker %{VCH-PARAMS} start ${container}
    Should Contain  ${out}  2 packets transmitted, 2 received

    Run Regression Tests