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
Documentation     Test 23-02 - VCH List
Resource          ../../resources/Util.robot
Resource          ../../resources/Group23-VIC-Machine-Service-Util.robot
Suite Setup       Setup
Suite Teardown    Teardown
Test Teardown     Run Keyword If Test Failed  Cleanup VIC Appliance On Test Server
Default Tags


*** Keywords ***
Setup
    Start VIC Machine Server
    Install VIC Appliance To Test Server

Teardown
    Terminate All Processes    kill=True

Get VCH List
    Get Path Under Target    vch

Get VCH List Within Datacenter
    ${dcID}=    Get Datacenter ID
    Get Path Under Target    datacenter/${dcID}/vch

Verify VCH List
    ${expectedId}=    Get VCH ID    %{VCH-NAME}

    Property Should Be Equal    .vchs[] | select(.name=="%{VCH-NAME}").id    ${expectedId}

    Property Should Not Be Empty    .vchs[] | select(.name=="%{VCH-NAME}").admin_portal
    Property Should Not Be Empty    .vchs[] | select(.name=="%{VCH-NAME}").docker_host
    Property Should Not Be Empty    .vchs[] | select(.name=="%{VCH-NAME}").upgrade_status
    Property Should Not Be Empty    .vchs[] | select(.name=="%{VCH-NAME}").version

Get VCH List Using Session
    Get Path Under Target Using Session    vch

Get VCH List Within Datacenter Using Session
    ${dcID}=    Get Datacenter ID
    Get Path Under Target Using Session    datacenter/${dcID}/vch

Verify VCH Power State
    [Arguments]  ${expected}
    Property Should Be Equal  .vchs[] | select(.name=="%{VCH-NAME}").power_state  ${expected}


*** Test Cases ***
Get VCH List
    Get VCH List

    Verify Return Code
    Verify Status Ok
    Verify VCH List

Get VCH List Using Session
    Get VCH List Using Session

    Verify Return Code
    Verify Status Ok
    Verify VCH List

Get VCH List Within Datacenter
    Get VCH List Within Datacenter

    Verify Return Code
    Verify Status Ok
    Verify VCH List

Get VCH List Within Datacenter Using Session
    Get VCH List Within Datacenter Using Session

    Verify Return Code
    Verify Status Ok
    Verify VCH List

Verify VCH List Power States
    Get VCH List

    Verify Return Code
    Verify Status Ok
    Verify VCH Power State  poweredOn
    Power Off VM OOB  %{VCH-NAME}

    Get VCH List
    Verify VCH Power State  poweredOff


# TODO: Add test for compute resource (once relevant code is updated to use ID instead of name)
# TODO: Add test for compute resource within datacenter (once relevant code is updated to use ID instead of name)

Get VCH List Within Invalid Datacenter
    Get Path Under Target    datacenter/INVALID/vch

    Verify Return Code
    Verify Status Not Found

Get VCH List Within Invalid Compute Resource
    Get Path Under Target    vch    compute-resource=INVALID

    Verify Return Code
    Verify Status Bad Request

Get VCH List Within Invalid Datacenter and Compute Resource
    Get Path Under Target    datacenter/INVALID/vch    compute-resource=INVALID

    Verify Return Code
    Verify Status Not Found


Get Empty VCH List When No VCH deployed
    Cleanup VIC Appliance On Test Server

    Get VCH List

    Verify Return Code
    Verify Status Ok

    Verify VCH List Empty
