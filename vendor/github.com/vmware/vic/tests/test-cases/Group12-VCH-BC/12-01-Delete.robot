# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 12-01 - Delete
Resource  ../../resources/Util.robot
Suite Setup  Install VIC 1.1.1 to Test Server
Test Teardown  Run Keyword If Test Failed  Clean up VIC Appliance And Local Binary

*** Keywords ***
Clean up VIC Appliance And Local Binary
    Cleanup VIC Appliance On Test Server
    Run  rm -rf vic.tar.gz vic

Install VIC 1.1.1 to Test Server
    Log To Console  \nDownloading VIC 1.1.1 from gcp...
    ${rc}  ${output}=  Run And Return Rc And Output  wget https://storage.googleapis.com/vic-engine-releases/vic_1.1.1.tar.gz -O vic.tar.gz
    ${rc}  ${output}=  Run And Return Rc And Output  tar zxvf vic.tar.gz
    Set Test Environment Variables

    Log To Console  \nInstalling VCH to test server...
    ${output}=  Run  ./vic/vic-machine-linux create --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=./vic/appliance.iso --bootstrap-iso=./vic/bootstrap.iso --password=%{TEST_PASSWORD} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --force=true --no-tlsverify
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  false
    Log To Console  Installer completed successfully: %{VCH-NAME}

*** Test Cases ***
Delete VCH with new vic-machine
    Log To Console  \nRunning docker pull busybox...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${name}=  Generate Random String  15
    ${rc}  ${container-id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} busybox /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${container-id}  Error
    Set Suite Variable  ${containerName}  ${name}

    # Get VCH uuid and container VM uuid, to check if resources are removed correctly
    Run Keyword And Ignore Error  Gather Logs From Test Server
    ${uuid}=  Run  govc vm.info -json\=true %{VCH-NAME} | jq -r '.VirtualMachines[0].Config.Uuid'
    ${ret}=  Run  bin/vic-machine-linux delete --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME}
    Should Contain  ${ret}  is different than installer version

    # Delete with force
    Run VIC Machine Delete Command

    # Check VM is removed
    ${ret}=  Run  govc vm.info -json=true ${containerName}-*
    Should Contain  ${ret}  {"VirtualMachines":null}
    ${ret}=  Run  govc vm.info -json=true %{VCH-NAME}
    Should Contain  ${ret}  {"VirtualMachines":null}

    # Check resource pool is removed
    ${ret}=  Run  govc pool.info -json=true host/*/Resources/%{VCH-NAME}
    Should Contain  ${ret}  {"ResourcePools":null}
    Run  rm -rf vic.tar.gz vic
