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
Documentation  Test 5-7 - NSX
Resource  ../../resources/Util.robot
Force Tags  nsx

*** Keywords ***
NSX Install VIC Appliance To Test Server
    [Arguments]  ${vic-machine}=bin/vic-machine-linux  ${appliance-iso}=bin/appliance.iso  ${bootstrap-iso}=bin/bootstrap.iso  ${certs}=${true}  ${vol}=default  ${cleanup}=${true}  ${debug}=1  ${additional-args}=${EMPTY}
    Set Test Environment Variables
    # disable firewall
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.esxcli network firewall set -e false
    # Attempt to cleanup old/canceled tests
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Networks On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling vSwitches On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Containers On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Resource Pools On Test Server

    # Install the VCH now
    Log To Console  \nInstalling VCH to test server...
    ${output}=  NSX Run VIC Machine Command  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${debug}  ${additional-args}
    Log  ${output}
    Should Contain  ${output}  Installer completed successfully

    Get Docker Params  ${output}  ${certs}
    Log To Console  Installer completed successfully: %{VCH-NAME}...

    [Return]  ${output}

NSX Run VIC Machine Command
    [Tags]  secret
    [Arguments]  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${debug}  ${additional-args}
    ${output}=  Run Keyword If  ${certs}  Run  ${vic-machine} create --debug ${debug} --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=${appliance-iso} --bootstrap-iso=${bootstrap-iso} --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --insecure-registry harbor.ci.drone.local --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:${vol} ${vicmachinetls} ${additional-args}
    Run Keyword If  ${certs}  Log  ${output}
    Run Keyword If  ${certs}  Should Contain  ${output}  Installer completed successfully
    Return From Keyword If  ${certs}  ${output}

    ${output}=  Run Keyword Unless  ${certs}  Run  ${vic-machine} create --debug ${debug} --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=${appliance-iso} --bootstrap-iso=${bootstrap-iso} --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --insecure-registry harbor.ci.drone.local --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:${vol} --no-tlsverify ${additional-args}
    Run Keyword Unless  ${certs}  Log  ${output}
    Run Keyword Unless  ${certs}  Should Contain  ${output}  Installer completed successfully
    [Return]  ${output}


*** Test Cases ***
Test
    Log To Console  \nWait until Nimbus is at least available...
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS2_USER}  %{NIMBUS2_PASSWORD}
    ${out}=  Execute Command  nimbus-ctl --lease 5 extend_lease '*'
    Log  ${out}
    Close Connection

    Log To Console  Set environment variables up for GOVC
    Set Environment Variable  GOVC_URL  10.160.21.102
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  TEST_URL_ARRAY  10.160.21.102
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  vxw-dvs-53-virtualwire-1-sid-5001-nsx-switch
    Set Environment Variable  PUBLIC_NETWORK  'VM Network'
    Remove Environment Variable  TEST_DATACENTER
    Set Environment Variable  TEST_DATASTORE  vsanDatastore
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  30m

    NSX Install VIC Appliance To Test Server  certs=${false}  vol=default

    Run Regression Tests

    Cleanup VIC Appliance On Test Server
