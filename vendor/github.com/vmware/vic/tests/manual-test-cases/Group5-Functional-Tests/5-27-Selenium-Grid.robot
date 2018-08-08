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
Documentation  Test 5-27 - Selenium Grid
Resource  ../../resources/Util.robot
Suite Setup  Nimbus Suite Setup  Selenium Grid Test Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
Selenium Grid Test Setup
    [Timeout]    110 minutes
    Log To Console  Starting testbed deployment...
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${name}=  Evaluate  'vic-5-27-' + str(random.randint(1000,9999))  modules=random
    ${out}=  Deploy Nimbus Testbed  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  spec=vic-iscsi-cluster.rb  args=--noSupportBundles --plugin testng --vcvaBuild ${VC_VERSION} --esxBuild ${ESX_VERSION} --testbedName vic-iscsi-cluster --runName ${name}
    Log  ${out}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc-ip}=  Get IP  ${name}.vc.0
    Close Connection

    Set Suite Variable  @{list}  %{NIMBUS_USER}-${name}.esx.0  %{NIMBUS_USER}-${name}.esx.1  %{NIMBUS_USER}-${name}.esx.2  %{NIMBUS_USER}-${name}.esx.3  %{NIMBUS_USER}-${name}.esx.4  %{NIMBUS_USER}-${name}.esx.5  %{NIMBUS_USER}-${name}.iscsi.0  %{NIMBUS_USER}-${name}.vc.0
    Log To Console  Finished Creating Cluster ${name}

    Log To Console  Set environment variables up for GOVC
    Set Environment Variable  GOVC_URL  ${vc-ip}
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23

    Log To Console  Set environment variables up for VIC
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Remove Environment Variable  TEST_DATACENTER
    Set Environment Variable  TEST_DATASTORE  sharedVmfs-0
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  15m

Wait Until Selenium Hub Is Ready
    :FOR  ${idx}  IN RANGE  1  10
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs selenium-hub
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${output}  Selenium Grid hub is up and running
    \   Return From Keyword If  ${status}
    \   Sleep  3
    Fail  Selenium Hub failed to start properly

Wait Until Selenium Node Is Ready
    [Arguments]  ${node-name}
    :FOR  ${idx}  IN RANGE  1  10
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${node-name}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${output}  The node is registered to the hub and ready to use
    \   Return From Keyword If  ${status}
    \   Sleep  3
    Fail  Selenium node ${node-name} failed to start properly

*** Test Cases ***    
Test
    Log To Console  Starting Selenium Grid test...

    Install VIC Appliance To Test Server

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create grid
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -p 4444:4444 --net grid --name selenium-hub selenium/hub:3.9.1
    Should Be Equal As Integers  ${rc}  0
    Wait Until Selenium Hub Is Ready

    :FOR  ${idx}  IN RANGE  1  15
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --net grid -e HOME=/home/seluser -e HUB_HOST=selenium-hub --name chrome${idx} selenium/node-chrome:3.9.1
    \   Should Be Equal As Integers  ${rc}  0

    :FOR  ${idx}  IN RANGE  1  15
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --net grid -e HOME=/home/seluser -e HUB_HOST=selenium-hub --name firefox${idx} selenium/node-firefox:3.9.1
    \   Should Be Equal As Integers  ${rc}  0

    :FOR  ${idx}  IN RANGE  1  15
    \   Wait Until Selenium Node Is Ready  chrome${idx}

    :FOR  ${idx}  IN RANGE  1  15
    \   Wait Until Selenium Node Is Ready  firefox${idx}
