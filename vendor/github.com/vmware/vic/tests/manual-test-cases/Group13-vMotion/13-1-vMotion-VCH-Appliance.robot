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
Documentation  Test 13-1 - vMotion VCH Appliance
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Create a VSAN Cluster  vic-vmotion-13-1
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}
Test Teardown  Run Keyword If Test Failed  Gather All vSphere Logs

*** Keywords ***
Gather All vSphere Logs
    ${hostList}=  Run  govc ls -t HostSystem host/cls | xargs
    Run  govc logs.download ${hostList}

*** Test Cases ***
#Step 1-5
#    ${status}=  Get State Of Github Issue  701
#    Run Keyword If  '${status}' == 'closed'  Fail  Test 13-1-vMotion-VCH-Appliance.robot needs to be updated now that Issue #701 has been resolved
#    Log  Issue \#701 is blocking implementation  WARN
#    Install VIC Appliance To Test Server
#    Run Regression Tests
#    ${host}=  Get VM Host Name  %{VCH-NAME}
#    Power Off VM OOB  %{VCH-NAME}
#    ${status}=  Run Keyword And Return Status  Should Contain  ${host}  ${esx1-ip}
#    Run Keyword If  ${status}  Run  govc vm.migrate -host cls/${esx2-ip} -pool cls/Resources %{VCH-NAME}
#    Run Keyword Unless  ${status}  Run  govc vm.migrate -host cls/${esx1-ip} -pool cls/Resources %{VCH-NAME}
#    Set Environment Variable  VCH-NAME  "%{VCH-NAME} (1)"
#    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.power -on %{VCH-NAME}
#    Should Be Equal As Integers  ${rc}  0

#    Log To Console  Waiting for VM to power on ...
#    :FOR  ${idx}  IN RANGE  0  30
#    \   ${ret}=  Run  govc vm.info %{VCH-NAME}
#    \   ${status}=  Run Keyword And Return Status  Should Contain  ${ret}  poweredOn
#    \   Exit For Loop If  ${status}
#    \   Sleep  1

#    Log To Console  Getting VCH IP ...
#    ${new-vch-ip}=  Get VM IP  %{VCH-NAME}
#    Log To Console  New VCH IP is ${new-vch-ip}
#    Replace String  %{VCH-PARAMS}  %{VCH-IP}  ${new-vch-ip}

#    Wait Until Keyword Succeeds  20x  5 seconds  Run Docker Info  %{VCH-PARAMS}

#    Run Regression Tests
    #TODO
    #This does not work currently, as the VM has been migrated out of the vApp

Step 6-9
    Set Test Variable  ${user}  %{NIMBUS_USER}
    Set Suite Variable  @{list}  ${user}-vic-vmotion-13-1.vcva-${VC_VERSION}  ${user}-vic-vmotion-13-1.esx.0  ${user}-vic-vmotion-13-1.esx.1  ${user}-vic-vmotion-13-1.esx.2  ${user}-vic-vmotion-13-1.esx.3  ${user}-vic-vmotion-13-1.nfs.0  ${user}-vic-vmotion-13-1.iscsi.0
    Install VIC Appliance To Test Server
    Run Regression Tests
    ${host}=  Get VM Host Name  %{VCH-NAME}
    ${status}=  Run Keyword And Return Status  Should Contain  ${host}  ${esx1-ip}
    Run Keyword If  ${status}  Run  govc vm.migrate -host cls/${esx2-ip} %{VCH-NAME}
    Run Keyword Unless  ${status}  Run  govc vm.migrate -host cls/${esx1-ip} %{VCH-NAME}
    Run Regression Tests

Step 10-13
    Install VIC Appliance To Test Server
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create busybox /bin/top
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${container2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create busybox /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container2}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${container3}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create busybox ls
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container3}
    Should Be Equal As Integers  ${rc}  0

    ${host}=  Get VM Host Name  %{VCH-NAME}
    ${status}=  Run Keyword And Return Status  Should Contain  ${host}  ${esx1-ip}
    Run Keyword If  ${status}  Run  govc vm.migrate -host cls/${esx2-ip} %{VCH-NAME}
    Run Keyword Unless  ${status}  Run  govc vm.migrate -host cls/${esx1-ip} %{VCH-NAME}
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container1}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container1}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container1}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container2}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container2}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${container3}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container3}
    Should Be Equal As Integers  ${rc}  0
