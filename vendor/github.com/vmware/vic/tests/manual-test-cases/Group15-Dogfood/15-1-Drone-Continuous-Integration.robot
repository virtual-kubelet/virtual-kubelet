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
Documentation  Test 15-1 - Drone Continuous Integration
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server Without TLS
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Install VIC Appliance To Test Server Without TLS
    [Arguments]  ${vic-machine}=bin/vic-machine-linux  ${appliance-iso}=bin/appliance.iso  ${bootstrap-iso}=bin/bootstrap.iso  ${vol}=default
    Set Test Environment Variables
    # disable firewall
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.esxcli network firewall set -e false
    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    Run Keyword And Ignore Error  Cleanup Dangling Networks On Test Server
    Run Keyword And Ignore Error  Cleanup Dangling vSwitches On Test Server

    # Install the VCH now
    Log To Console  \nInstalling VCH to test server...
    ${output}=  No TLS VIC Install  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${vol}
    Log  ${output}
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${false}
    Log To Console  Installer completed successfully: %{VCH-NAME}...

No TLS VIC Install
    [Tags]  secret
    [Arguments]  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${vol}
    ${output}=  Run  ${vic-machine} create --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=${appliance-iso} --bootstrap-iso=${bootstrap-iso} --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:${vol} --no-tls
    Should Contain  ${output}  Installer completed successfully
    [Return]  ${output}

*** Test Cases ***
Drone CI
    ${output}=  Run  git clone https://github.com/vmware/vic.git drone-ci
    Log  ${output}
    ${result}=  Run Process  drone exec --docker-host %{VCH-IP}:2375 --trusted -e .drone.sec -yaml .drone.yml  shell=True  cwd=drone-ci
    Log  ${result.stderr}
    Log  ${result.stdout}
    Log  ${result.rc}