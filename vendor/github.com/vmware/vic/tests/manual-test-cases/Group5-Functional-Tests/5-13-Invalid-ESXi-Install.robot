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
Documentation  Test 5-13 - Invalid ESXi Install
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Invalid ESXi Install Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
Invalid ESXi Install Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    Set Suite Variable  ${datacenter}  datacenter1
    Set Suite Variable  ${cluster}  cls

    ${esx1}  ${esx1-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${ESX1}  ${esx1}
    Set Suite Variable  ${esx1-ip}  ${esx1-ip}
    ${esx2}  ${esx2-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${ESX2}  ${esx2}
    Set Suite Variable  ${esx2-ip}  ${esx2-ip}
    
    ${vc}  ${vc-ip}=  Deploy Nimbus vCenter Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${VC}  ${vc}
    Set Suite Variable  ${vc-ip}  ${vc-ip}

    Set Suite Variable  @{list}  ${esx1}  ${esx2}  ${vc}

    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create ${datacenter}
    Should Be Empty  ${out}

    Log To Console  Create a cluster on the VC
    ${out}=  Run  govc cluster.create ${cluster}
    Should Be Empty  ${out}

    Log To Console  Add ESX host to the VC
    ${out}=  Run  govc cluster.add -hostname=${esx1-ip} -username=root -dc=${datacenter} -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK
    ${out}=  Run  govc cluster.add -hostname=${esx2-ip} -username=root -dc=${datacenter} -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK
    
    Log To Console  Create a distributed switch
    ${out}=  Run  govc dvs.create -dc=${datacenter} test-ds
    Should Contain  ${out}  OK

    Log To Console  Create three new distributed switch port groups for management and vm network traffic
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=test-ds management
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=test-ds vm-network
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=test-ds bridge
    Should Contain  ${out}  OK

    Log To Console  Add all the hosts to the distributed switch
    ${out}=  Run  govc dvs.add -dvs=test-ds -pnic=vmnic1 /${datacenter}/host/${cluster}
    Should Contain  ${out}  OK

    Log To Console  Enable DRS on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /${datacenter}/host/${cluster}
    Should Be Empty  ${out}

*** Test Cases ***
Test
    ${out}=  Run  bin/vic-machine-linux create --target ${esx1-ip} --user root --password e2eFunctionalTest --no-tls --name VCH-invalid-test --force --timeout 30m
    Should Contain  ${out}  Target is managed by vCenter server \\"${vc-ip}\\", please change --target to vCenter server address or select a standalone ESXi
