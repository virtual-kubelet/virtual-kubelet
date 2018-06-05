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
Documentation  This resource contains any keywords dealing with operations being performed on a Vsphere instance, mostly govc wrappers

*** Keywords ***
Power On VM OOB
    [Arguments]  ${vm}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.power -on "${vm}"
    Should Be Equal As Integers  ${rc}  0
    Log To Console  Waiting for VM to power on ...
    Wait Until VM Powers On  ${vm}

Power Off VM OOB
    [Arguments]  ${vm}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.power -off "${vm}"
    Should Be Equal As Integers  ${rc}  0
    Log To Console  Waiting for VM to power off ...
    Wait Until VM Powers Off  "${vm}"

Destroy VM OOB
    [Arguments]  ${vm}
    ${rc}  ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run And Return Rc And Output  govc object.method -name Destroy_Task -enable %{TEST_DATACENTER}/vm/${vm}
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.destroy "${vm}"
    Should Be Equal As Integers  ${rc}  0

Put Host Into Maintenance Mode
    ${rc}  ${output}=  Run And Return Rc And Output  govc host.maintenance.enter -host.ip=%{TEST_URL}
    Should Contain  ${output}  entering maintenance mode... OK

Remove Host From Maintenance Mode
    ${rc}  ${output}=  Run And Return Rc And Output  govc host.maintenance.exit -host.ip=%{TEST_URL}
    Should Contain  ${output}  exiting maintenance mode... OK

Reboot VM
    [Arguments]  ${vm}
    Log To Console  Rebooting ${vm} ...
    Power Off VM OOB  ${vm}
    Power On VM OOB  ${vm}
    Log To Console  ${vm} Powered On

Wait Until VM Powers On
    [Arguments]  ${vm}
    :FOR  ${idx}  IN RANGE  0  30
    \   ${ret}=  Run  govc vm.info ${vm}
    \   Set Test Variable  ${out}  ${ret}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  poweredOn
    \   Return From Keyword If  ${status}
    \   Sleep  1
    Fail  VM did not power on within 30 seconds

Wait Until VM Powers Off
    [Arguments]  ${vm}
    :FOR  ${idx}  IN RANGE  0  30
    \   ${ret}=  Run  govc vm.info ${vm}
    \   Set Test Variable  ${out}  ${ret}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  poweredOff
    \   Return From Keyword If  ${status}
    \   Sleep  1
    Fail  VM did not power off within 30 seconds

Wait Until VM Is Destroyed
    [Arguments]  ${vm}
    :FOR  ${idx}  IN RANGE  0  30
    \   ${ret}=  Run  govc ls vm/${vm}
    \   Set Test Variable  ${out}  ${ret}
    \   ${status}=  Run Keyword And Return Status  Should Be Empty  ${out}
    \   Return From Keyword If  ${status}
    \   Sleep  1
    Fail  VM was not destroyed within 30 seconds

Get VM IP
    [Arguments]  ${vm}
    ${rc}  ${out}=  Run And Return Rc And Output  govc vm.ip ${vm}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${out}

Get VM Host Name
    [Arguments]  ${vm}
    ${rc}  ${out}=  Run And Return Rc And Output  govc vm.info ${vm}
    Should Be Equal As Integers  ${rc}  0
    ${out}=  Split To Lines  ${out}
    ${host}=  Fetch From Right  @{out}[-1]  ${SPACE}
    [Return]  ${host}

Get VM Info
    [Arguments]  ${vm}
    ${rc}  ${out}=  Run And Return Rc And Output  govc vm.info -r ${vm}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${out}

Get VM Moid
    [Arguments]  ${vm}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -dump -json ${vm} | jq -c '.VirtualMachines[] | .Self.Value'
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${output}

Check ImageStore
    ${rc}  ${output}=  Run And Return Rc And Output  govc datastore.ls -R -ds=%{TEST_DATASTORE} %{VCH-NAME}/VIC
    Should Be Equal As Integers  ${rc}  0
    Log  ${output}

vMotion A VM
    [Arguments]  ${vm}
    ${host}=  Get VM Host Name  ${vm}
    ${status}=  Run Keyword And Return Status  Should Contain  ${host}  ${esx1-ip}
    Run Keyword If  ${status}  Run  govc vm.migrate -host cls/${esx2-ip} -pool cls/Resources ${vm}
    Run Keyword Unless  ${status}  Run  govc vm.migrate -host cls/${esx1-ip} -pool cls/Resources ${vm}

vMotion A VM And Wait
    [Arguments]  ${vm}  ${attempts}  ${interval}
    ${host}=  Get VM Host Name  ${vm}
    ${status}=  Run Keyword And Return Status  Should Contain  ${host}  ${esx1-ip}
    Run Keyword If  ${status}  Run  govc vm.migrate -host cls/${esx2-ip} -pool cls/Resources ${vm}
    Run Keyword Unless  ${status}  Run  govc vm.migrate -host cls/${esx1-ip} -pool cls/Resources ${vm}
    Wait Until Keyword Succeeds  ${attempts}  ${interval}  VM Host Has Changed  ${host}  ${vm}

VM Host Has Changed
    [Arguments]  ${oldHost}  ${vm}
    ${curHost}=  Get VM Host Name  ${vm}
    Should Not Be Equal  ${oldHost}  ${curHost}

Create Test Server Snapshot
    [Arguments]  ${vm}  ${snapshot}
    Set Environment Variable  GOVC_URL  %{BUILD_SERVER}
    ${rc}  ${out}=  Run And Return Rc And Output  govc snapshot.create -vm ${vm} ${snapshot}
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${out}
    Set Environment Variable  GOVC_URL  %{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}

Revert Test Server Snapshot
    [Arguments]  ${vm}  ${snapshot}
    Set Environment Variable  GOVC_URL  %{BUILD_SERVER}
    ${rc}  ${out}=  Run And Return Rc And Output  govc snapshot.revert -vm ${vm} ${snapshot}
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${out}
    Set Environment Variable  GOVC_URL  %{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}

Delete Test Server Snapshot
    [Arguments]  ${vm}  ${snapshot}
    Set Environment Variable  GOVC_URL  %{BUILD_SERVER}
    ${rc}  ${out}=  Run And Return Rc And Output  govc snapshot.remove -vm ${vm} ${snapshot}
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${out}
    Set Environment Variable  GOVC_URL  %{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}

Setup Snapshot
    ${hostname}=  Get Test Server Hostname
    Set Environment Variable  TEST_HOSTNAME  ${hostname}
    Set Environment Variable  SNAPSHOT  vic-ci-test-%{DRONE_BUILD_NUMBER}
    Create Test Server Snapshot  %{TEST_HOSTNAME}  %{SNAPSHOT}

Get Datacenter Name
    ${out}=  Run  govc datacenter.info
    ${out}=  Split To Lines  ${out}
    ${name}=  Fetch From Right  @{out}[0]  ${SPACE}
    [Return]  ${name}

Get Datacenter ID
    ${name}=  Get Datacenter Name
    ${id}=  Run  govc datacenter.info -k --json -dc ${name} | jq .Datacenters[0].Self.Value
    [Return]  ${id}

Get Test Server Hostname
    [Tags]  secret
    ${hostname}=  Run  sshpass -p $TEST_PASSWORD ssh $TEST_USERNAME@$TEST_URL hostname
    [Return]  ${hostname}

Check Delete Success
    [Arguments]  ${name}
    ${out}=  Run  govc ls vm
    Log  ${out}
    Should Not Contain  ${out}  ${name}
    ${out}=  Run  govc datastore.ls
    Log  ${out}
    Should Not Contain  ${out}  ${name}
    ${out}=  Run  govc ls host/*/Resources/*
    Log  ${out}
    Should Not Contain  ${out}  ${name}

Gather vSphere Logs
    Log To Console  Collecting vSphere logs...
    ${out}=  Run  govc logs.download
    Log To Console  vSphere logs collected

Change Log Level On Server
    [Arguments]  ${level}
    ${out}=  Run  govc host.option.set Config.HostAgent.log.level ${level}
    Should Be Empty  ${out}

Add Vsphere License
    [Tags]  secret
    [Arguments]  ${license}
    ${out}=  Run  govc license.add ${license}
    Should Contain  ${out}  Key:

Assign Vsphere License
    [Tags]  secret
    [Arguments]  ${license}  ${host}
    ${out}=  Run  govc license.assign -host ${host} ${license}
    Should Contain  ${out}  Key:

Assign vCenter License
    [Tags]  secret
    [Arguments]  ${license}
    ${out}=  Run  govc license.assign ${license}
    Should Contain  ${out}  Key:

Add Host To VCenter
    [Arguments]  ${host}  ${user}  ${dc}  ${pw}
    :FOR  ${idx}  IN RANGE  1  4
    \   ${out}=  Run  govc cluster.add -hostname=${host} -username=${user} -dc=${dc} -password=${pw} -noverify=true
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  OK
    \   Return From Keyword If  ${status}
    \   Sleep  3 minutes
    Fail  Failed to add the host to the VC in 3 attempts

Get Host Firewall Enabled
    ${output}=  Run  govc host.esxcli network firewall get
    Should Contain  ${output}  Enabled
    @{output}=  Split To Lines  ${output}
    :FOR  ${line}  IN  @{output}
    \   Run Keyword If  "Enabled" in '''${line}'''  Set Test Variable  ${out}  ${line}
    ${enabled}=  Fetch From Right  ${out}  :
    ${enabled}=  Strip String  ${enabled}
    Return From Keyword If  '${enabled}' == 'false'  ${false}
    Return From Keyword If  '${enabled}' == 'true'  ${true}

Enable Host Firewall
    Run  govc host.esxcli network firewall set --enabled true

Disable Host Firewall
    Run  govc host.esxcli network firewall set --enabled false

Check VM Guestinfo
    [Arguments]  ${vm}  ${str}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e ${vm} | grep ${str}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${output}

Get Session List
    ${rc}  ${sessions}=  Run And Return Rc And Output  govc session.ls
    Run Keyword If  ${rc} != 0  Fatal Error  The host appears to be in an unrecoverable state
    [Return]  ${sessions}

Get Hostd ID
    [Tags]  secret
    Return From Keyword If  '%{HOST_TYPE}' != 'ESXi'  Get Hostd ID keyword not valid for non-ESXi servers
    Open Connection  %{TEST_URL}
    Login  %{TEST_USERNAME}  %{TEST_PASSWORD}
    ${out}=  Execute Command  memstats -r group-stats | grep 'hostd '
    ${out}=  Strip String  ${out}
    ${id}=  Fetch From Left  ${out}  ${SPACE}
    Close Connection
    [Return]  ${id}

Get Hostd Memory Consumption
    [Tags]  secret
    Return From Keyword If  '%{HOST_TYPE}' != 'ESXi'  Get Hostd Memory Consumption keyword not valid for non-ESXi servers
    ${id}=  Get Hostd ID
    Log to console  ${id}
    Open Connection  %{TEST_URL}
    Login  %{TEST_USERNAME}  %{TEST_PASSWORD}
    ${out}=  Execute Command  memstats -r group-stats -v -g ${id} -s name:min:max:consumed -l 2
    Log to console  ${out}
    Close Connection
    [Return]  ${out}

# This function will use %{VCH-NAME} and the provided vm name to confirm that the supplied vm will exist at `vm/%{VCH-NAME}/vm-name`
Check VM Folder Path
    [Arguments]  ${vm-name}
    ${vm-path}=  Run  govc find / -type m | grep ${vm-name}
    # If it is esxi - we should find the vm in the vmfolder
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Contain  ${vm-path}  vm/${vm-name}
    # If it is VC - we should find the vm in a folder named after the VCH.
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Contain  ${vm-path}  vm/%{VCH-NAME}/${vm-name}

Check VM Folder Path Doesn't Exist
    [Arguments]  ${vm-name}
    ${vm-path}=  Run  govc find / -type m | grep ${vm-name}
    # If it is esxi - we should find the vm in the vmfolder
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Not Contain  ${vm-path}  vm/${vm-name}
    # If it is VC - we should find the vm in a folder named after the VCH.
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Not Contain  ${vm-path}  vm/%{VCH-NAME}/${vm-name}

Get Public Network VLAN ID
    ${noQuotes}=  Strip String  %{PUBLIC_NETWORK}  characters='"
    ${vlan}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.info --json | jq -r '.Portgroup[].Spec | select(.Name == "${noQuotes}") | .VlanId'
    Return From Keyword If  '%{HOST_TYPE}' == 'ESXi'  ${vlan}

    ${dvs}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  govc find -type DistributedVirtualSwitch | head -n1
    ${vlan}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  govc dvs.portgroup.info -json -pg='${noQuotes}' ${dvs} | jq -r '.Port[0].Config.Setting.Vlan.VlanId'
    Return From Keyword If  '%{HOST_TYPE}' == 'VC'  ${vlan}

Query Cluster DRS Setting
    [Arguments]  ${cluster}

    ${rc}  ${output}=  Run And Return Rc And Output  govc object.collect -json ${cluster} configurationEx | jq '.[].Val.DrsConfig.Enabled'
    Should Be Equal As Integers  ${rc}  0

    [Return]  ${output}
