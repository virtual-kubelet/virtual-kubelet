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
Documentation  This resource contains any keywords related to using the Nimbus cluster

*** Variables ***
${ESX_VERSION}  ob-7867845
${VC_VERSION}  ob-7867539
${NIMBUS_ESX_PASSWORD}  e2eFunctionalTest
${NIMBUS_LOCATION}  ${EMPTY}

*** Keywords ***
Fetch IP
    [Arguments]  ${name}
    ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-ctl ip %{NIMBUS_USER}-${name} | grep %{NIMBUS_USER}-${name}
    Should Not Be Empty  ${out}
    ${len}=  Get Line Count  ${out}
    Should Be Equal As Integers  ${len}  1
    [Return]  ${out}

Get IP
    [Arguments]  ${name}
    ${out}=  Wait Until Keyword Succeeds  10x  1 minute  Fetch IP  ${name}
    ${ip}=  Fetch From Right  ${out}  ${SPACE}
    [Return]  ${ip}

Fetch POD
      [Arguments]  ${name}
      ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-ctl list | grep ${name}
      Should Not Be Empty  ${out}
      ${len}=  Get Line Count  ${out}
      Should Be Equal As Integers  ${len}  1
      ${pod}=  Fetch From Left  ${out}  :
      [return]  ${pod}

Deploy Nimbus ESXi Server
    [Arguments]  ${user}  ${password}  ${version}=${ESX_VERSION}  ${tls_disabled}=True
    ${name}=  Evaluate  'ESX-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    Log To Console  \nDeploying Nimbus ESXi server: ${name}
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  ${user}  ${password}

    :FOR  ${IDX}  IN RANGE  1  5
    \   ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-esxdeploy ${name} --disk=48000000 --ssd=24000000 --memory=8192 --lease=0.25 --nics 2 ${version}
    \   Log  ${out}
    \   # Make sure the deploy actually worked
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  To manage this VM use
    \   Exit For Loop If  ${status}
    \   Log To Console  ${out}
    \   Log To Console  Nimbus deployment ${IDX} failed, trying again in 1 minutes
    \   Sleep  1 minutes

    # Now grab the IP address and return the name and ip for later use
    @{out}=  Split To Lines  ${out}
    :FOR  ${item}  IN  @{out}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  IP is
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${line}  ${item}
    @{gotIP}=  Split String  ${line}  ${SPACE}
    ${ip}=  Remove String  @{gotIP}[5]  ,

    # Let's set a password so govc doesn't complain
    Remove Environment Variable  GOVC_PASSWORD
    Remove Environment Variable  GOVC_USERNAME
    Remove Environment Variable  GOVC_DATACENTER
    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_URL  root:@${ip}
    ${out}=  Run  govc host.account.update -id root -password ${NIMBUS_ESX_PASSWORD}
    Should Be Empty  ${out}
    Run Keyword If  ${tls_disabled}  Disable TLS On ESX Host
    Log To Console  Successfully deployed new ESXi server - ${user}-${name}
    Close connection
    [Return]  ${user}-${name}  ${ip}

Set Host Password
    [Arguments]  ${ip}  ${NIMBUS_ESX_PASSWORD}
    Remove Environment Variable  GOVC_PASSWORD
    Remove Environment Variable  GOVC_USERNAME
    Remove Environment Variable  GOVC_DATACENTER
    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_URL  root:@${ip}
    ${out}=  Run  govc host.account.update -id root -password ${NIMBUS_ESX_PASSWORD}
    Should Be Empty  ${out}
    Disable TLS On ESX Host
    Log To Console  \nNimbus ESXi server IP: ${ip}

Deploy Multiple Nimbus ESXi Servers in Parallel
    [Arguments]  ${number}  ${user}=%{NIMBUS_USER}  ${password}=%{NIMBUS_PASSWORD}  ${version}=${ESX_VERSION}
    @{names}=  Create List
    ${num}=  Convert To Integer  ${number}
    :FOR  ${x}  IN RANGE  ${num}
    \     ${name}=  Evaluate  'ESX-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    \     Append To List  ${names}  ${name}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  ${user}  ${password}

    @{processes}=  Create List
    :FOR  ${name}  IN  @{names}
    \    ${output}=  Deploy Nimbus ESXi Server Async  ${name}
    \    Log  ${output}
    \    Append To List  ${processes}  ${output}

    :FOR  ${process}  IN  @{processes}
    \    ${pid}=  Convert To Integer  ${process}
    \    ${result}=  Wait For Process  ${pid}
    \    Log  ${result.stdout}
    \    Log  ${result.stderr}
    \    Should Be Equal As Integers  ${result.rc}  0

    &{ips}=  Create Dictionary
    :FOR  ${name}  IN  @{names}
    \    ${ip}=  Get IP  ${name}
    \    ${ip}=  Evaluate  $ip if $ip else ''
    \    Run Keyword If  '${ip}'  Set To Dictionary  ${ips}  ${user}-${name}  ${ip}

    # Let's set a password so govc doesn't complain
    ${just_ips}=  Get Dictionary Values  ${ips}
    :FOR  ${ip}  IN  @{just_ips}
    \    Log To Console  Successfully deployed new ESXi server - ${ip}
    \    Set Host Password  ${ip}  ${NIMBUS_ESX_PASSWORD}

    Close connection
    [Return]  ${ips}

Deploy Nimbus vCenter Server
    [Arguments]  ${user}  ${password}  ${version}=${VC_VERSION}
    ${name}=  Evaluate  'VC-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    Log To Console  \nDeploying Nimbus vCenter server: ${name}
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  ${user}  ${password}

    :FOR  ${IDX}  IN RANGE  1  5
    \   ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-vcvadeploy --lease=0.25 --vcvaBuild ${version} ${name}
    \   Log  ${out}
    \   # Make sure the deploy actually worked
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  Overall Status: Succeeded
    \   Exit For Loop If  ${status}
    \   Log To Console  Nimbus deployment ${IDX} failed, trying again in 1 minute
    \   Sleep  1 minutes

    # Now grab the IP address and return the name and ip for later use
    @{out}=  Split To Lines  ${out}
    :FOR  ${item}  IN  @{out}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  Cloudvm is running on IP
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${line}  ${item}
    ${ip}=  Fetch From Right  ${line}  ${SPACE}

    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin!23
    Set Environment Variable  GOVC_URL  ${ip}
    Log To Console  Successfully deployed new vCenter server - ${user}-${name}
    Close connection
    [Return]  ${user}-${name}  ${ip}

Deploy Nimbus ESXi Server Async
    [Tags]  secret
    [Arguments]  ${name}  ${version}=${ESX_VERSION}
    Log To Console  \nDeploying Nimbus ESXi server: ${name}
    ${out}=  Run Secret SSHPASS command  %{NIMBUS_USER}  '%{NIMBUS_PASSWORD}'  '${NIMBUS_LOCATION} nimbus-esxdeploy ${name} --disk\=48000000 --ssd\=24000000 --memory\=8192 --lease=0.25 --nics 2 ${version}'
    [Return]  ${out}

Run Secret SSHPASS command
    [Tags]  secret
    [Arguments]  ${user}  ${password}  ${cmd}

    ${out}=  Start Process  sshpass -p ${password} ssh -o StrictHostKeyChecking\=no -o ServerAliveInterval\=60 -o ServerAliveCountMax\=10 ${user}@%{NIMBUS_GW} ${cmd}  shell=True
    [Return]  ${out}

Deploy Nimbus vCenter Server Async
    [Tags]  secret
    [Arguments]  ${name}  ${version}=${VC_VERSION}
    Log To Console  \nDeploying Nimbus VC server: ${name}

    ${out}=  Run Secret SSHPASS command  %{NIMBUS_USER}  '%{NIMBUS_PASSWORD}'  '${NIMBUS_LOCATION} nimbus-vcvadeploy --lease=0.25 --vcvaBuild ${version} ${name}'
    [Return]  ${out}

# Deploys a nimbus testbed based on the specified testbed spec and options
# Calls Nimbus Cleanup first as a precaution. Impact on concurrent builds
# is unknown.
# user [required] - nimbus user
# password [required] - password for nimbus user
# args [optional] - args to pass into testbeddeploy
# spec [optional] - name of spec file in tests/resources/nimbus-testbeds
Deploy Nimbus Testbed
    [Arguments]  ${user}  ${password}  ${args}=  ${spec}=${EMPTY}

    Run Keyword And Ignore Error  Cleanup Nimbus Folders  deletePxe=${true}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  ${user}  ${password}

    ${specarg}=  Set Variable If  '${spec}' == '${EMPTY}'  ${EMPTY}  --testbedSpecRubyFile ./%{BUILD_TAG}/testbeds/${spec}    

    :FOR  ${IDX}  IN RANGE  1  5
    \   Run Keyword Unless  '${spec}' == '${EMPTY}'  Put File  tests/resources/nimbus-testbeds/${spec}  destination=./%{BUILD_TAG}/testbeds/
    \   ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-testbeddeploy --lease 0.25 ${specarg} ${args}
    \   Log  ${out}
    \   # Make sure the deploy actually worked
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  "deployment_result"=>"PASS"
    \   Return From Keyword If  ${status}  ${out}
    \   Log To Console  Nimbus deployment ${IDX} failed, trying again in 1 minute
    \   Sleep  1 minutes
    Fail  Deploy Nimbus Testbed Failed 5 times over the course of more than 5 minutes

Kill Nimbus Server
    [Arguments]  ${user}  ${password}  ${name}
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  ${user}  ${password}
    ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-ctl kill ${name}
    Log  ${out}
    Close connection

Cleanup Nimbus Folders
    [Arguments]  ${deletePXE}=${false}
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    # TODO: this may need pabot shared resource locking around it for multiple jobs. We're likely making use of the
    # retry paths currently but it's not good practice.
    Run Keyword If  ${deletePXE}  Execute Command  ${NIMBUS_LOCATION} rm -rf public_html/pxe/* public_html/pxeinstall/*
    Execute Command  ${NIMBUS_LOCATION} rm -rf %{BUILD_TAG}
    Close connection

# Cleans up a list of VMs and deletes the pxe folder on nimbus gateway
Nimbus Cleanup
    [Arguments]  ${vm_list}  ${collect_log}=True  ${dontDelete}=${false}
    ${list}=  Catenate  @{vm_list}
    Run Keyword   Nimbus Cleanup Single VM  ${list}  ${collect_log}  ${dontDelete}  ${true}

# Cleans up a vm (or space separated string list of vms) but does not delete pxe folder on nimbus gateway
Nimbus Cleanup Single VM
    [Arguments]  ${vms}  ${collect_log}=True  ${dontDelete}=${false}  ${deletePXE}=${false}
    Run Keyword If  ${collect_log}  Run Keyword And Continue On Failure  Gather Logs From Test Server
    Run Keyword And Ignore Error  Cleanup Nimbus Folders  ${deletePXE}
    Return From Keyword If  ${dontDelete}
    Run Keyword And Ignore Error  Kill Nimbus Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  ${vms}

Gather Host IPs
    ${out}=  Run  govc ls host/cls
    ${out}=  Split To Lines  ${out}
    ${idx}=  Set Variable  1
    :FOR  ${line}  IN  @{out}
    \   Continue For Loop If  '${line}' == '/vcqaDC/host/cls/Resources'
    \   ${ip}=  Fetch From Right  ${line}  /
    \   Set Suite Variable  ${esx${idx}-ip}  ${ip}
    \   ${idx}=  Evaluate  ${idx}+1

Create a VSAN Cluster
    [Arguments]  ${name}=vic-vmotion
    [Timeout]    110 minutes
    Log To Console  \nStarting basic VSAN cluster deploy...
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${out}=  Deploy Nimbus Testbed  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  --plugin testng --lease 0.25 --noStatsDump --noSupportBundles --vcvaBuild ${VC_VERSION} --esxPxeDir ${ESX_VERSION} --esxBuild ${ESX_VERSION} --testbedName vcqa-vsan-simple-pxeBoot-vcva --runName ${name}
    Should Contain  ${out}  .vcva-${VC_VERSION}' is up. IP:
    ${out}=  Split To Lines  ${out}
    :FOR  ${line}  IN  @{out}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  .vcva-${VC_VERSION}' is up. IP:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Suite Variable  ${vc-ip}  ${ip}
    \   Exit For Loop If  ${status}

    Log To Console  Set environment variables up for GOVC
    Set Environment Variable  GOVC_URL  ${vc-ip}
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23

    Log To Console  Create a distributed switch
    ${out}=  Wait Until Keyword Succeeds  10x  3 minutes  Run  govc dvs.create -dc=vcqaDC test-ds
    Should Contain  ${out}  OK

    Log To Console  Create three new distributed switch port groups for management and vm network traffic
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=vcqaDC -dvs=test-ds management
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=vcqaDC -dvs=test-ds vm-network
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=vcqaDC -dvs=test-ds bridge
    Should Contain  ${out}  OK

    Log To Console  Add all the hosts to the distributed switch
    ${out}=  Run  govc dvs.add -dvs=test-ds -pnic=vmnic1 /vcqaDC/host/cls
    Should Contain  ${out}  OK

    Log To Console  Enable DRS and VSAN on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /vcqaDC/host/cls
    Should Be Empty  ${out}

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Set Environment Variable  TEST_DATASTORE  vsanDatastore
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  15m

    Gather Host IPs

Create a Simple VC Cluster
    [Arguments]  ${datacenter}=ha-datacenter  ${cluster}=cls  ${esx_number}=3  ${network}=True
    Log To Console  \nStarting simple VC cluster deploy...
    ${vc}=  Evaluate  'VC-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    ${pid}=  Deploy Nimbus vCenter Server Async  ${vc}

    &{esxes}=  Deploy Multiple Nimbus ESXi Servers in Parallel  ${esx_number}  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  ${ESX_VERSION}
    @{esx_names}=  Get Dictionary Keys  ${esxes}
    @{esx_ips}=  Get Dictionary Values  ${esxes}

    # Finish vCenter deploy
    ${output}=  Wait For Process  ${pid}
    Should Contain  ${output.stdout}  Overall Status: Succeeded

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc_ip}=  Get IP  ${vc}
    Close Connection

    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin!23
    Set Environment Variable  GOVC_URL  ${vc_ip}

    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create ${datacenter}
    Should Be Empty  ${out}

    Log To Console  Create a cluster on the VC
    ${out}=  Run  govc cluster.create ${cluster}
    Should Be Empty  ${out}

    Log To Console  Add ESX host to the VC
    :FOR  ${IDX}  IN RANGE  ${esx_number}
    \   ${out}=  Run  govc cluster.add -hostname=@{esx_ips}[${IDX}] -username=root -dc=${datacenter} -password=${NIMBUS_ESX_PASSWORD} -noverify=true
    \   Should Contain  ${out}  OK

    Run Keyword If  ${network}  Setup Network For Simple VC Cluster  ${esx_number}  ${datacenter}  ${cluster}

    Log To Console  Enable DRS on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /${datacenter}/host/${cluster}
    Should Be Empty  ${out}

    Set Environment Variable  TEST_URL_ARRAY  ${vc_ip}
    Set Environment Variable  TEST_URL  ${vc_ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_DATACENTER  /${datacenter}
    Set Environment Variable  TEST_RESOURCE  ${cluster}
    Set Environment Variable  TEST_TIMEOUT  30m
    [Return]  @{esx_names}  ${vc}  @{esx_ips}  ${vc_ip}

Setup Network For Simple VC Cluster
    [Arguments]  ${esx_number}  ${datacenter}  ${cluster}
    Log To Console  Create a distributed switch
    ${out}=  Run  govc dvs.create -dc=${datacenter} test-ds
    Should Contain  ${out}  OK

    Log To Console  Create three new distributed switch port groups for management and vm network traffic
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=test-ds management
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=test-ds vm-network
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=test-ds bridge
    Should Contain  ${out}  OK

    Add Host To Distributed Switch  /${datacenter}/host/${cluster}  test-ds

    Log To Console  Enable DRS on the cluster
    ${out}=  Run  govc cluster.change -drs-enabled /${datacenter}/host/${cluster}
    Should Be Empty  ${out}

    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network

Create A Distributed Switch
    [Arguments]  ${datacenter}  ${dvs}=test-ds
    Log To Console  \nCreate a distributed switch
    ${out}=  Run  govc dvs.create -dc=${datacenter} ${dvs}
    Should Contain  ${out}  OK

Create Three Distributed Port Groups
    [Arguments]  ${datacenter}  ${dvs}=test-ds
    Log To Console  \nCreate three new distributed switch port groups for management and vm network traffic
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=${dvs} management
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=${dvs} vm-network
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=${datacenter} -dvs=${dvs} bridge
    Should Contain  ${out}  OK

Add Host To Distributed Switch
    [Arguments]  ${host}  ${dvs}=test-ds
    Log To Console  \nAdd host(s) to the distributed switch
    ${out}=  Wait Until Keyword Succeeds  10x  30s  Run  govc dvs.add -dvs=${dvs} -pnic=vmnic1 ${host}
    Should Contain  ${out}  OK

Disable TLS On ESX Host
    Log To Console  \nDisable TLS on the host
    ${ver}=  Get Vsphere Version
    ${out}=  Run Keyword If  '${ver}' != '5.5.0'  Run  govc host.option.set UserVars.ESXiVPsDisabledProtocols sslv3,tlsv1,tlsv1.1
    Run Keyword If  '${ver}' != '5.5.0'  Should Be Empty  ${out}

Get Vsphere Version
    ${out}=  Run  govc about
    ${out}=  Split To Lines  ${out}
    :FOR  ${line}  IN  @{out}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  Version:
    \   Run Keyword And Return If  ${status}  Fetch From Right  ${line}  ${SPACE}

Deploy Simple NFS Testbed
    [Arguments]  ${user}  ${password}  ${spec}=  ${args}=
    ${name}=  Evaluate  'NFS-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    Log To Console  \nDeploying Nimbus NFS testbed: ${name}

    ${out}=  Deploy Nimbus Testbed  user=${user}  password=${password}  spec=${spec}  args=--testbedName nfs --runName ${name} ${args}
    Log  ${out}

    # Make sure the deploy actually worked and all components are up
    Should Contain  ${out}  ${name}.nfs.0' is up. IP:
    Should Contain  ${out}  ${name}.nfs.1' is up. IP:
    Should Contain  ${out}  ${name}.esx.0' is up. IP:

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${nfs-ip}=  Get IP  ${name}.nfs.0
    ${nfs-ro-ip}=  Get IP  ${name}.nfs.1
    ${esx-ip}=  Get IP  ${name}.esx.0
    Close Connection

    Log To Console  \nNFS IP: ${nfs-ip}
    Log To Console  \nNFS READ-Only IP: ${nfs-ro-ip}
    Log To Console  \nESX IP: ${esx-ip}

    [Return]  ${user}-${name}.nfs.0  ${user}-${name}.nfs.1  ${user}-${name}.esx.0  ${nfs-ip}  ${nfs-ro-ip}  ${esx-ip}

Deploy Nimbus NFS Datastore
    [Arguments]  ${user}  ${password}  ${additional-args}=
    ${name}=  Evaluate  'NFS-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    Log To Console  \nDeploying Nimbus NFS server: ${name}
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  ${user}  ${password}

    ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-nfsdeploy ${name} ${additional-args}
    Log  ${out}
    # Make sure the deploy actually worked
    Should Contain  ${out}  To manage this VM use
    # Now grab the IP address and return the name and ip for later use
    @{out}=  Split To Lines  ${out}
    :FOR  ${item}  IN  @{out}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  IP is
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${line}  ${item}
    @{gotIP}=  Split String  ${line}  ${SPACE}
    ${ip}=  Remove String  @{gotIP}[5]  ,

    Log To Console  Successfully deployed new NFS server - ${user}-${name}
    Close connection
    [Return]  ${user}-${name}  ${ip}

Change ESXi Server Password
    [Arguments]  ${password}
    ${out}=  Run  govc host.account.update -id root -password ${password}
    Should Be Empty  ${out}

Check License Present
    ${license}=  Run  govc license.ls
    Log  ${license}
    Should Contain      ${license}  Key
    Should Not Contain  ${license}  SecurityError

Check License Features
    Check License Present
    ${out}=  Run  govc object.collect -json $(govc object.collect -s - content.licenseManager) licenses | jq '.[].Val.LicenseManagerLicenseInfo[].Properties[] | select(.Key == "feature") | .Value'
    Log  ${out}
    Should Contain  ${out}  serialuri
    Should Contain  ${out}  dvs

# Abruptly power off the host
Power Off Host
    [Arguments]  ${host}
    Open Connection  ${host}  prompt=:~]
    Login  root  ${NIMBUS_ESX_PASSWORD}
    ${out}=  Execute Command  poweroff -d 0 -f
    Close connection

Create Simple VC Cluster With Static IP
    [Arguments]  ${name}=vic-simple-vc-static-ip
    [Timeout]    110 minutes
    Set Suite Variable  ${NIMBUS_LOCATION}  NIMBUS_LOCATION=wdc
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    Log To Console  Create a new simple vc cluster with static ip support...
    ${out}=  Deploy Nimbus Testbed  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  spec=vic-simple-cluster-with-static.rb  args=--noSupportBundles --plugin testng --vcvaBuild ${VC_VERSION} --esxBuild ${ESX_VERSION} --testbedName vic-simple-cluster-with-static --runName ${name}
    Log  ${out}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc-ip}=  Get IP  ${name}.vc.0
    ${worker-ip}=  Get IP  ${name}.worker.0
    Close Connection

    Set Suite Variable  @{list}  %{NIMBUS_USER}-${name}.esx.0  %{NIMBUS_USER}-${name}.esx.1  %{NIMBUS_USER}-${name}.esx.2  %{NIMBUS_USER}-${name}.nfs.0  %{NIMBUS_USER}-${name}.vc.0  %{NIMBUS_USER}-${name}.worker.0
    Log To Console  Finished creating cluster ${name}

    Set Environment Variable  STATIC_WORKER_IP  ${worker-ip}
    ${out}=  Get Static IP Address
    Set Suite Variable  ${static}  ${out}

    Log To Console  Set environment variables up for GOVC
    Set Environment Variable  GOVC_URL  ${vc-ip}
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23

    Log To Console  Deploy VIC to the VC cluster
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  BRIDGE_NETWORK  bridge
    Set Environment Variable  PUBLIC_NETWORK  vm-network
    Remove Environment Variable  TEST_DATACENTER
    Set Environment Variable  TEST_DATASTORE  nfs0-1
    Set Environment Variable  TEST_RESOURCE  cls
    Set Environment Variable  TEST_TIMEOUT  15m

Create Static IP Worker
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Log To Console  Create a new static ip address worker...
    ${name}=  Evaluate  'static-worker-' + str(random.randint(1000,9999)) + str(time.clock())  modules=random,time
    Log To Console  \nDeploying static ip worker: ${name}
    ${out}=  Execute Command  ${NIMBUS_LOCATION} nimbus-ctl --silentObjectNotFoundError kill '%{NIMBUS_USER}-static-worker' && ${NIMBUS_LOCATION} nimbus-worker-deploy --nimbus ${NIMBUS_POD} --enableStaticIpService ${name}
    Should Contain  ${out}  "deploy_status": "success"

    ${pod}=  Fetch POD  ${name}
    Run Keyword If  '${pod}' != '${NIMBUS_POD}'  Kill Nimbus Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  %{NIMBUS_USER}-${name}
    Run Keyword If  '${pod}' != '${NIMBUS_POD}'  Fail  Nimbus pod suggestion failed

    Set Environment Variable  STATIC_WORKER_NAME  %{NIMBUS_USER}-${name}
    ${ip}=  Get IP  ${name}
    Set Environment Variable  STATIC_WORKER_IP  ${ip}
    Close Connection

Get Static IP Address
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  STATIC_WORKER_IP
    Run Keyword If  '${status}' == 'FAIL'  Wait Until Keyword Succeeds  10x  10s  Create Static IP Worker
    Log To Console  Curl a new static ip address from the created worker...
    ${out}=  Run  curl -s http://%{STATIC_WORKER_IP}:4827/nsips

    &{static}=  Create Dictionary
    ${ip}=  Run  echo '${out}' | jq -r ".ip"
    Set To Dictionary  ${static}  ip  ${ip}
    ${netmask}=  Run  echo '${out}' | jq -r ".netmask"
    ${netmask}=  Evaluate  sum([bin(int(x)).count("1") for x in "${netmask}".split(".")])
    Set To Dictionary  ${static}  netmask  ${netmask}
    ${gateway}=  Run  echo '${out}' | jq -r ".gateway"
    Set To Dictionary  ${static}  gateway  ${gateway}
    [Return]  ${static}

Is Nimbus Location WDC
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  10 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${out}=  Execute Command  env | grep NIMBUS_LOCATION=wdc
    ${status}=  Run Keyword And Return Status  Should Not Be Empty  ${out}
    Close Connection
    [Return]  ${status}

Get Name of First Local Storage For Host
    [Arguments]  ${host}
    ${datastores}=  Run  govc host.info -host ${host} -json | jq -r '.HostSystems[].Config.FileSystemVolume.MountInfo[].Volume | select (.Type\=\="VMFS") | select (.Local\=\=true) | .Name'
    @{datastores}=  Split To Lines  ${datastores}
    [Return]  @{datastores}[0]

# Simple wrapper to Wait Until Keyword Succeeds that allows callers to:
# * use default retry count and delay
# * specify specific retry counts and delays
# * executor can globally override the above via the environment variables:
#   NIMBUS_RETRY_ATTEMPTS
#   NIMBUS_RETRY_DELAY
Nimbus Suite Setup
    [Arguments]  ${keyword}  @{varargs}

    ${useAttempts}=  Get Environment Variable  NIMBUS_RETRY_ATTEMPTS  1
    ${useDelay}=     Get Environment Variable  NIMBUS_RETRY_DELAY     1m

    Wait Until Keyword Succeeds  ${useAttempts}x  ${useDelay}  ${keyword}  @{varargs}
