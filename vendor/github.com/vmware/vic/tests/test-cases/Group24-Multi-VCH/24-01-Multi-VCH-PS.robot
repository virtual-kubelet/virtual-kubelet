# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 24-01 - Verify docker ps from multiple vchs do not influence each other
Resource  ../../resources/Util.robot
Suite Teardown  Cleanup VCHs

*** Keywords ***
Clean Up VCHs
    Run Keyword And Continue On Failure  Cleanup VIC Appliance On Test Server
    Set Environment Variable  VCH-NAME  ${old-vm}
    Set Environment Variable  BRIDGE_NETWORK  ${old-vch-bridge}
    Set Environment Variable  VCH-PARAMS  ${old-vch-params}
    Set Environment Variable  VIC-ADMIN  ${old-vic-admin}
    Run Keyword And Continue On Failure  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Create Multi VCH - Docker Ps Only Contains The Correct Containers
    ${container1}=  Evaluate  'cvm-vch1-' + str(random.randint(1000,9999))  modules=random
    ${container2}=  Evaluate  'cvm-vch2-' + str(random.randint(1000,9999))  modules=random

    Install VIC Appliance To Test Server  cleanup=${false}  certs=${false}
    Set Suite Variable  ${old-vm}  %{VCH-NAME}
    Set Suite Variable  ${old-vch-params}  %{VCH-PARAMS}
    Set Suite Variable  ${old-vch-bridge}  %{BRIDGE_NETWORK}
    Set Suite Variable  ${old-vic-admin}  %{VIC-ADMIN}

    # make sure we create two different bridge networks
    Remove Environment Variable  BRIDGE_NETWORK

    Install VIC Appliance To Test Server  cleanup=${false}  certs=${false}

    ${rc}  ${output}=  Run And Return Rc And Output  docker ${old-vch-params} create --name ${container1} busybox
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${container2} busybox
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker ${old-vch-params} ps -a
    Should Contain  ${output}  ${container1}
    Should Not Contain  ${output}  ${container2}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Contain  ${output}  ${container2}
    Should Not Contain  ${output}  ${container1}

