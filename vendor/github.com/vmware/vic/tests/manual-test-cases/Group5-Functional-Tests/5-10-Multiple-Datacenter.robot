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
Documentation  Test 5-10 - Multiple Datacenters
Resource  ../../resources/Util.robot
Suite Setup  Nimbus Suite Setup  Multiple Datacenter Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
# Insert elements from dict2 into dict1, overwriting conflicts in dict1 & returning new dict
Combine Dictionaries
    [Arguments]  ${dict1}  ${dict2}
    ${dict2keys}=  Get Dictionary Keys  ${dict2}
    :FOR  ${key}  IN  @{dict2keys}
    \    ${elem}=  Get From Dictionary  ${dict2}  ${key}
    \    Set To Dictionary  ${dict1}  ${key}  ${elem}
    [Return]  ${dict1}

Multiple Datacenter Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    &{esxes}=  Create Dictionary
    ${num_of_esxes}=  Evaluate  2
    :FOR  ${i}  IN RANGE  3
    # Deploy some ESXi instances
    \    &{new_esxes}=  Deploy Multiple Nimbus ESXi Servers in Parallel  ${num_of_esxes}  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    \    ${esxes}=  Combine Dictionaries  ${esxes}  ${new_esxes}

    # Investigate to see how many were actually deployed
    \    ${len}=  Get Length  ${esxes}
    \    ${num_of_esxes}=  Evaluate  ${num_of_esxes} - ${len}

    # Exit if we've got enough & continue loop if we don't
    \    Exit For Loop If  ${len} >= 2
    \    Log To Console  Only got ${len} ESXi instance(s); Trying again

    @{esx-names}=  Get Dictionary Keys  ${esxes}
    @{esx-ips}=  Get Dictionary Values  ${esxes}
    ${esx1}=  Get From List  ${esx-names}  0
    ${esx2}=  Get From List  ${esx-names}  1
    ${esx1-ip}=  Get From List  ${esx-ips}  0
    ${esx2-ip}=  Get From List  ${esx-ips}  1

    ${esx3}  ${esx4}  ${esx5}  ${vc}  ${esx3-ip}  ${esx4-ip}  ${esx5-ip}  ${vc-ip}=  Create a Simple VC Cluster  datacenter1  cls1
    Set Suite Variable  @{list}  ${esx1}  ${esx2}  ${esx3}  ${esx4}  ${esx5}  %{NIMBUS_USER}-${vc}

    Log To Console  Create datacenter2 on the VC
    ${out}=  Run  govc datacenter.create datacenter2
    Should Be Empty  ${out}
    ${out}=  Run  govc host.add -hostname=${esx1-ip} -username=root -dc=datacenter2 -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK

    Log To Console  Create datacenter3 on the VC
    ${out}=  Run  govc datacenter.create datacenter3
    Should Be Empty  ${out}
    ${out}=  Run  govc host.add -hostname=${esx2-ip} -username=root -dc=datacenter3 -password=e2eFunctionalTest -noverify=true
    Should Contain  ${out}  OK

    Set Environment Variable  TEST_DATACENTER  /datacenter1
    Set Environment Variable  GOVC_DATACENTER  /datacenter1

*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Install VIC Appliance To Test Server  certs=${false}  vol=default
    Run Regression Tests
    Remove Environment Variable  TEST_DATACENTER
    Remove Environment Variable  GOVC_DATACENTER
