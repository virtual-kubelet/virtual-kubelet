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
Documentation  Test 11-3-Many-Containers
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Docker run 1000 containers rapidly
    ${pids}=  Create List

    Log To Console  \nRun 1000 containers rapidly
    :FOR  ${idx}  IN RANGE  0  1000
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} run busybox date  shell=True
    \   Append To List  ${pids}  ${pid}

    Log To Console  \nWait for them to finish and check their RC
    :FOR  ${pid}  IN  @{pids}
    \   Log To Console  \nWaiting for ${pid}
    \   ${res}=  Wait For Process  ${pid}
    \   Should Be Equal As Integers  ${res.rc}  0

    Run Regression Tests