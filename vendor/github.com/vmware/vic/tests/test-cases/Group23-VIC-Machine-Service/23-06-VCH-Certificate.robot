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
Documentation     Test 23-06 - VCH Certificate
Resource          ../../resources/Util.robot
Resource          ../../resources/Group23-VIC-Machine-Service-Util.robot
Suite Setup       Start VIC Machine Server
Suite Teardown    Stop VIC Machine Server
Test Teardown     Cleanup VIC Appliance On Test Server
Default Tags

*** Keywords ***
Install VIC Machine Without TLS
    [Arguments]  ${vic-machine}=bin/vic-machine-linux  ${appliance-iso}=bin/appliance.iso  ${bootstrap-iso}=bin/bootstrap.iso  ${certs}=${true}  ${vol}=default  ${cleanup}=${true}  ${debug}=1  ${opsuser-args}=${EMPTY}  ${additional-args}=${EMPTY}
    Set Test Environment Variables
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.esxcli network firewall set -e false
    # Attempt to cleanup old/canceled tests
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Networks On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling vSwitches On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Containers On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Resource Pools On Test Server

    ${opsuser-args}=  Get Ops User Args

    Set Suite Variable  ${vicmachinetls}  --no-tls
    Log To Console  \nInstalling VCH to test server with tls disabled...
    ${output}=  Run VIC Machine Command  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${debug}  ${opsuser-args}  ${additional-args}
    Log  ${output}
    Should Contain  ${output}  Installer completed successfully

    Get Docker Params  ${output}  ${certs}
    Log To Console  Installer completed successfully: %{VCH-NAME}...

    [Return]  ${output}


Get VCH Certificate
    [Arguments]    ${vch-id}

    Get Path Under Target    vch/${vch-id}/certificate


Get VCH Certificate Within Datacenter
    [Arguments]    ${vch-id}
    ${dcID}=    Get Datacenter ID

    Get Path Under Target    datacenter/${dcID}/vch/${vch-id}/certificate


Verify Certificate
    Output Should Contain        BEGIN CERTIFICATE
    Output Should Contain        END CERTIFICATE
    Output Should Not Contain    "


Verify Certificate Not Found
    Output Should Contain        no certificate found for VCH
    Output Should Not Contain    BEGIN CERTIFICATE
    Output Should Not Contain    END CERTIFICATE


*** Test Cases ***
Get VCH Certificate
    [Setup]    Install VIC Appliance To Test Server
    ${id}=    Get VCH ID    %{VCH-NAME}

    Get VCH Certificate    ${id}

    Verify Return Code
    Verify Status Ok
    Verify Certificate

    Get VCH Certificate Within Datacenter    ${id}

    Verify Return Code
    Verify Status Ok
    Verify Certificate


Get VCH Certificate No TLS
    [Setup]    Install VIC Machine Without TLS
    ${id}=    Get VCH ID    %{VCH-NAME}

    Get VCH Certificate    ${id}

    Verify Return Code
    Verify Status Not Found
    Verify Certificate Not Found

    Get VCH Certificate Within Datacenter    ${id}

    Verify Return Code
    Verify Status Not Found
    Verify Certificate Not Found
