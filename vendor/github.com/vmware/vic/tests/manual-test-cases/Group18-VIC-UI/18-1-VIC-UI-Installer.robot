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
Documentation  Test 18-1 - VIC UI Installation
Resource  ../../resources/Util.robot
Resource  ./vicui-common.robot
Test Teardown  Cleanup Installer Environment
Suite Setup  Check Config And Install VCH  remove
Suite Teardown  Uninstall VCH  ${TRUE}

*** Test Cases ***
Attempt To Install With Configs File Missing
    # Rename the configs file and run the installer script to see if it fails in an expected way
    Move File  ${UI_INSTALLER_PATH}/configs  ${UI_INSTALLER_PATH}/configs_renamed
    ${rc}  ${output}=  Run And Return Rc And Output  ${INSTALLER_SCRIPT_PATH}
    Run Keyword And Continue On Failure  Should Contain  ${output}  Configs file is missing
    Move File  ${UI_INSTALLER_PATH}/configs_renamed  ${UI_INSTALLER_PATH}/configs

Attempt To Install With Manifest Missing
    Move File  ${UI_INSTALLER_PATH}/../plugin-manifest  ${UI_INSTALLER_PATH}/../plugin-manifest-a
    ${rc}  ${output}=  Run And Return Rc And Output  cd ${UI_INSTALLER_PATH} && ${INSTALLER_SCRIPT_PATH}
    Run Keyword And Continue On Failure  Should Contain  ${output}  manifest was not found
    Move File  ${UI_INSTALLER_PATH}/../plugin-manifest-a  ${UI_INSTALLER_PATH}/../plugin-manifest

Attempt To Install To A Non vCenter Server
    Install Fails  not-a-vcenter-server  admin  password  ${TRUE}
    ${output}=  OperatingSystem.GetFile  install.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  vCenter Server was not found
    Run Keyword Unless  ${passed}  Move File  install.log  install-fail-attempt-to-a-non-vcenter-server.log
    Should Be True  ${passed}

Attempt To Install With Wrong Vcenter Credentials
    Set Fileserver And Thumbprint In Configs
    Append To File  ${UI_INSTALLER_PATH}/configs  BYPASS_PLUGIN_VERIFICATION=1\n
    Install Fails  ${TEST_VC_IP}  ${TEST_VC_USERNAME}_nope  ${TEST_VC_PASSWORD}_nope  ${FALSE}  %{VC_FINGERPRINT}
    ${output}=  OperatingSystem.GetFile  install.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  Cannot complete login due to an incorrect user name or password
    Run Keyword Unless  ${passed}  Move File  install.log  install-fail-attempt-to-install-with-wrong-vcenter-credentials.log
    Should Be True  ${passed}

Attempt to Install With Unmatching Fingerprint
    Append To File  ${UI_INSTALLER_PATH}/configs  BYPASS_PLUGIN_VERIFICATION=1\n
    Install Fails  ${TEST_VC_IP}  ${TEST_VC_USERNAME}  ${TEST_VC_PASSWORD}  ${FALSE}  ff:ff:ff
    ${output}=  OperatingSystem.GetFile  install.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  does not match
    Run Keyword Unless  ${passed}  Move File  install.log  install-fail-attempt-to-install-with-unmatching-fingerprint.log
    Should Be True  ${passed}

Attempt To Install With Wrong OVA Fileserver URL
    Set Fileserver And Thumbprint In Configs  ${TRUE}
    Install Fails  ${TEST_VC_IP}  ${TEST_VC_USERNAME}  ${TEST_VC_PASSWORD}  ${TRUE}
    ${output}=  OperatingSystem.GetFile  install.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  Error
    Run Keyword Unless  ${passed}  Move File  install.log  install-fail-attempt-to-install-with-wrong-ova-fileserver-url.log
    Should Be True  ${passed}

Install Plugin Successfully
    Set Fileserver And Thumbprint In Configs
    Append To File  ${UI_INSTALLER_PATH}/configs  BYPASS_PLUGIN_VERIFICATION=1\n
    Install Plugin Successfully  ${TEST_VC_IP}  ${TEST_VC_USERNAME}  ${TEST_VC_PASSWORD}  ${TRUE}
    ${output}=  OperatingSystem.GetFile  install.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  exited successfully
    Run Keyword Unless  ${passed}  Move File  install.log  install-fail-ensure-vicui-is-installed-before-testing.log
    Should Be True  ${passed}
