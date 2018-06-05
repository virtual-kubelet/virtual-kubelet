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
Documentation  Test 5-5 - Heterogeneous ESXi
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Heterogenous ESXi Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}
Force Tags  hetero

*** Keywords ***
Heterogenous ESXi Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${vc}=  Evaluate  'VC-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    ${pid-vc}=  Deploy Nimbus vCenter Server Async  ${vc}
    Set Suite Variable  @{list}  %{NIMBUS_USER}-${vc}

    Run Keyword And Ignore Error  Cleanup Nimbus PXE folder  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${esx1}  ${esx1-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  3029944
    Append To List  ${list}  ${esx1}

    Run Keyword And Ignore Error  Cleanup Nimbus PXE folder  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${esx2}  ${esx2-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  5572656
    Append To List  ${list}  ${esx2}

    Run Keyword And Ignore Error  Cleanup Nimbus PXE folder  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${esx3}  ${esx3-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Append To List  ${list}  ${esx3}

    # Finish vCenter deploy
    ${output}=  Wait For Process  ${pid-vc}  timeout=40 minutes  on_timeout=terminate
    Log  ${output.stdout}
    Log  ${output.stderr}
    ${status}=  Run Keyword And Return Status  Should Contain  ${output.stdout}  Overall Status: Succeeded

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc-ip}=  Get IP  ${vc}
    Close Connection

    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin!23
    Set Environment Variable  GOVC_URL  ${vc-ip}

    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create ha-datacenter
    Should Be Empty  ${out}

    Log To Console  Create a cluster on the VC
    ${out}=  Run  govc cluster.create cls
    Should Be Empty  ${out}

    Log To Console  Add ESX host to the VC
    Add Host To VCenter  ${esx1-ip}  root  ha-datacenter  e2eFunctionalTest
    Add Host To VCenter  ${esx2-ip}  root  ha-datacenter  e2eFunctionalTest
    ${vc-ver}=  Run  govc about | grep Version:
    ${vc-ver}=  Fetch From Right  ${vc-ver}  ${SPACE}
    Run Keyword If  '${vc-ver}' == '6.5.0'  Add Host To VCenter  ${esx3-ip}  root  ha-datacenter  e2eFunctionalTest

    ${out}=  Run  govc dvs.create -product-version 5.5.0 -dc=ha-datacenter test-ds
    Should Contain  ${out}  OK

    Create Three Distributed Port Groups  ha-datacenter

    Add Host To Distributed Switch  /ha-datacenter/host/cls

    Log To Console  Enable DRS on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /ha-datacenter/host/cls
    Should Be Empty  ${out}

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Remove Environment Variable  TEST_DATACENTER
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  30m

    ${name}  ${ip}=  Deploy Nimbus NFS Datastore  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Append To List  ${list}  ${name}

    ${out}=  Run  govc datastore.create -mode readWrite -type nfs -name nfsDatastore -remote-host ${ip} -remote-path /store /ha-datacenter/host/cls
    Should Be Empty  ${out}

    Set Environment Variable  TEST_DATASTORE  nfsDatastore

*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Install VIC Appliance To Test Server  certs=${false}  vol=default
    Run Regression Tests
