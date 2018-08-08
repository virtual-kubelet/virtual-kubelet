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
Documentation  Common keywords used by VIC UI installation & uninstallation test suites
Resource  ../../resources/Util.robot
Library  VicUiInstallPexpectLibrary.py

*** Variables ***
${TEST_VC_VERSION}          6.0
${TEST_VC_ROOT_PASSWORD}    vmware
${TIMEOUT}                  10 minutes

${SELENIUM_SERVER_PORT}     4444
${DATACENTER_NAME}          Datacenter
${CLUSTER_NAME}             Cluster
${DATASTORE_TYPE}           NFS
${DATASTORE_NAME}           fake
${DATASTORE_IP}             1.1.1.1
${CONTAINER_VM_NAME}        sharp_feynman-d39db0a231f2f639a073814c2affc03e4737d9ad361649069eb424e6c4e09b52
${TEST_OS}                  %{TEST_OS}
${vic_macmini_fileserver_url}  https://10.20.121.192:3443/vsphere-plugins/
${vic_macmini_fileserver_thumbprint}  BE:64:39:8B:BD:98:47:4D:E8:3B:2F:20:A5:21:8B:86:5F:AD:79:CE

*** Keywords ***
Set Fileserver And Thumbprint In Configs
    [Arguments]  ${fake}=${FALSE}
    ${fileserver_url}=  Run Keyword If  ${fake} == ${TRUE}  Set Variable  256.256.256.256  ELSE  Set Variable  ${vic_macmini_fileserver_url}
    ${fileserver_thumbprint}=  Run Keyword If  ${fake} == ${TRUE}  Set Variable  ab:cd:ef  ELSE  Set Variable  ${vic_macmini_fileserver_thumbprint}
    ${results}=  Replace String Using Regexp  ${configs}  VIC_UI_HOST_URL=.*  VIC_UI_HOST_URL=\"${fileserver_url}\"
    ${results}=  Replace String Using Regexp  ${results}  VIC_UI_HOST_THUMBPRINT=.*  VIC_UI_HOST_THUMBPRINT=\"${fileserver_thumbprint}\"
    Create File  ${UI_INSTALLER_PATH}/configs  ${results}

Load Nimbus Testbed Env
    Should Exist  testbed-information
    ${envs}=  OperatingSystem.Get File  testbed-information
    @{envs}=  Split To Lines  ${envs}
    :FOR  ${item}  IN  @{envs}
    \  @{kv}=  Split String  ${item}  =
    \  Set Environment Variable  @{kv}[0]  @{kv}[1]
    \  Set Suite Variable  \$@{kv}[0]  @{kv}[1]
    Set Suite Variable  ${TEST_VC_USERNAME}  %{TEST_USERNAME}
    Set Suite Variable  ${TEST_VC_PASSWORD}  %{TEST_PASSWORD}

Install VIC Appliance For VIC UI
    [Arguments]  ${vic-machine}=ui-nightly-run-bin/vic-machine-linux  ${appliance-iso}=ui-nightly-run-bin/appliance.iso  ${bootstrap-iso}=ui-nightly-run-bin/bootstrap.iso  ${certs}=${true}  ${vol}=default
    Set Test Environment Variables
    # disable firewall
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.esxcli network firewall set -e false
    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On VIC UI Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    Run Keyword And Ignore Error  Cleanup Dangling Networks On Test Server
    Run Keyword And Ignore Error  Cleanup Dangling vSwitches On Test Server

    # Install the VCH now
    Log To Console  \nInstalling VCH to test server...
    ${output}=  Run VIC Machine Command  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${EMPTY}
    Log  ${output}
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${certs}
    Log To Console  Installer completed successfully: %{VCH-NAME}...

Cleanup Dangling VMs On VIC UI Test Server
    ${out}=  Run  govc ls vm
    ${vms}=  Split To Lines  ${out}
    :FOR  ${vm}  IN  @{vms}
    \   ${vm}=  Fetch From Right  ${vm}  /
    \   ${build}=  Split String  ${vm}  -
    \   # Skip any VM that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   # Skip any VM that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   ${uuid}=  Run  govc vm.info -json\=true ${vm} | jq -r '.VirtualMachines[0].Config.Uuid'
    \   Log To Console  Destroying dangling VCH: ${vm}
    \   ${rc}  ${output}=  Delete VIC Machine  ${vm}  ../../../ui-nightly-run-bin/vic-machine-linux

Check Config And Install VCH
    [Arguments]  ${plugin}=noop
    Run Keyword  Set Absolute Script Paths
    Load Nimbus Testbed Env
    Set Environment Variable  DOMAIN  ${EMPTY}
    Install VIC Appliance For VIC UI  ../../../ui-nightly-run-bin/vic-machine-linux  ../../../ui-nightly-run-bin/appliance.iso  ../../../ui-nightly-run-bin/bootstrap.iso
    Set Environment Variable  VCH_VM_NAME  %{VCH-NAME}
    ${vc_fingerprint}=  Run  ../../../ui-nightly-run-bin/vic-ui-linux info --user ${TEST_VC_USERNAME} --password ${TEST_VC_PASSWORD} --target ${TEST_VC_IP} --key com.vmware.vic.noop 2>&1 | grep -o "(thumbprint.*)" | awk -F= '{print $2}' | sed 's/.$//'
    Set Environment Variable  VC_FINGERPRINT  ${vc_fingerprint}
    Run Keyword If  '${plugin}' == 'install'  Force Install Vicui Plugin
    Run Keyword If  '${plugin}' == 'remove'  Force Remove Vicui Plugin

Set Absolute Script Paths
    ${UI_INSTALLERS_ROOT}=  Run  pwd
    ${UI_INSTALLERS_ROOT}=  Join Path  ${UI_INSTALLERS_ROOT}  ../../../ui/installer
    Set Suite Variable  ${UI_INSTALLER_PATH}  ${UI_INSTALLERS_ROOT}/VCSA
    Should Exist  ${UI_INSTALLER_PATH}
    ${configs_content}=  OperatingSystem.GetFile  ${UI_INSTALLER_PATH}/configs
    Set Suite Variable  ${configs}  ${configs_content}
    Run Keyword If  %{TEST_VSPHERE_VER} == 65  Set Suite Variable  ${plugin_folder}  plugin-packages  ELSE  Set Suite Variable  ${plugin_folder}  vsphere-client-serenity

    # set exact paths for installer and uninstaller scripts
    Set Script Filename  INSTALLER_SCRIPT_PATH  ./install
    Set Script Filename  UNINSTALLER_SCRIPT_PATH  ./uninstall

Set Script Filename
    [Arguments]    ${suite_varname}  ${script_name}
    ${SCRIPT_FILENAME}=  Set Variable  ${script_name}.sh
    ${SCRIPT_FILENAME}=  Join Path  ${UI_INSTALLER_PATH}  ${SCRIPT_FILENAME}
    Set Suite Variable  \$${suite_varname}  ${SCRIPT_FILENAME}

Reset Configs
    # Revert the configs file back to what it was
    ${results}=  Replace String Using Regexp  ${configs}  VIC_UI_HOST_URL=.*  VIC_UI_HOST_URL=\"\"
    ${results}=  Replace String Using Regexp  ${results}  VIC_UI_HOST_THUMBPRINT=.*  VIC_UI_HOST_THUMBPRINT=\"\"
    Create File  ${UI_INSTALLER_PATH}/configs  ${results}
    Should Exist  ${UI_INSTALLER_PATH}/configs

Force Install Vicui Plugin
    Set Fileserver And Thumbprint In Configs
    Append To File  ${UI_INSTALLER_PATH}/configs  BYPASS_PLUGIN_VERIFICATION=1\n
    Install Plugin Successfully  ${TEST_VC_IP}  ${TEST_VC_USERNAME}  ${TEST_VC_PASSWORD}  ${TRUE}  None  ${TRUE}
    Reset Configs
    ${output}=  OperatingSystem.GetFile  install.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  exited successfully
    Run Keyword Unless  ${passed}  Copy File  install.log  fail-force-install-vicui-plugin.log
    Remove File  install.log
    Should Be True  ${passed}

Force Remove Vicui Plugin
    ${rc}  ${output}=  Run And Return Rc And Output  ../../../ui-nightly-run-bin/vic-ui-linux remove --thumbprint %{VC_FINGERPRINT} --target ${TEST_VC_IP} --user ${TEST_VC_USERNAME} --password ${TEST_VC_PASSWORD} --key com.vmware.vic.ui
    ${rc}  ${output}=  Run And Return Rc And Output  ../../../ui-nightly-run-bin/vic-ui-linux remove --thumbprint %{VC_FINGERPRINT} --target ${TEST_VC_IP} --user ${TEST_VC_USERNAME} --password ${TEST_VC_PASSWORD} --key com.vmware.vic

Rename Folder
    [Arguments]  ${old}  ${new}
    Move Directory  ${old}  ${new}
    Should Exist  ${new}

Cleanup Installer Environment
    # Reverts the configs file and make sure the folder containing the UI binaries has its original name that might've been left modified due to a test failure
    Reset Configs
    @{folders}=  OperatingSystem.List Directory  ${UI_INSTALLER_PATH}/..  ${plugin_folder}*
    Run Keyword If  ('@{folders}[0]' != '${plugin_folder}')  Rename Folder  ${UI_INSTALLER_PATH}/../@{folders}[0]  ${UI_INSTALLER_PATH}/../${plugin_folder}

Delete VIC Machine
    [Tags]  secret
    [Arguments]  ${vch-name}  ${vic-machine}=ui-nightly-run-bin/vic-machine-linux
    ${rc}  ${output}=  Run And Return Rc And Output  ${vic-machine} delete --name=${vch-name} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
    [Return]  ${rc}  ${output}

Uninstall VCH
    [Arguments]  ${remove_plugin}=${FALSE}
    Log To Console  Gathering logs from the test server...
    Gather Logs From Test Server
    Log To Console  Deleting the VCH appliance...
    ${rc}  ${output}=  Delete VIC Machine  %{VCH-NAME}  ../../../ui-nightly-run-bin/vic-machine-linux
    Check Delete Success  %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Completed successfully
    ${output}=  Run  rm -f %{VCH-NAME}-*.pem
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.remove %{VCH-NAME}-bridge
    Run Keyword If  ${remove_plugin} == ${TRUE}  Force Remove Vicui Plugin
