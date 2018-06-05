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
Documentation  Test 5-26 - Static IP Address
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Static IP Address Create
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
Static IP Address Create
    [Timeout]    110 minutes
    Log To Console  Starting Static IP Address test...
    Set Suite Variable  ${NIMBUS_LOCATION}  NIMBUS_LOCATION=wdc
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${name}=  Evaluate  'vic-5-26-' + str(random.randint(1000,9999))  modules=random
    ${out}=  Deploy Nimbus Testbed  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  --noSupportBundles --plugin testng --vcvaBuild ${VC_VERSION} --esxBuild ${ESX_VERSION} --testbedName vic-simple-cluster --testbedSpecRubyFile /dbc/pa-dbc1111/mhagen/nimbus-testbeds/testbeds/vic-simple-cluster.rb --runName ${name}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc-ip}=  Get IP  ${name}.vc.0
    ${pod}=  Fetch POD  ${name}.vc.0
    Set Suite Variable  ${NIMBUS_POD}  ${pod}
    Close Connection

    Set Suite Variable  @{list}  %{NIMBUS_USER}-${name}.esx.0  %{NIMBUS_USER}-${name}.esx.1  %{NIMBUS_USER}-${name}.esx.2  %{NIMBUS_USER}-${name}.nfs.0  %{NIMBUS_USER}-${name}.vc.0
    Log To Console  Finished Creating Cluster ${name}

    ${out}=  Get Static IP Address
    Set Suite Variable  ${static}  ${out}
    Append To List  ${list}  %{STATIC_WORKER_NAME}
    
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
    Set Environment Variable  TEST_DATASTORE  nfs0-1
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  15m

*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Custom Testbed Keepalive  /dbc/pa-dbc1111/mhagen

    Install VIC Appliance To Test Server  additional-args=--public-network-ip &{static}[ip]/&{static}[netmask] --public-network-gateway &{static}[gateway] --dns-server 10.170.16.48
    Run Regression Tests