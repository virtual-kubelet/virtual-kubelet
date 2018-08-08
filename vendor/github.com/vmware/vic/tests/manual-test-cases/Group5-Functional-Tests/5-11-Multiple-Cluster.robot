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
Documentation  Test 5-11 - Multiple Clusters
Resource  ../../resources/Util.robot
Suite Setup  Nimbus Suite Setup  Multiple Cluster Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup Single VM  '*5-11-multiple-cluster*'  ${false}
Test Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Multiple Cluster Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup Single VM  '*5-11-multiple-cluster*'  ${false}
    Log To Console  \nStarting testbed deploy...
    ${out}=  Deploy Nimbus Testbed  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  spec=vic-multiple-cluster.rb  args=--noSupportBundles --plugin testng --vcvaBuild ${VC_VERSION} --esxBuild ${ESX_VERSION} --testbedName vic-multiple-cluster --runName 5-11-multiple-cluster
    Log  ${out}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc-ip}=  Get IP  5-11-multiple-cluster.vc.0
    Close Connection
    
    Log To Console  Set environment variables up for GOVC
    Set Environment Variable  GOVC_URL  ${vc-ip}
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Remove Environment Variable  TEST_DATACENTER
    Set Environment Variable  TEST_TIMEOUT  15m
    Set Environment Variable  TEST_RESOURCE  cls

    # Get one of the hosts in the cluster we want and make sure we use the correct local datastore
    ${hosts}=  Run  govc ls -t HostSystem host/cls
    @{hosts}=  Split To Lines  ${hosts}
    ${datastore}=  Get Name of First Local Storage For Host  @{hosts}[0]
    Set Environment Variable  TEST_DATASTORE  "${datastore}"

*** Test Cases ***
Test
    Log To Console  \nStarting test...

    Install VIC Appliance To Test Server  certs=${false}  vol=default
    Run Regression Tests
