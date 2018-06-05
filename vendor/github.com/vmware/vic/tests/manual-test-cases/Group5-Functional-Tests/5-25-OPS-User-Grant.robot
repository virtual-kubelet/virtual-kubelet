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
Documentation  Test 5-25 - OPS-User-Grant
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Ops User Create
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}
Test Teardown  Run Keyword If Test Failed  Gather VC Logs

*** Keywords ***

Gather VC Logs
    Log To Console  Collecting VC logs ..
    Run Keyword And Ignore Error  Gather Logs From ESX Server
    Log To Console  VC logs collected

Ops User Create
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    Set Suite Variable  ${datacenter}  datacenter1
    Set Suite Variable  ${cluster}  cls1
    ${esx1}  ${esx2}  ${esx3}  ${vc}  ${esx1-ip}  ${esx2-ip}  ${esx3-ip}  ${vc-ip}=  Create a Simple VC Cluster  ${datacenter}  ${cluster}
    Log To Console  Finished Creating Cluster ${vc}
    Set Suite Variable  @{list}  ${esx1}  ${esx2}  ${esx3}  %{NIMBUS_USER}-${vc}
    ${vc}=  Set Variable  vcname

    Set Suite Variable  ${ops_user_base_name}  vch-user
    Set Suite Variable  ${ops_user_domain}  vsphere.local
    ${ops_user_name}=  Catenate  SEPARATOR=@  ${ops_user_base_name}  ${ops_user_domain}
    Log To Console  Base User Name: ${ops_user_base_name}
    Log To Console  Full User Name: ${ops_user_name}

    Set Suite Variable  ${ops_user_name}
    Set Suite Variable  ${ops_user_password}  Admin!23
    Set Suite Variable  ${vc_admin_password}  Admin!23

    Log To Console  Setting up ops-user: ${ops_user_name}
    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -p vmware ssh -o StrictHostKeyChecking=no root@${vc-ip} /usr/lib/vmware-vmafd/bin/dir-cli user create --account ${ops_user_base_name} --user-password ${ops_user_password} --first-name ${ops_user_base_name} --last-name ${ops_user_domain} --password ${vc_admin_password}
    Log  User Create ${ops_user_name}, rc: ${rc}, output: ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -p vmware ssh -o StrictHostKeyChecking=no root@${vc-ip} /usr/lib/vmware-vmafd/bin/dir-cli user find-by-name --account ${ops_user_base_name} --password ${vc_admin_password}
    Log  User Find ${ops_user_base_name}, rc: ${rc}, output: ${output}
    Should Be Equal As Integers  ${rc}  0

    ${out}=  Run  govc role.usage
    Log  Output, govc role.usage: ${out}

*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Install VIC Appliance To Test Server  additional-args=--ops-user ${ops_user_name} --ops-password ${ops_user_password} --ops-grant-perms

    # Run a govc test to check that access is denied on some resources
    Log To Console  Running govc to set drs-enabled, it should fail
    ${rc}  ${output}=  Run And Return Rc And Output  GOVC_USERNAME=${ops_user_name} GOVC_PASSWORD=${ops_user_password} govc cluster.change -drs-enabled /${datacenter}/host/${cluster}
    Log  Govc output: ${output}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Permission to perform this operation was denied


    Run Regression Tests

    Cleanup VIC Appliance On Test Server
