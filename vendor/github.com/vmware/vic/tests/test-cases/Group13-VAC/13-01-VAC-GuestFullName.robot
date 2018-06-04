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
Documentation  Test 13-01 - Guest Full Name
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Check VCH VM Guest Operating System
    ${rc}  ${output}=  Run And return Rc and Output  govc vm.info %{VCH-NAME} | grep 'Guest name'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Photon - VCH

Create a test container and check Guest Operating System
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name test ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${shortID}=  Get container shortID  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info test-${shortID} | grep 'Guest name'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Photon - Container
