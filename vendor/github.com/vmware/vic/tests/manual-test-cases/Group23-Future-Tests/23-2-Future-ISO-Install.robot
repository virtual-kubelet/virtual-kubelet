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
Documentation  Test 23-2-Future-ISO-Install
Resource  ../../resources/Util.robot
Suite Setup  Future ESXi Install Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}
Test Teardown  Reset Domain Variable

*** Keywords ***
Future ESXi Install Setup
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${esx}  ${esx-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  @{list}  ${esx}

    Set Environment Variable  TEST_URL_ARRAY  ${esx-ip}
    Set Environment Variable  TEST_URL  ${esx-ip}
    Set Environment Variable  TEST_USERNAME  root
    Set Environment Variable  TEST_PASSWORD  ${NIMBUS_ESX_PASSWORD}
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  HOST_TYPE  ESXi
    Remove Environment Variable  TEST_DATACENTER
    Remove Environment Variable  TEST_RESOURCE
    Remove Environment Variable  BRIDGE_NETWORK
    Remove Environment Variable  PUBLIC_NETWORK

Curl nginx endpoint
    [Arguments]  ${endpoint}
    ${rc}  ${output}=  Run And Return Rc And Output  curl ${endpoint}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${output}

Reset Domain Variable
    ${status}=  Run Keyword And Return Status  Environment Variable Should Be Set  ORIG_DOMAIN
    Run Keyword If  ${status}  Set Environment Variable  DOMAIN  %{ORIG_DOMAIN}

*** Test Cases ***
Test
    Set Environment Variable  ORIG_DOMAIN  %{DOMAIN}
    Set Environment Variable  DOMAIN  ${EMPTY}

    ${cur_year}=  Run  date +%Y
    ${future}=  Evaluate  ${cur_year} + 2

    ${out}=  Run  govc host.esxcli system time set -d 10 -H 10 -m 18 -M 04 -y ${future}
    ${out}=  Run  govc host.esxcli hardware clock set -d 10 -H 10 -m 18 -M 04 -y ${future}

    Install VIC Appliance To Test Server

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} info
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ID: vSphere Integrated Containers