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
Documentation  Set up testbed before running the UI tests
Resource  ../../resources/Util.robot

*** Variables ***
${MACOS_SELENIUM_IP}    10.20.121.192
${UBUNTU_SELENIUM_IP}   10.20.121.145

*** Keywords ***
Check If Nimbus VMs Exist
    # remove testbed-information if it exists
    ${ti_exists}=  Run Keyword And Return Status  OperatingSystem.Should Exist  testbed-information
    Run Keyword If  ${ti_exists}  Remove File  testbed-information

    ${nimbus_machines}=  Set Variable  %{NIMBUS_USER}-UITEST-*
    Log To Console  \nFinding Nimbus machines for UI tests
    Open Connection  %{NIMBUS_GW}
    Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}

    ${out}=  Execute Command  nimbus-ctl list | grep -i "${nimbus_machines}"
    @{out}=  Split To Lines  ${out}
    ${out_len}=  Get Length  ${out}
    Close connection

    Run Keyword If  ${out_len} == 0  Setup Testbed  ELSE  Load Testbed  ${out}
    Create File  testbed-information  TEST_VSPHERE_VER=%{TEST_VSPHERE_VER}\nSELENIUM_SERVER_IP=%{SELENIUM_SERVER_IP}\nTEST_ESX_NAME=%{TEST_ESX_NAME}\nESX_HOST_IP=%{ESX_HOST_IP}\nESX_HOST_PASSWORD=%{ESX_HOST_PASSWORD}\nTEST_VC_NAME=%{TEST_VC_NAME}\nTEST_VC_IP=%{TEST_VC_IP}\nTEST_URL_ARRAY=%{TEST_URL_ARRAY}\nTEST_USERNAME=%{TEST_USERNAME}\nTEST_PASSWORD=%{TEST_PASSWORD}\nTEST_DATASTORE=datastore1\nEXTERNAL_NETWORK=%{EXTERNAL_NETWORK}\nTEST_TIMEOUT=%{TEST_TIMEOUT}\nGOVC_INSECURE=%{GOVC_INSECURE}\nGOVC_USERNAME=%{GOVC_USERNAME}\nGOVC_PASSWORD=%{GOVC_PASSWORD}\nGOVC_URL=%{GOVC_URL}\n

Destroy Testbed
    [Arguments]  ${name}
    Log To Console  Destroying VM(s) ${name}
    Run Keyword And Ignore Error  Kill Nimbus Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  ${name}

Load Testbed
    [Arguments]  ${list}
    Log To Console  Retrieving VMs information for UI testing...\n
    ${len}=  Get Length  ${list}
    @{browservm-found}=  Create List
    @{esx-found}=  Create List
    @{vcsa-found}=  Create List
    ${browservm-requested}=  Run Keyword If  '%{TEST_OS}' == 'Ubuntu'  Set Variable  BROWSERVM-UBUNTU  ELSE  Set Variable  BROWSERVM-WINDOWS
    :FOR  ${vm}  IN  @{list}
    \  @{vm_items}=  Split String  ${vm}  :
    \  ${is_esx}=  Run Keyword And Return Status  Should Match Regexp  @{vm_items}[1]  (?i)esx%{TEST_VSPHERE_VER}
    \  ${is_vcsa}=  Run Keyword And Return Status  Should Match Regexp  @{vm_items}[1]  (?i)vc%{TEST_VSPHERE_VER}
    \  ${is_browservm}=  Run Keyword And Return Status  Should Match Regexp  @{vm_items}[1]  (?i)${browservm-requested}
    \  Run Keyword If  ${is_browservm}  Set Test Variable  @{browservm-found}  @{vm_items}  ELSE IF  ${is_esx}  Set Test Variable  @{esx-found}  @{vm_items}  ELSE IF  ${is_vcsa}  Set Test Variable  @{vcsa-found}  @{vm_items}
    ${browservm_len}=  Get Length  ${browservm-found}
    ${esx_len}=  Get Length  ${esx-found}
    ${vcsa_len}=  Get Length  ${vcsa-found}
    Run Keyword If  ${browservm_len} > 0  Extract BrowserVm Info  @{browservm-found}  ELSE  Deploy BrowserVm
    Run Keyword If  (${esx_len} == 0 and ${vcsa_len} > 0) or (${esx_len} > 0 and ${vcsa_len} == 0)  Run Keywords  Destroy Testbed  '%{NIMBUS_USER}-UITEST-VC%{TEST_VSPHERE_VER}*'  AND  Destroy Testbed  '%{NIMBUS_USER}-UITEST-ESX%{TEST_VSPHERE_VER}*'  AND  Deploy Esx  AND  Deploy Vcsa
    Run Keyword If  ${esx_len} == 0 and ${vcsa_len} == 0  Run Keywords  Deploy Esx  AND  Deploy Vcsa
    Run Keyword If  ${esx_len} > 0 and ${vcsa_len} > 0  Run Keywords  Extract Esx Info  @{esx-found}  AND  Extract Vcsa Info  @{vcsa-found}

Extract BrowserVm Info
    [Arguments]  @{vm_fields}
    Open Connection  %{NIMBUS_GW}
    Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vm_name}=  Evaluate  '@{vm_fields}[1]'.strip()
    ${out}=  Execute Command  NIMBUS=@{vm_fields}[0] nimbus-ctl ip ${vm_name} | grep -i ".*: %{NIMBUS_USER}-.*"
    @{out}=  Split String  ${out}  :
    ${vm_ip}=  Evaluate  '@{out}[2]'.strip()
    Run Keyword If  '%{TEST_OS}' == 'Mac'  Set Environment Variable  SELENIUM_SERVER_IP  ${MACOS_SELENIUM_IP}  ELSE IF  '%{TEST_OS}' == 'Ubuntu'  Set Environment Variable  SELENIUM_SERVER_IP  ${UBUNTU_SELENIUM_IP}  ELSE  Set Environment Variable  SELENIUM_SERVER_IP  ${vm_ip}
    Close Connection

Extract Esx Info
    [Arguments]  @{vm_fields}
    Open Connection  %{NIMBUS_GW}
    Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vm_name}=  Evaluate  '@{vm_fields}[1]'.strip()
    ${out}=  Execute Command  NIMBUS=@{vm_fields}[0] nimbus-ctl ip ${vm_name} | grep -i ".*: %{NIMBUS_USER}-.*"
    @{out}=  Split String  ${out}  :
    ${vm_ip}=  Evaluate  '@{out}[2]'.strip()
    Set Environment Variable  TEST_ESX_NAME  ${vm_name}
    Set Environment Variable  ESX_HOST_IP  ${vm_ip}
    Set Environment Variable  ESX_HOST_PASSWORD  e2eFunctionalTest
    Close Connection

Extract Vcsa Info
    [Arguments]  @{vm_fields}
    Open Connection  %{NIMBUS_GW}
    Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vm_name}=  Evaluate  '@{vm_fields}[1]'.strip()
    ${out}=  Execute Command  NIMBUS=@{vm_fields}[0] nimbus-ctl ip ${vm_name} | grep -i ".*: %{NIMBUS_USER}-.*"
    @{out}=  Split String  ${out}  :
    ${vm_ip}=  Evaluate  '@{out}[2]'.strip()
    Set Environment Variable  TEST_VC_NAME  ${vm_name}
    Set Environment Variable  TEST_VC_IP  ${vm_ip}
    Set Environment Variable  TEST_URL_ARRAY  ${vm_ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  EXTERNAL_NETWORK  vm-network
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23
    Set Environment Variable  GOVC_URL  ${vm_ip}
    Close Connection

Deploy BrowserVm
    # deploy a browser vm
    ${browservm}  ${browservm-ip}=  Run Keyword If  '%{TEST_OS}' == 'Mac'  No Operation  ELSE IF  '%{TEST_OS}' == 'Ubuntu'  No Operation  ELSE  Deploy Nimbus BrowserVm For NGC Testing  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Run Keyword If  '%{TEST_OS}' == 'Mac'  Set Environment Variable  SELENIUM_SERVER_IP  ${MACOS_SELENIUM_IP}  ELSE IF  '%{TEST_OS}' == 'Ubuntu'  Set Environment Variable  SELENIUM_SERVER_IP  ${UBUNTU_SELENIUM_IP}  ELSE  Set Environment Variable  SELENIUM_SERVER_IP  ${browservm-ip}

Deploy Esx
    # deploy an esxi server
    ${name}=  Evaluate  'UITEST-ESX%{TEST_VSPHERE_VER}-' + str(random.randint(1000,9999))  modules=random
    ${buildnum}=  Run Keyword If  %{TEST_VSPHERE_VER} == 60  Set Variable  3620759  ELSE  Set Variable  5310538
    ${out}=  Deploy Nimbus ESXi Server Async  ${name}  ${buildnum}
    ${result}=  Wait For Process  ${out}
    Log  ${result.stdout}
    Log  ${result.stderr}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${esx1-ip}=  Get IP  ${name}
    Remove Environment Variable  GOVC_PASSWORD
    Remove Environment Variable  GOVC_USERNAME
    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_URL  root:@${esx1-ip}
    ${out}=  Run  govc host.account.update -id root -password e2eFunctionalTest
    Should Be Empty  ${out}
    Log To Console  Successfully deployed %{NIMBUS_USER}-${name}. IP: ${esx1-ip}
    Close Connection

    Set Environment Variable  TEST_ESX_NAME  %{NIMBUS_USER}-${name}
    Set Environment Variable  ESX_HOST_IP  ${esx1-ip}
    Set Environment Variable  ESX_HOST_PASSWORD  e2eFunctionalTest

Deploy Vcsa
    # deploy a vcsa
    ${name}=  Evaluate  'UITEST-VC%{TEST_VSPHERE_VER}-' + str(random.randint(1000,9999))  modules=random
    ${buildnum}=  Run Keyword If  %{TEST_VSPHERE_VER} == 60  Set Variable  3634791  ELSE  Set Variable  5318154
    ${out}=  Deploy Nimbus vCenter Server Async  ${name} --useQaNgc  ${buildnum}
    ${result}=  Wait For Process  ${out}
    Log  ${result.stdout}
    Log  ${result.stderr}

    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${vc-ip}=  Get IP  ${name}
    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin!23
    Set Environment Variable  GOVC_URL  ${vc-ip}
    Log To Console  Successfully deployed %{NIMBUS_USER}-${name}. IP: ${vc-ip}
    Close Connection

    # create a datacenter
    Log To Console  Create a datacenter on the VC
    ${out}=  Run  govc datacenter.create Datacenter
    Should Be Empty  ${out}

    # make a cluster
    Log To Console  Create a cluster on the datacenter
    ${out}=  Run  govc cluster.create -dc=Datacenter Cluster
    Should Be Empty  ${out}
    ${out}=  Run  govc cluster.change -dc=Datacenter -drs-enabled=true /Datacenter/host/Cluster
    Should Be Empty  ${out}

    # add the esx host to the cluster
    Log To Console  Add ESX host to Cluster
    ${out}=  Run  govc cluster.add -dc=Datacenter -cluster=/Datacenter/host/Cluster -username=root -password=e2eFunctionalTest -noverify=true -hostname=%{ESX_HOST_IP}
    Should Contain  ${out}  OK

    # create a distributed switch
    Log To Console  Create a distributed switch
    ${out}=  Run  govc dvs.create -dc=Datacenter test-ds
    Should Contain  ${out}  OK

    # make three port groups
    Log To Console  Create three new distributed switch port groups for management and vm network traffic
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=Datacenter -dvs=test-ds management
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=Datacenter -dvs=test-ds vm-network
    Should Contain  ${out}  OK
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=Datacenter -dvs=test-ds network
    Should Contain  ${out}  OK

    Log To Console  Add the ESXi hosts to the portgroups
    ${out}=  Run  govc dvs.add -dvs=test-ds -pnic=vmnic1 -host.ip=%{ESX_HOST_IP} %{ESX_HOST_IP}
    Should Contain  ${out}  OK

    Set Environment Variable  TEST_VC_NAME  %{NIMBUS_USER}-${name}
    Set Environment Variable  TEST_VC_IP  ${vc-ip}
    Set Environment Variable  TEST_URL_ARRAY  ${vc-ip}
    Set Environment Variable  TEST_USERNAME  Administrator@vsphere.local
    Set Environment Variable  TEST_PASSWORD  Admin\!23
    Set Environment Variable  EXTERNAL_NETWORK  vm-network
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  GOVC_INSECURE  1
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23
    Set Environment Variable  GOVC_URL  ${vc-ip}

Setup Testbed
    Deploy BrowserVm
    Deploy Esx
    Deploy Vcsa

Deploy Nimbus BrowserVm For NGC Testing
    [Arguments]  ${user}  ${password}
    # for Mac & Ubuntu this keyword will never be called
    # ${os}=  Run Keyword If  '%{TEST_OS}' == 'Ubuntu'  Set Variable  UBUNTU  ELSE  Set Variable  WINDOWS
    # ${vm-template}=  Run Keyword If  '%{TEST_OS}' == 'Ubuntu'  Set Variable  hsuia-seleniumNode-ubuntu --memory 2048  ELSE  Set Variable  ngc-testvm-3
    ${os}=  Set Variable  WINDOWS
    ${vm-template}=  Set Variable  ngc-testvm-3
    ${name}=  Evaluate  'UITEST-BROWSERVM-${os}-' + str(random.randint(1000,9999))  modules=random
    Log To Console  \nDeploying Browser VM: ${name}
    Open Connection  %{NIMBUS_GW}
    Login  ${user}  ${password}

    ${out}=  Execute Command  nimbus-genericdeploy --type ${vm-template} ${name} --lease 3
    # Make sure the deploy actually worked
    Should Contain  ${out}  To manage this VM use
    # Now grab the IP address and return the name and ip for later use
    @{out}=  Split To Lines  ${out}
    :FOR  ${item}  IN  @{out}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  IP is
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${line}  ${item}
    @{gotIP}=  Split String  ${line}  ${SPACE}
    ${ip}=  Remove String  @{gotIP}[5]  ,

    Log To Console  Successfully deployed new Browser VM - ${user}-${name}
    Close connection
    [Return]  ${user}-${name}  ${ip}

*** Test Cases ***
Check Variables
    ${isset_SHELL}=  Run Keyword And Return Status  Environment Variable Should Be Set  SHELL
    ${isset_DRONE_SERVER}=  Run Keyword And Return Status  Environment Variable Should Be Set  DRONE_SERVER
    ${isset_DRONE_TOKEN}=  Run Keyword And Return Status  Environment Variable Should Be Set  DRONE_TOKEN
    ${isset_NIMBUS_USER}=  Run Keyword And Return Status  Environment Variable Should Be Set  NIMBUS_USER
    ${isset_NIMBUS_PASSWORD}=  Run Keyword And Return Status  Environment Variable Should Be Set  NIMBUS_PASSWORD
    ${isset_NIMBUS_GW}=  Run Keyword And Return Status  Environment Variable Should Be Set  NIMBUS_GW
    ${isset_TEST_DATASTORE}=  Run Keyword And Return Status  Environment Variable Should Be Set  TEST_DATASTORE
    ${isset_TEST_RESOURCE}=  Run Keyword And Return Status  Environment Variable Should Be Set  TEST_RESOURCE
    ${isset_GOVC_INSECURE}=  Run Keyword And Return Status  Environment Variable Should Be Set  GOVC_INSECURE
    Log To Console  \nChecking environment variables
    Log To Console  SHELL ${isset_SHELL}
    Log To Console  DRONE_SERVER ${isset_DRONE_SERVER}
    Log To Console  DRONE_TOKEN ${isset_DRONE_TOKEN}
    Log To Console  NIMBUS_USER ${isset_NIMBUS_USER}
    Log To Console  NIMBUS_PASSWORD ${isset_NIMBUS_PASSWORD}
    Log To Console  NIMBUS_GW ${isset_NIMBUS_GW}
    Log To Console  TEST_DATASTORE ${isset_TEST_DATASTORE}
    Log To Console  TEST_RESOURCE ${isset_TEST_RESOURCE}
    Log To Console  GOVC_INSECURE ${isset_GOVC_INSECURE}
    Log To Console  TEST_VSPHERE_VER %{TEST_VSPHERE_VER}
    Should Be True  ${isset_SHELL} and ${isset_DRONE_SERVER} and ${isset_DRONE_TOKEN} and ${isset_NIMBUS_USER} and ${isset_NIMBUS_GW} and ${isset_TEST_DATASTORE} and ${isset_TEST_RESOURCE} and ${isset_GOVC_INSECURE} and %{TEST_VSPHERE_VER}

Check Nimbus Machines
    Check If Nimbus VMs Exist
