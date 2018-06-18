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
Documentation  Test 23-1-Future-ESXi-Install
Resource  ../../resources/Util.robot
Suite Setup  Future ESXi Install Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

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

*** Test Cases ***
Test
    Install VIC Appliance To Test Server
    Run Regression Tests

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name nginx -d -p 8080:80 nginx
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl nginx endpoint  %{VCH-IP}:8080
    Should Contain  ${output}  Welcome to nginx!

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name alp alpine ls
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${cur_year}=  Run  date +%Y
    ${future}=  Evaluate  ${cur_year} + 2

    ${out}=  Run  govc host.esxcli system time set -d 10 -H 10 -m 18 -M 04 -y ${future}
    ${out}=  Run  govc host.esxcli hardware clock set -d 10 -H 10 -m 18 -M 04 -y ${future}

    Run Regression Tests

    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl nginx endpoint  %{VCH-IP}:8080
    Should Contain  ${output}  Welcome to nginx!

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} restart alp
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0