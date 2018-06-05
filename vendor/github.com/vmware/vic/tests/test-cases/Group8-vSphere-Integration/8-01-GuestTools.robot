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
Documentation  Test 8-01 - Verify VM guest tools integration
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  certs=${false}
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Assert VM Power State
    [Arguments]  ${name}  ${state}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -json ${name}-* | jq -r .VirtualMachines[].Runtime.PowerState
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  ${state}

*** Test Cases ***
Verify VCH VM guest IP is reported
    ${ip}=  Run  govc vm.ip %{VCH-NAME}
    # VCH ip should be the same as docker host param
    Should Contain  %{VCH-PARAMS}  ${ip}

Verify container VM guest IP is reported
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${name}=  Generate Random String  15
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${name} -d ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${ip}=  Run And Return Rc And Output  govc vm.ip ${name}-*
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -json ${name}-* | jq -r .VirtualMachines[].Guest.Net[].IpAddress[]
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${ip}

Stop container VM using guest shutdown
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${name}=  Generate Random String  15
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${name} -d ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.power -s ${name}-*
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  3s  Assert VM Power State  ${name}  poweredOff

Signal container VM using vix command
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${name}=  Generate Random String  15
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${name} -d ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Run  govc vm.ip ${name}-*
    # Invalid command
    ${rc}=  Run And Return Rc  govc guest.start -vm ${name}-* -l ${id} hello world
    Should Be Equal As Integers  ${rc}  1
    # Invalid id (via auth user)
    ${rc}=  Run And Return Rc  govc guest.start -vm ${name}-* kill USR1
    Should Be Equal As Integers  ${rc}  1
    # OK
    ${rc}  ${output}=  Run And Return Rc And Output  govc guest.start -vm ${name}-* -l ${id} kill USR1
    Should Be Equal As Integers  ${rc}  0
