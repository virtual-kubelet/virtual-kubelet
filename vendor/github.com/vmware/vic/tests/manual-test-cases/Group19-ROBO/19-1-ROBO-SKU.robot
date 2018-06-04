# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
Documentation  Test 5-19 - ROBO SKU
Resource  ../../resources/Util.robot
#Suite Setup  Wait Until Keyword Succeeds  10x  10m  ROBO SKU Setup
#Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
ROBO SKU Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${esx1}  ${esx1-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${ESX1}  ${esx1}
    ${esx2}  ${esx2-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${ESX2}  ${esx2}
    ${esx3}  ${esx3-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${ESX3}  ${esx3}

    ${vc}  ${vc-ip}=  Deploy Nimbus vCenter Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${VC}  ${vc}

    Set Suite Variable  @{list}  ${esx1}  ${esx2}  ${esx3}  ${vc}

    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create ha-datacenter
    Should Be Empty  ${out}

    Log To Console  Add ESX host to the VC
    ${out}=  Run  govc host.add -hostname=${esx1-ip} -username=root -dc=ha-datacenter -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK
    ${out}=  Run  govc host.add -hostname=${esx2-ip} -username=root -dc=ha-datacenter -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK
    ${out}=  Run  govc host.add -hostname=${esx3-ip} -username=root -dc=ha-datacenter -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK

    Create A Distributed Switch  ha-datacenter

    Create Three Distributed Port Groups  ha-datacenter

    Add Host To Distributed Switch  ${esx1-ip}
    Add Host To Distributed Switch  ${esx2-ip}
    Add Host To Distributed Switch  ${esx3-ip}

    Add Vsphere License  %{ROBO_LICENSE}

    # Assign license to each of the ESX servers
    Assign Vsphere License  %{ROBO_LICENSE}  ${esx1-ip}
    Assign Vsphere License  %{ROBO_LICENSE}  ${esx2-ip}
    Assign Vsphere License  %{ROBO_LICENSE}  ${esx3-ip}

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Set Environment Variable  TEST_RESOURCE  /ha-datacenter/host/${esx1-ip}/Resources
    Set Environment Variable  TEST_TIMEOUT  30m

*** Test Cases ***
Test
    Log To Console  VIC does not support ROBO SKU yet, waiting on a valid license with serial support for this to work
    #Log To Console  \nStarting test...
    #Install VIC Appliance To Test Server
    #Run Regression Tests
