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
Documentation  Test 6-03 - Verify delete clean up all resources
Resource  ../../resources/Util.robot
Test Setup  Install VIC Appliance To Test Server
Test Teardown  Run Keyword If Test Failed  Cleanup Delete Tests
Test Timeout  20 minutes

*** Keywords ***
Initial load
    # Create container VM first
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${name}=  Generate Random String  15
    ${rc}  ${container-id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${container-id}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container-id}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error:
    Set Suite Variable  ${containerName}  ${name}

Cleanup Delete Tests
    Cleanup VIC Appliance On Test Server

    ${rc}  ${output}=  Run Keyword If  '${tempvm}'!='${EMPTY}'  Run And Return Rc And Output  govc vm.destroy ${tempvm}
    Run Keyword If  '${tempvm}'!='${EMPTY}'  Log  ${output}
    Run Keyword If  '${tempvm}'!='${EMPTY}'  Should Be Equal As Integers  ${rc}  0

Cleanup Delete VCH with non-cVM in VCH folder test
    Run Keyword And Ignore Error  Cleanup VIC Appliance On Test Server

    Set Environment Variable  TEST_DATACENTER  ${orig-dc}

    # Delete VCH folder and the non-cVM under it
    ${output}=  Run  govc object.destroy %{VCH-NAME}
    Log  ${output}

    # Delete resource pool
    ${output}=  Run  govc pool.destroy "%{TEST_RESOURCE}/Resources/%{VCH-NAME}"
    Log  ${output}
    ${output}=  Run  govc pool.destroy "%{TEST_RESOURCE}/%{VCH-NAME}"
    Log  ${output}

*** Test Cases ***
Delete VCH and verify
    Initial load
    # Get VCH uuid and container VM uuid, to check if resources are removed correctly
    Run Keyword And Ignore Error  Gather Logs From Test Server
    ${uuid}=  Run  govc vm.info -json\=true %{VCH-NAME} | jq -r '.VirtualMachines[0].Config.Uuid'
    ${ret}=  Run  bin/vic-machine-linux delete --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME}
    Should Contain  ${ret}  is powered on

    # Delete with force
    ${ret}=  Run  bin/vic-machine-linux delete --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --force
    Should Contain  ${ret}  Completed successfully
    Should Not Contain  ${ret}  Operation failed: Error caused by file
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network

    # Check VM is removed
    ${ret}=  Run  govc vm.info -json=true ${containerName}-*
    Should Contain  ${ret}  {"VirtualMachines":null}
    ${ret}=  Run  govc vm.info -json=true %{VCH-NAME}
    Should Contain  ${ret}  {"VirtualMachines":null}

    # Check that the VCH folder is removed
    ${ret}=  Run  govc find %{TEST_DATACENTER}/vm -type f
    Should Not Contain  ${ret}  %{VCH-NAME}

    # Check directories are removed
    ${ret}=  Run  govc datastore.ls VIC/${uuid}
    Should Contain  ${ret}   was not found
    ${ret}=  Run  govc datastore.ls %{VCH-NAME}
    Should Contain  ${ret}   was not found
    ${ret}=  Run  govc datastore.ls VIC/${containerName}-*
    Should Contain  ${ret}   was not found

    # Check resource pool is removed
    ${ret}=  Run  govc pool.info -json=true host/*/Resources/%{VCH-NAME}
    Should Contain  ${ret}  {"ResourcePools":null}


Attach Disks and Delete VCH
    # VCH should delete normally during commit/pull/cp/push operations
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${ubuntu}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  govc datastore.ls %{VCH-NAME}/VIC/
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # iterate through found images and attach them to the appliance VM
    ${vm-ip}=  Run  govc vm.ip %{VCH-NAME}
    ${imagedir}=  Run  govc datastore.ls %{VCH-NAME}/VIC/
    ${images}=  Run  govc datastore.ls %{VCH-NAME}/VIC/${imagedir}/images/ | tr '${\n}' ' '
    ${rc}  ${output}=  Run And Return Rc And Output  (set -e; for x in ${images}; do echo $x; govc vm.disk.attach -disk=%{VCH-NAME}/VIC/${imagedir}/images/$x/$x.vmdk -vm.ip=${vm-ip}; done)
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux delete --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Completed successfully
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network

    ${rc}=  Run And Return Rc  govc datastore.ls -dc=%{TEST_DATACENTER} %{VCH-NAME}/VIC/
    Should Be Equal As Integers  ${rc}  1


Delete VCH with non-cVM in same RP
    # Don't perform unconditional setup as we skip the test on a non-DRS env
    [Setup]     NONE

    ${output}=  Query Cluster DRS Setting  %{TEST_RESOURCE}
    Run Keyword If  '${output}' == 'false'  Pass Execution  Test not applicable on non-DRS env - no resource pools

    Install VIC Appliance To Test Server

    ${rand}=  Generate Random String  15
    ${dummyvm}=  Set Variable  anothervm-${rand}
    Set Suite Variable  ${tempvm}  ${dummyvm}

    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Set Test Variable  ${pool}  "%{TEST_RESOURCE}/%{VCH-NAME}"
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Set Test Variable  ${pool}  "%{TEST_RESOURCE}/Resources/%{VCH-NAME}"

    Log To Console  Create VM ${dummyvm} in ${pool} net %{PUBLIC_NETWORK}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.create -pool=${pool} -net=%{PUBLIC_NETWORK} -on=false ${dummyvm}
    Should Be Equal As Integers  ${rc}  0

    # Verify VM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Delete with force
    ${ret}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux delete --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --force
    Log  ${output}
    Should Contain  ${output}  Completed successfully

    # Verify VM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Delete VM and RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.destroy ${dummyvm}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Run Keyword And Continue On Failure  Run  govc pool.destroy "%{TEST_RESOURCE}/Resources/%{VCH-NAME}"
    ${rc}  ${output}=  Run And Return Rc And Output  govc pool.destroy "%{TEST_RESOURCE}/%{VCH-NAME}"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network


Delete VCH moved from its RP
    # Don't perform unconditional setup as we skip the test on ESX or a non-DRS env
    [Setup]     NONE

    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Pass Execution  Test skipped on ESX due to unable to move into RP
    ${output}=  Query Cluster DRS Setting  %{TEST_RESOURCE}
    Run Keyword If  '${output}' == 'false'  Pass Execution  Test not applicable on non-DRS env - no resource pools

    Install VIC Appliance To Test Server

    Set Test Variable  ${test-resource}  "%{TEST_RESOURCE}/Resources"

    ${rand}=  Generate Random String  15
    ${dummyvm}=  Set Variable  anothervm-${rand}
    ${dummyRP}=  Set Variable  rp-${rand}

    Set Suite Variable  ${tempvm}  ${dummyvm}
    Log To Console  Create VM ${dummyvm} in ${test-resource}/%{VCH-NAME} net %{PUBLIC_NETWORK}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.create -pool=${test-resource}/%{VCH-NAME} -net=%{PUBLIC_NETWORK} -on=false ${dummyvm}
    Should Be Equal As Integers  ${rc}  0

    # Verify VM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Create temp RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc pool.create "${test-resource}/${dummyRP}"
    Should Be Equal As Integers  ${rc}  0

    # Move VCH to temp RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.migrate -pool "${test-resource}/${dummyRP}" %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

    # Delete with force
    ${moid}=  Get VM Moid  %{VCH-NAME}
    ${ret}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux delete --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --id ${moid} --force
    Log  ${output}
    Should Contain  ${output}  Completed successfully

    # Verify VM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Delete VM and RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.destroy ${dummyvm}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  govc pool.destroy "${test-resource}/%{VCH-NAME}"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  govc pool.destroy "${dummyRP}"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network


Delete VCH moved to root RP and original RP deleted
    # Don't perform unconditional setup as we skip the test on ESX or a non-DRS env
    [Setup]     NONE

    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Pass Execution  Test skipped on ESX due to unable to move into RP
    ${output}=  Query Cluster DRS Setting  %{TEST_RESOURCE}
    Run Keyword If  '${output}' == 'false'  Pass Execution  Test not applicable on non-DRS env - no resource pools

    Install VIC Appliance To Test Server

    ${rand}=  Generate Random String  15
    ${dummyvm}=  Set Variable  anothervm-${rand}
    Set Suite Variable  ${tempvm}  ${dummyvm}
    Log To Console  Create VM ${dummyvm} in %{TEST_RESOURCE}/%{VCH-NAME} net %{PUBLIC_NETWORK}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.create -pool="%{TEST_RESOURCE}" -net=%{PUBLIC_NETWORK} -on=false ${dummyvm}
    Should Be Equal As Integers  ${rc}  0

    # Verify VM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Move VCH to root RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.migrate -pool %{TEST_RESOURCE} %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

    # Delete with force
    ${moid}=  Get VM Moid  %{VCH-NAME}
    ${ret}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux delete --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --id ${moid} --force
    Log  ${output}
    Should Contain  ${output}  Completed successfully

    # Verify VM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Delete VM and RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.destroy ${dummyvm}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network


Delete VCH with non-cVM in VCH folder
    # Don't perform unconditional setup as we skip the test on ESX
    [Setup]     NONE
    # Custom teardown step for this test to clean up VCH folder and resource pool
    [Teardown]  Cleanup Delete VCH with non-cVM in VCH folder test

    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Pass Execution  Not applicable on ESX - no VCH folder

    Install VIC Appliance To Test Server

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${con}=  Evaluate  'cvm-' + str(random.randint(1000,9999))  modules=random
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${con} ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${dc}=  Get Environment Variable  TEST_DATACENTER
    Set Suite Variable  ${orig-dc}  ${dc}
    ${dc}=  Run Keyword If  '%{TEST_DATACENTER}' == '${SPACE}'  Get Datacenter Name
    Run Keyword If  '%{TEST_DATACENTER}' == '${SPACE}'  Set Environment Variable  TEST_DATACENTER  /${dc}

    # Create a non-cVM in the VCH's parent folder
    ${rand}=  Generate Random String  15
    ${dummyvm}=  Set Variable  anothervm-${rand}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.create -net=%{PUBLIC_NETWORK} -folder=%{TEST_DATACENTER}/vm/%{VCH-NAME} -on=false ${dummyvm}
    Should Be Equal As Integers  ${rc}  0

    # Verify non-cVM exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/%{VCH-NAME}/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

    # Delete VCH with force
    ${ret}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux delete --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --force
    Log  ${output}
    Should Contain  ${output}  Completed successfully

    # Verify VCH folder exists
    ${ret}=  Run  govc find %{TEST_DATACENTER}/vm -type f
    Should Contain  ${ret}  %{VCH-NAME}

    # Verify cVM does not exist
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/%{VCH-NAME}/${con}
    Log  ${output}
    Should Not Contain  ${output}  ${con}

    # Verify non-cVM still exists
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm/%{VCH-NAME}/${dummyvm}
    Log  ${output}
    Should Contain  ${output}  ${dummyvm}

