# Copyright 2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 24-02 - Verify vic-machine delete only removes own cVMs
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If Test Failed  Cleanup VCHs

*** Keywords ***
Cleanup VCHs
    Run Keyword And Continue On Failure  Cleanup VIC Appliance On Test Server
    Run Keyword If  '${old-vch}' != '${EMPTY}'  Set Environment Variable  VCH-NAME  ${old-vch}
    Run Keyword If  '${old-vch-bridge}' != '${EMPTY}'  Set Environment Variable  BRIDGE_NETWORK  ${old-vch-bridge}
    Run Keyword If  '${old-vch-params}' != '${EMPTY}'  Set Environment Variable  VCH-PARAMS  ${old-vch-params}
    Run Keyword If  '${old-vic-admin}' != '${EMPTY}'  Set Environment Variable  VIC-ADMIN  ${old-vic-admin}
    Run Keyword And Continue On Failure  Cleanup VIC Appliance On Test Server

*** Test Cases ***
VCH delete only removes its own containers
    ${c1}=  Evaluate  'cvm-vch1-' + str(random.randint(1000,9999))  modules=random

    Install VIC Appliance To Test Server
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} create --name ${c1} ${busybox}
    Should Be Equal As Integers  ${rc}  0

    Set Suite Variable  ${old-vch}  %{VCH-NAME}
    Set Suite Variable  ${old-vch-params}  %{VCH-PARAMS}
    Set Suite Variable  ${old-vch-bridge}  %{BRIDGE_NETWORK}
    Set Suite Variable  ${old-vic-admin}  %{VIC-ADMIN}

    # Unset BRIDGE_NETWORK so the new VCH uses a unique bridge network
    Remove Environment Variable  BRIDGE_NETWORK

    Install VIC Appliance To Test Server  cleanup=${false}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0

    # Clean up the second VCH
    Cleanup VIC Appliance On Test Server

    # The old VCH's cVM should still exist
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${old-vch}/${c1}*
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${c1}

    # Clean up the first VCH
    Set Environment Variable  VCH-NAME  ${old-vch}
    Set Environment Variable  BRIDGE_NETWORK  ${old-vch-bridge}
    Set Environment Variable  VCH-PARAMS  ${old-vch-params}
    Set Environment Variable  VIC-ADMIN  ${old-vic-admin}
    Cleanup VIC Appliance On Test Server