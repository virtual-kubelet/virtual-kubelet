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
Documentation  Test 6-10 - Verify ls list all VCHs
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Verify Listed Machines
    [Arguments]  ${list}
    Should Contain  ${list}  ID
    Should Contain  ${list}  PATH
    Should Contain  ${list}  NAME
    Should Not Contain  ${list}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux version
    Should Be Equal As Integers  ${rc}  0
    @{version}=  Split String  ${output}
    ${machines}=  Get Lines Containing String  ${list}  %{VCH-NAME}
    @{lines}=  Split To Lines  ${machines}
    Length Should Be  ${lines}  1
    # Get VCH ID, PATH and NAME
    @{vch}=  Split String  @{lines}[-1]
    ${vch-id}=  Strip String  @{vch}[0]
    ${vch-path}=  Strip String  @{vch}[1]
    ${vch-name}=  Strip String  @{vch}[2]
    ${vch-version}=  Strip String  @{vch}[3]
    Should Be Equal As Strings  @{version}[-1]  ${vch-version}
    # Run vic-machine inspect
    ${ret}=  Run  bin/vic-machine-linux inspect --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --id ${vch-id}
    Should Contain  ${ret}  Completed successfully
    ${ret}=  Run  bin/vic-machine-linux inspect --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource ${vch-path} --name ${vch-name}
    Should Contain  ${ret}  Completed successfully

*** Test Cases ***
List all VCHs
    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD}
    Verify Listed Machines  ${ret}

List with compute-resource
    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource %{TEST_RESOURCE}
    Verify Listed Machines  ${ret}

List with trailing slash
    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL}/ --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource %{TEST_RESOURCE}
    Verify Listed Machines  ${ret}

List suggest compute resource
    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource fakeComputeResource
    Should Contain  ${ret}  Suggested values for --compute-resource:

List suggest valid datacenter
    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL}/fakeDatacenter --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource %{TEST_RESOURCE}
    Should Contain  ${ret}  Suggesting valid values for datacenter in --target

List with valid datacenter
    ${orig}=  Get Environment Variable  TEST_DATACENTER
    ${dc}=  Run Keyword If  '%{TEST_DATACENTER}' == '${SPACE}'  Get Datacenter Name
    Run Keyword If  '%{TEST_DATACENTER}' == '${SPACE}'  Set Environment Variable  TEST_DATACENTER  ${dc}

    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL}/%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD}
    Set Environment Variable  TEST_DATACENTER  ${orig}
    Verify Listed Machines  ${ret}
