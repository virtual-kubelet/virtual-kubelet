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
Documentation  Test 21-2 - Artifactory
Resource  ../../resources/Util.robot
Suite Setup  Nimbus Suite Setup  Artifactory Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
Artifactory Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    Log To Console  \nStarting test...

    ${esx1}  ${esx1_ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  @{list}  ${esx1}
    Set Suite Variable  ${ESX1}  ${esx1}
    Set Suite Variable  ${ESX1_IP}  ${esx1_ip}

    Set Environment Variable  TEST_URL_ARRAY  ${ESX1_IP}
    Set Environment Variable  TEST_URL  ${ESX1_IP}
    Set Environment Variable  TEST_USERNAME  root
    Set Environment Variable  TEST_PASSWORD  ${NIMBUS_ESX_PASSWORD}
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  HOST_TYPE  ESXi
    Remove Environment Variable  TEST_DATACENTER
    Remove Environment Variable  TEST_RESOURCE
    Remove Environment Variable  BRIDGE_NETWORK
    Remove Environment Variable  PUBLIC_NETWORK

*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Install VIC Appliance To Test Server  additional-args=--insecure-registry=vic-docker-local.artifactory.eng.vmware.com

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u vic-deployer -p vmware!123 http://vic-docker-local.artifactory.eng.vmware.com/
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Login Succeeded
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull vic-docker-local.artifactory.eng.vmware.com/busybox:1
    Should Be Equal As Integers  ${rc}  0
