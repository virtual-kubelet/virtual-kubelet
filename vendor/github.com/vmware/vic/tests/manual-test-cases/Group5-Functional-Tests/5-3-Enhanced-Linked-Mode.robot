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
Documentation  Test 5-3 - Enhanced Linked Mode
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Enhanced Link Mode Setup
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

Enhanced Link Mode Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${name}=  Evaluate  'els-' + str(random.randint(1000,9999))  modules=random
    Set Suite Variable  ${user}  %{NIMBUS_USER}
    Log To Console  \nDeploying Nimbus Testbed: ${name}

    ${pid}=  Run Secret SSHPASS command  %{NIMBUS_USER}  '%{NIMBUS_PASSWORD}'  'nimbus-testbeddeploy --lease=1 --noStatsDump --noSupportBundles --plugin test-vpx --testbedName test-vpx-m2n2-vcva-3esx-pxeBoot-8gbmem --vcvaBuild ${VC_VERSION} --esxPxeDir ${ESX_VERSION} --runName ${name}'

    &{esxes}=  Create Dictionary
    ${num_of_esxes}=  Evaluate  3
    :FOR  ${i}  IN RANGE  3
    # Deploy some ESXi instances
    \    &{new_esxes}=  Deploy Multiple Nimbus ESXi Servers in Parallel  ${num_of_esxes}  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    \    ${esxes}=  Combine Dictionaries  ${esxes}  ${new_esxes}

    # Investigate to see how many were actually deployed
    \    ${len}=  Get Length  ${esxes}
    \    ${num_of_esxes}=  Evaluate  3 - ${len}

    # Exit if we've got enough & continue loop if we don't
    \    Exit For Loop If  ${len} >= 3
    \    Log To Console  Only got ${len} ESXi instance(s); Trying again

    @{esx-names}=  Get Dictionary Keys  ${esxes}
    @{esx-ips}=  Get Dictionary Values  ${esxes}
    ${esx1}=  Get From List  ${esx-names}  0
    ${esx2}=  Get From List  ${esx-names}  1
    ${esx3}=  Get From List  ${esx-names}  2
    ${esx4-ip}=  Get From List  ${esx-ips}  0
    ${esx5-ip}=  Get From List  ${esx-ips}  1
    ${esx6-ip}=  Get From List  ${esx-ips}  2

    # Finish test bed deploy
    ${output}=  Wait For Process  ${pid}  timeout=70 minutes  on_timeout=terminate
    Log  ${output.stdout}
    Log  ${output.stderr}
    Should Be Equal As Integers  ${output.rc}  0
    ${output}=  Split To Lines  ${output.stdout}
    :FOR  ${line}  IN  @{output}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  ${name}.vc.0' is up. IP:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Suite Variable  ${vc1-ip}  ${ip}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  ${name}.vc.1' is up. IP:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Suite Variable  ${vc2-ip}  ${ip}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  ${name}.esx.0' is up. IP:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Suite Variable  ${esx1-ip}  ${ip}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  ${name}.esx.1' is up. IP:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Suite Variable  ${esx2-ip}  ${ip}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  ${name}.esx.2' is up. IP:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Suite Variable  ${esx3-ip}  ${ip}

    Set Suite Variable  @{list}  ${esx1}  ${esx2}  ${esx3}  ${user}-${name}.vc.0  ${user}-${name}.vc.1  ${user}-${name}.vc.2  ${user}-${name}.vc.3  ${user}-${name}.nfs.0  ${user}-${name}.esx.0  ${user}-${name}.esx.1  ${user}-${name}.esx.2

    Remove Environment Variable  GOVC_PASSWORD
    Remove Environment Variable  GOVC_USERNAME
    Set Environment Variable  GOVC_INSECURE  1
    :FOR  ${ip}  IN  ${esx1-ip}  ${esx2-ip}  ${esx3-ip}
    \   Log To Console  Changing password for ${ip}
    \   Set Environment Variable  GOVC_URL  root:@${ip}
    \   Wait Until Keyword Succeeds  10x  3 minutes  Change ESXi Server Password  e2eFunctionalTest
    \   ${license}=  Run  govc license.ls
    \   Check License Features

    Set Environment Variable  GOVC_URL  ${vc1-ip}
    Set Environment Variable  GOVC_USERNAME  administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin!23
    ${license}=  Run  govc license.ls

    # First VC cluster
    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create ha-datacenter
    Should Be Empty  ${out}

    Log To Console  Create a cluster on the VC
    ${out}=  Run  govc cluster.create cls
    Should Be Empty  ${out}

    Log To Console  Add ESX host to the VC
    :FOR  ${ip}  IN  ${esx1-ip}  ${esx2-ip}  ${esx3-ip}
    \    Log To Console  Adding ${ip} to VC
    \    ${out}=  Run  govc cluster.add -hostname=${ip} -username=root -dc=ha-datacenter -password=e2eFunctionalTest -noverify=true
    \    Should Contain  ${out}  OK

    Create A Distributed Switch  ha-datacenter

    Create Three Distributed Port Groups  ha-datacenter

    Add Host To Distributed Switch  /ha-datacenter/host/cls

    Log To Console  Enable DRS on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /ha-datacenter/host/cls
    Should Be Empty  ${out}

    # Second VC cluster
    Set Environment Variable  GOVC_URL  ${vc2-ip}

    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create ha-datacenter
    Should Be Empty  ${out}

    Log To Console  Create a cluster on the VC
    ${out}=  Run  govc cluster.create cls
    Should Be Empty  ${out}

    Log To Console  Add ESX host to the VC
    :FOR  ${ip}  IN  @{esx-ips}
    \    ${out}=  Run  govc cluster.add -hostname=${ip} -username=root -dc=ha-datacenter -password=e2eFunctionalTest -noverify=true
    \    Should Contain  ${out}  OK

    Create A Distributed Switch  ha-datacenter

    Create Three Distributed Port Groups  ha-datacenter

    Add Host To Distributed Switch  /ha-datacenter/host/cls

    Log To Console  Enable DRS on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /ha-datacenter/host/cls
    Should Be Empty  ${out}

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  GOVC_URL  ${vc1-ip}
    Set Environment Variable  TEST_URL_ARRAY  ${vc1-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Remove Environment Variable  TEST_DATACENTER
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  30m

*** Test Cases ***
Test
    Install VIC Appliance To Test Server
    Run Regression Tests
