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
Documentation     Test 23-08 - VCH Delete
Resource          ../../resources/Util.robot
Resource          ../../resources/Group23-VIC-Machine-Service-Util.robot
Suite Setup       Start VIC Machine Server
Suite Teardown    Stop VIC Machine Server
Test Setup        Install And Prepare VIC Appliance
Test Teardown     Run  govc datastore.rm %{VCH-NAME}-VOL

Default Tags


*** Keywords ***
Pull Busybox
    Run Docker Command    pull ${busybox}
    Verify Return Code
    Output Should Not Contain    Error

Install And Prepare VIC Appliance
    Install VIC Appliance To Test Server
    Pull Busybox

Re-Install And Prepare VIC Appliance
    Install VIC Appliance To Test Server With Current Environment Variables  cleanup=${false}
    Pull Busybox

Install And Prepare VIC Appliance With Volume Stores
    Set Test Environment Variables
    Set Test Variable    ${VOLUME_STORE_PATH}    %{VCH-NAME}-VOL-foo
    Set Test Variable    ${VOLUME_STORE_NAME}    foo

    Re-Install And Prepare VIC Appliance With Volume Stores

Re-Install And Prepare VIC Appliance With Volume Stores
    Install VIC Appliance To Test Server With Current Environment Variables    additional-args=--volume-store=%{TEST_DATASTORE}/${VOLUME_STORE_PATH}:${VOLUME_STORE_NAME}  cleanup=${false}
    Pull Busybox


Get VCH ID ${name}
    Get Path Under Target    vch
    ${id}=    Run    echo '${OUTPUT}' | jq -r '.vchs[] | select(.name=="${name}").id'
    [Return]    ${id}


Run Docker Command
    [Arguments]    ${command}

    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    docker %{VCH-PARAMS} ${command}
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}


Start Last Container
    Run Docker Command    start ${OUTPUT}
    Verify Return Code
    Output Should Not Contain    Error:


Populate VCH with Powered Off Container
    ${POWERED_OFF_CONTAINER_NAME}=  Generate Random String  15

    Run Docker Command    create --name ${POWERED_OFF_CONTAINER_NAME} ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain    Error

    Set Test Variable    ${POWERED_OFF_CONTAINER_NAME}

Populate VCH with Powered On Container
    ${POWERED_ON_CONTAINER_NAME}=  Generate Random String  15

    Run Docker Command    create --name ${POWERED_ON_CONTAINER_NAME} ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain    Error

    Start Last Container

    Set Test Variable    ${POWERED_ON_CONTAINER_NAME}


Populate VCH with Named Volume on Default Volume Store Attached to Powered Off Container
    ${BASE}=  Generate Random String  15

    Set Test Variable    ${OFF_NV_DVS_VOLUME_NAME}       ${BASE}-nv-dvs
    Set Test Variable    ${OFF_NV_DVS_CONTAINER_NAME}    ${BASE}-c-nv-dvs

    Run Docker Command    volume create --name ${OFF_NV_DVS_VOLUME_NAME}
    Verify Return Code
    Output Should Not Contain    Error

    Run Docker Command    create --name ${OFF_NV_DVS_CONTAINER_NAME} -v ${OFF_NV_DVS_VOLUME_NAME}:/volume ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain    Error

Populate VCH with Named Volume on Named Volume Store Attached to Powered Off Container
    ${BASE}=  Generate Random String  15

    Set Test Variable    ${OFF_NV_NVS_VOLUME_NAME}       ${BASE}-nv-nvs
    Set Test Variable    ${OFF_NV_NVS_CONTAINER_NAME}    ${BASE}-c-nv-nvs

    Run Docker Command    volume create --name ${OFF_NV_NVS_VOLUME_NAME} --opt VolumeStore=${VOLUME_STORE_NAME}
    Verify Return Code
    Output Should Not Contain    Error

    Run Docker Command    create --name ${OFF_NV_NVS_CONTAINER_NAME} -v ${OFF_NV_NVS_VOLUME_NAME}:/volume ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain    Error

Populate VCH with Named Volume on Default Volume Store Attached to Powered On Container
    ${BASE}=  Generate Random String  15

    Set Test Variable    ${ON_NV_DVS_VOLUME_NAME}       ${BASE}-nv-dvs-on
    Set Test Variable    ${ON_NV_DVS_CONTAINER_NAME}    ${BASE}-c-nv-dvs-on

    Run Docker Command    volume create --name ${ON_NV_DVS_VOLUME_NAME}
    Verify Return Code
    Output Should Not Contain    Error

    Run Docker Command    create --name ${ON_NV_DVS_CONTAINER_NAME} -v ${ON_NV_DVS_VOLUME_NAME}:/volume ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain    Error

    Start Last Container

Populate VCH with Named Volume on Named Volume Store Attached to Powered On Container
    ${BASE}=  Generate Random String  15

    Set Test Variable    ${ON_NV_NVS_VOLUME_NAME}       ${BASE}-nv-nvs-on
    Set Test Variable    ${ON_NV_NVS_CONTAINER_NAME}    ${BASE}-c-nv-nvs-on

    Run Docker Command    volume create --name ${ON_NV_NVS_VOLUME_NAME} --opt VolumeStore=${VOLUME_STORE_NAME}
    Verify Return Code
    Output Should Not Contain    Error

    Run Docker Command    create --name ${ON_NV_NVS_CONTAINER_NAME} -v ${ON_NV_NVS_VOLUME_NAME}:/volume ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain    Error

    Start Last Container


Verify Container Exists
    [Arguments]    ${name}

    ${vm}=    Run    govc vm.info -json=true ${name}-* | jq '.VirtualMachines | length'
    Should Be Equal As Integers       ${vm}    1

Verify Container Not Exists
    [Arguments]    ${name}

    ${vm}=    Run    govc vm.info -json=true ${name}-* | jq '.VirtualMachines | length'
    Should Be Equal As Integers       ${vm}    0


Verify VCH Exists
    [Arguments]    ${path}    ${name}=%{VCH-NAME}

    Get Path Under Target             ${path}
    Verify Return Code
    Verify Status Ok

    ${rp}=    Run    govc ls -json=true host/*/Resources/${name} | jq '.elements | length'
    ${vm}=    Run    govc vm.info -json=true ${name} | jq '.VirtualMachines | length'
    ${ds}=    Run    govc datastore.ls ${name}

    Should Be Equal As Integers       ${rp}    1
    Should Be Equal As Integers       ${vm}    1
    Should Not Contain                ${ds}    was not found

Verify VCH Not Exists
    [Arguments]    ${path}    ${name}=%{VCH-NAME}

    Get Path Under Target             ${path}
    Verify Return Code
    Verify Status Not Found

    ${rp}=    Run    govc ls -json=true host/*/Resources/${name} | jq '.elements | length'
    ${vm}=    Run    govc vm.info -json=true ${name} | jq '.VirtualMachines | length'
    ${ds}=    Run    govc datastore.ls ${name}

    Should Be Equal As Integers       ${rp}    0
    Should Be Equal As Integers       ${vm}    0
    Should Contain                    ${ds}    was not found


Verify Volume Exists
    [Arguments]    ${volume}    ${name}

    ${ds}=  Run                       govc datastore.ls ${volume}/volumes/${name}
    Should Not Contain                ${ds}    was not found


Verify Volume Exists Docker
    [Arguments]    ${volume}    ${name}

    ${ds}=  Run                       govc datastore.ls ${volume}/volumes/${name}
    Should Not Contain                ${ds}    was not found

    Run Docker Command                volume ls -q -f name=${name}
    Verify Return Code
    Output Should Contain             ${name}


Verify Volume Not Exists
    [Arguments]    ${volume}    ${name}

    ${ds}=  Run                       govc datastore.ls ${volume}/volumes/${name}
    Should Contain                    ${ds}    was not found


Verify Volume Not Exists Docker
    [Arguments]    ${volume}    ${name}

    ${ds}=  Run                       govc datastore.ls ${volume}/volumes/${name}
    Should Contain                    ${ds}    was not found

    Run Docker Command                volume ls -q -f name=${name}
    Verify Return Code
    Output Should Not Contain         ${name}


Verify Volume Store Exists
    [Arguments]    ${name}

    ${ds}=  Run                       govc datastore.ls ${name}
    Should Not Contain                ${ds}    was not found

# don't currently have the pretty volume store name
#    Run Docker Command                info
#    Verify Return Code
#    Output Should Match Regexp        ^VolumeStores:\s[^$]*${name}

Verify Volume Store Not Exists
    [Arguments]    ${name}

    ${ds}=  Run                       govc datastore.ls ${name}
    Should Contain                    ${ds}    was not found

# don't currently have the pretty volume store name
#    Run Docker Command                info
#    Verify Return Code
#    Output Should Not Match Regexp    ^VolumeStores:\s[^$]*${name}

Cleanup VIC Appliance and Specified Volume
    [Arguments]  ${volume-cleanup}
    Run Keyword And Continue On Failure  Run  govc datastore.rm ${volume-cleanup}-VOL
    Cleanup VIC Appliance On Test Server

*** Test Cases ***
Delete VCH
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Delete Path Under Target          vch/${id}
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}

    # No VCH to delete
    [Teardown]                        Run  govc datastore.rm %{VCH-NAME}-VOL

Delete VCH within datacenter
    ${dc}=    Get Datacenter ID
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 datacenter/${dc}/vch/${id}

    Delete Path Under Target          datacenter/${dc}/vch/${id}
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             datacenter/${dc}/vch/${id}

    # No VCH to delete
    [Teardown]                        Run  govc datastore.rm %{VCH-NAME}-VOL

Delete the correct VCH
    ${one}=    Get VCH ID %{VCH-NAME}
    ${old}=    Set Variable    %{VCH-NAME}

    Install VIC Appliance To Test Server

    ${two}=    Get VCH ID %{VCH-NAME}

    Should Not Be Equal    ${one}    ${two}

    # This will fail when run outside of drone because "Install VIC Appliance To Test Server"
    # will delete "dangling" VCHs - which means any associated with a drone job id that isn't running
    Verify VCH Exists                 vch/${one}    ${old}
    Verify VCH Exists                 vch/${two}

    Delete Path Under Target          vch/${one}
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${one}    ${old}
    Verify VCH Exists                 vch/${two}

    [Teardown]                        Cleanup VIC Appliance and Specified Volume  ${old}


Delete invalid VCH
    ${id}=    Get VCH ID %{VCH-NAME}

    Delete Path Under Target           vch/INVALID
    Verify Return Code
    Verify Status Not Found

    Verify VCH Exists                  vch/${id}

    [Teardown]                        Cleanup VIC Appliance On Test Server


Delete VCH in invalid datacenter
    ${id}=    Get VCH ID %{VCH-NAME}

    Delete Path Under Target           datacenter/INVALID/vch/${id}
    Verify Return Code
    Verify Status Not Found

    Verify VCH Exists                  vch/${id}

    [Teardown]                        Cleanup VIC Appliance On Test Server


Delete with invalid bodies
    ${id}=    Get VCH ID %{VCH-NAME}

    Delete Path Under Target           vch/${id}    '{"invalid"}'
    Verify Return Code
    Verify Status Bad Request

    Delete Path Under Target           vch/${id}    '{"containers":"invalid"}'
    Verify Return Code
    Verify Status Unprocessable Entity
    Output Should Contain              containers

    Delete Path Under Target           vch/${id}    '{"volume_stores":"invalid"}'
    Verify Return Code
    Verify Status Unprocessable Entity
    Output Should Contain              volume_stores

    Delete Path Under Target           vch/${id}    '{"containers":"invalid", "volume_stores":"all"}'
    Verify Return Code
    Verify Status Unprocessable Entity
    Output Should Contain              containers

    Delete Path Under Target           vch/${id}    '{"containers":"all", "volume_stores":"invalid"}'
    Verify Return Code
    Verify Status Unprocessable Entity
    Output Should Contain              volume_stores

    Verify VCH Exists                  vch/${id}

    [Teardown]                         Cleanup VIC Appliance On Test Server


Delete VCH with powered off container
    ${id}=    Get VCH ID %{VCH-NAME}

    Populate VCH with Powered Off Container

    Verify Container Exists           ${POWERED_OFF_CONTAINER_NAME}
    Verify VCH Exists                 vch/${id}

    Delete Path Under Target          vch/${id}
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}
    Verify Container Not Exists       ${POWERED_OFF_CONTAINER_NAME}

    # No VCH to delete
    [Teardown]                        Run  govc datastore.rm %{VCH-NAME}-VOL

Delete VCH with powered off container deletes files
    ${id}=    Get VCH ID %{VCH-NAME}

    Populate VCH with Powered Off Container

    Verify VCH Exists                 vch/${id}

    Run Docker Command    inspect ${POWERED_OFF_CONTAINER_NAME}
    ${uuid}=                          Run      echo '${OUTPUT}' | jq -r '.[0].Id'
    ${ds}=                            Run      govc datastore.ls ${uuid}
    Should Not Contain                ${ds}    was not found

    Delete Path Under Target          vch/${id}
    Verify Return Code
    Verify Status Accepted

    ${ds}=                            Run      govc datastore.ls ${uuid}
    Should Contain                    ${ds}    was not found

    Verify VCH Not Exists             vch/${id}

    # No VCH to delete
    [Teardown]                        NONE

Delete VCH without deleting powered on container
    ${id}=    Get VCH ID %{VCH-NAME}

    Populate VCH with Powered On Container
    Populate VCH with Powered Off Container

    Verify Container Exists           ${POWERED_ON_CONTAINER_NAME}
    Verify Container Exists           ${POWERED_OFF_CONTAINER_NAME}
    Verify VCH Exists                 vch/${id}

    Delete Path Under Target          vch/${id}
    Verify Return Code
    Verify Status Internal Server Error

    Verify VCH Exists                 vch/${id}
    Verify Container Exists           ${POWERED_ON_CONTAINER_NAME}
    Verify Container Not Exists       ${POWERED_OFF_CONTAINER_NAME}

    [Teardown]                        Cleanup VIC Appliance On Test Server


Delete VCH explicitly without deleting powered on container
    ${id}=    Get VCH ID %{VCH-NAME}

    Populate VCH with Powered On Container
    Populate VCH with Powered Off Container

    Verify Container Exists           ${POWERED_ON_CONTAINER_NAME}
    Verify Container Exists           ${POWERED_OFF_CONTAINER_NAME}
    Verify VCH Exists                 vch/${id}

    Delete Path Under Target          vch/${id}    '{"containers":"off"}'
    Verify Return Code
    Verify Status Internal Server Error

    Verify VCH Exists                 vch/${id}
    Verify Container Exists           ${POWERED_ON_CONTAINER_NAME}
    Verify Container Not Exists       ${POWERED_OFF_CONTAINER_NAME}

    [Teardown]                        Cleanup VIC Appliance On Test Server


Delete VCH and delete powered on container
    ${id}=    Get VCH ID %{VCH-NAME}

    Populate VCH with Powered On Container
    Populate VCH with Powered Off Container

    Verify Container Exists           ${POWERED_ON_CONTAINER_NAME}
    Verify Container Exists           ${POWERED_OFF_CONTAINER_NAME}
    Verify VCH Exists                 vch/${id}

    Delete Path Under Target          vch/${id}    '{"containers":"all"}'
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}
    Verify Container Not Exists       ${POWERED_ON_CONTAINER_NAME}
    Verify Container Not Exists       ${POWERED_OFF_CONTAINER_NAME}

Delete VCH and powered off containers and volumes
    [Setup]    Install And Prepare VIC Appliance With Volume Stores
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Populate VCH with Named Volume on Default Volume Store Attached to Powered Off Container

    Verify Container Exists           ${OFF_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                 ${OFF_NV_DVS_VOLUME_NAME}

    Populate VCH with Named Volume on Named Volume Store Attached to Powered Off Container

    Verify Container Exists           ${OFF_NV_NVS_CONTAINER_NAME}
    Verify Volume Store Exists        ${VOLUME_STORE_PATH}
    Verify Volume Exists              ${VOLUME_STORE_PATH}            ${OFF_NV_NVS_VOLUME_NAME}

    Delete Path Under Target          vch/${id}    '{"containers":"off","volume_stores":"all"}'
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}

    Verify Container Not Exists       ${OFF_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Not Exists    %{VCH-NAME}-VOL
    Verify Volume Not Exists          %{VCH-NAME}-VOL                ${OFF_NV_DVS_VOLUME_NAME}

    Verify Container Not Exists       ${OFF_NV_NVS_CONTAINER_NAME}
    Verify Volume Store Not Exists    ${VOLUME_STORE_PATH}
    Verify Volume Not Exists          ${VOLUME_STORE_PATH}           ${OFF_NV_NVS_VOLUME_NAME}

    # No VCH to delete
    [Teardown]                        NONE

Delete VCH and powered on containers and volumes
    [Setup]    Install And Prepare VIC Appliance With Volume Stores
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Populate VCH with Named Volume on Default Volume Store Attached to Powered On Container

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    Populate VCH with Named Volume on Named Volume Store Attached to Powered On Container

    Verify Container Exists           ${ON_NV_NVS_CONTAINER_NAME}
    Verify Volume Store Exists        ${VOLUME_STORE_PATH}
    Verify Volume Exists              ${VOLUME_STORE_PATH}           ${ON_NV_NVS_VOLUME_NAME}

    Delete Path Under Target          vch/${id}    '{"containers":"all","volume_stores":"all"}'
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}

    Verify Container Not Exists       ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Not Exists    %{VCH-NAME}-VOL
    Verify Volume Not Exists          %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    Verify Container Not Exists       ${ON_NV_NVS_CONTAINER_NAME}
    Verify Volume Store Not Exists    ${VOLUME_STORE_PATH}
    Verify Volume Not Exists          ${VOLUME_STORE_PATH}           ${ON_NV_NVS_VOLUME_NAME}

    # No VCH to delete
    [Teardown]                        NONE

Delete VCH and powered off container and preserve volumes
    [Setup]    Install And Prepare VIC Appliance With Volume Stores
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Populate VCH with Named Volume on Default Volume Store Attached to Powered Off Container

    Verify Container Exists           ${OFF_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${OFF_NV_DVS_VOLUME_NAME}

    Populate VCH with Named Volume on Named Volume Store Attached to Powered Off Container

    Verify Container Exists           ${OFF_NV_NVS_CONTAINER_NAME}
    Verify Volume Store Exists        ${VOLUME_STORE_PATH}
    Verify Volume Exists              ${VOLUME_STORE_PATH}           ${OFF_NV_NVS_VOLUME_NAME}

    Delete Path Under Target          vch/${id}    '{"containers":"off","volume_stores":"none"}'
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}

    Verify Container Not Exists       ${OFF_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${OFF_NV_DVS_VOLUME_NAME}

    Verify Container Not Exists       ${OFF_NV_NVS_CONTAINER_NAME}
    Verify Volume Store Exists        ${VOLUME_STORE_PATH}
    Verify Volume Exists              ${VOLUME_STORE_PATH}           ${OFF_NV_NVS_VOLUME_NAME}

    # Re-use preserved volumes
    Re-Install And Prepare VIC Appliance With Volume Stores
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    # volume should already exist even before use - default volume store
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists Docker       %{VCH-NAME}-VOL                ${OFF_NV_DVS_VOLUME_NAME}

    # confirm volume can be referenced
    Run Docker Command                create --name ${OFF_NV_DVS_CONTAINER_NAME} -v ${OFF_NV_DVS_VOLUME_NAME}:/volume ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain         Error
    Verify Container Exists           ${OFF_NV_DVS_CONTAINER_NAME}

    # volume should already exist even before use - named volume store
    Verify Volume Store Exists        ${VOLUME_STORE_PATH}
    Verify Volume Exists Docker       ${VOLUME_STORE_PATH}           ${OFF_NV_NVS_VOLUME_NAME}

    # confirm volume can be referenced
    Run Docker Command                create --name ${OFF_NV_NVS_CONTAINER_NAME} -v ${OFF_NV_NVS_VOLUME_NAME}:/volume ${busybox} /bin/top
    Verify Return Code
    Output Should Not Contain         Error
    Verify Container Exists           ${OFF_NV_NVS_CONTAINER_NAME}

    [Teardown]                        Run Keywords  Cleanup VIC Appliance On Test Server  Cleanup Datastore On Test Server


Delete VCH and powered on container but preserve volume
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Populate VCH with Named Volume on Default Volume Store Attached to Powered On Container

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    Delete Path Under Target          vch/${id}    '{"containers":"all","volume_stores":"none"}'
    Verify Return Code
    Verify Status Accepted

    Verify VCH Not Exists             vch/${id}

    Verify Container Not Exists       ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    # Re-use preserved volume
    Re-Install And Prepare VIC Appliance
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    # volume should already exist even before use
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists Docker       %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    # confirm volume can be referenced AND USED - confirms the disk is healthy
    Run Docker Command                run --name ${ON_NV_DVS_CONTAINER_NAME} -v ${ON_NV_DVS_VOLUME_NAME}:/volume ${busybox} /bin/touch /volume/hello
    Verify Return Code
    Output Should Not Contain         Error

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}

    [Teardown]                        Run Keywords  Cleanup VIC Appliance On Test Server  Cleanup Datastore On Test Server


Delete VCH and preserve powered on container and volumes
    [Setup]    Install And Prepare VIC Appliance With Volume Stores
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Populate VCH with Named Volume on Default Volume Store Attached to Powered On Container

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    Delete Path Under Target          vch/${id}    '{"containers":"off","volume_stores":"none"}'
    Verify Return Code
    Verify Status Internal Server Error

    Verify VCH Exists                 vch/${id}

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    [Teardown]                        Run Keywords  Cleanup VIC Appliance On Test Server  Cleanup Datastore On Test Server


Delete VCH and preserve powered on container and fail to delete volumes
    [Setup]    Install And Prepare VIC Appliance With Volume Stores
    ${id}=    Get VCH ID %{VCH-NAME}

    Verify VCH Exists                 vch/${id}

    Populate VCH with Named Volume on Default Volume Store Attached to Powered On Container

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    Delete Path Under Target          vch/${id}    '{"containers":"off","volume_stores":"all"}'
    Verify Return Code
    Verify Status Internal Server Error

    Verify VCH Exists                 vch/${id}

    Verify Container Exists           ${ON_NV_DVS_CONTAINER_NAME}
    Verify Volume Store Exists        %{VCH-NAME}-VOL
    Verify Volume Exists              %{VCH-NAME}-VOL                ${ON_NV_DVS_VOLUME_NAME}

    [Teardown]                        Run Keywords  Cleanup VIC Appliance On Test Server  Cleanup Datastore On Test Server
