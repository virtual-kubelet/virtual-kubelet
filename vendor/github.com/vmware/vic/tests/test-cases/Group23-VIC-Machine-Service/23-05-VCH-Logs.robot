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
Documentation     Test 23-05 - VCH Logs
Resource          ../../resources/Util.robot
Resource          ../../resources/Group23-VIC-Machine-Service-Util.robot
Suite Setup       Start VIC Machine Server
Suite Teardown    Stop VIC Machine Server
Test Setup        Install VIC Appliance To Test Server
Test Teardown     Cleanup VIC Appliance On Test Server
Default Tags

*** Keywords ***
Get VCH Log
    [Arguments]    ${vch-id}

    Get Path Under Target    vch/${vch-id}/log


Get VCH Log Within Datacenter
    [Arguments]    ${vch-id}
    ${dcID}=    Get Datacenter ID

    Get Path Under Target    datacenter/${dcID}/vch/${vch-id}/log


Delete Log File From VCH Datastore
    ${filename}=    Run    GOVC_DATASTORE=%{TEST_DATASTORE} govc datastore.ls %{VCH-NAME} | grep vic-machine_
    Should Not Be Empty    ${filename}

    ${output}=      Run    govc datastore.rm "%{VCH-NAME}/${filename}"

    ${filename}=    Run    GOVC_DATASTORE=%{TEST_DATASTORE} govc datastore.ls %{VCH-NAME} | grep vic-machine_
    Should Be Empty        ${filename}


Verify Log
    Output Should Contain    \n
    Output Should Contain    Installer completed successfully


Verify Error String
    Should Be String         ${OUTPUT}


*** Test Cases ***
Get VCH Creation Log succeeds after installation completes
    ${id}=    Get VCH ID    %{VCH-NAME}

    Get VCH Log    ${id}

    Verify Return Code
    Verify Status Ok
    Verify Log

    Get VCH Log Within Datacenter    ${id}

    Verify Return Code
    Verify Status Ok
    Verify Log


Get VCH Creation log errors with 404 after log file is deleted
    ${id}=    Get VCH ID    %{VCH-NAME}

    Delete Log File From VCH Datastore

    Get VCH Log    ${id}

    Verify Return Code
    Verify Status Not Found

    Get VCH Log Within Datacenter    ${id}

    Verify Error String
    Verify Return Code
    Verify Status Not Found
