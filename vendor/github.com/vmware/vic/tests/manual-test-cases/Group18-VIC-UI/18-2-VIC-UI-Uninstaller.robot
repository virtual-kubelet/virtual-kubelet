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
Documentation  Test 18-2 - VIC UI Uninstallation
Resource  ../../resources/Util.robot
Resource  ./vicui-common.robot
Test Teardown  Cleanup Installer Environment
Suite Setup  Check Config And Install VCH  install
Suite Teardown  Uninstall VCH  ${TRUE}

*** Test Cases ***
Attempt To Uninstall With Configs File Missing
    # Rename the configs file and run the uninstaller script to see if it fails in an expected way
    Move File  ${UI_INSTALLER_PATH}/configs  ${UI_INSTALLER_PATH}/configs_renamed
    ${rc}  ${output}=  Run And Return Rc And Output  ${UNINSTALLER_SCRIPT_PATH}
    Run Keyword And Continue On Failure  Should Contain  ${output}  Configs file is missing
    Move File  ${UI_INSTALLER_PATH}/configs_renamed  ${UI_INSTALLER_PATH}/configs

Attempt To Uninstall With Plugin Manifest Missing
    Move File  ${UI_INSTALLER_PATH}/../plugin-manifest  ${UI_INSTALLER_PATH}/../plugin-manifest-a
    ${rc}  ${output}=  Run And Return Rc And Output  cd ${UI_INSTALLER_PATH} && ${UNINSTALLER_SCRIPT_PATH}
    Run Keyword And Continue On Failure  Should Contain  ${output}  manifest was not found
    Move File  ${UI_INSTALLER_PATH}/../plugin-manifest-a  ${UI_INSTALLER_PATH}/../plugin-manifest

Attempt To Uninstall From A Non vCenter Server
    Uninstall Fails  not-a-vcenter-server  admin  password
    ${output}=  OperatingSystem.GetFile  uninstall.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  vCenter Server was not found
    Run Keyword Unless  ${passed}  Move File  uninstall.log  uninstall-fail-attempt-to-uninstall-from-a-non-vcenter-server.log
    Should Be True  ${passed}

Attempt To Uninstall With Wrong Vcenter Credentials
    Set Fileserver And Thumbprint In Configs
    Uninstall Fails  ${TEST_VC_IP}  ${TEST_VC_USERNAME}_nope  ${TEST_VC_PASSWORD}_nope
    ${output}=  OperatingSystem.GetFile  uninstall.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  Cannot complete login due to an incorrect user name or password
    Run Keyword Unless  ${passed}  Move File  uninstall.log  uninstall-fail-attempt-to-uninstall-with-wrong-vcenter-credentials.log
    Should Be True  ${passed}

Uninstall Successfully
    Set Fileserver And Thumbprint In Configs
    Uninstall Vicui  ${TEST_VC_IP}  ${TEST_VC_USERNAME}  ${TEST_VC_PASSWORD}
    ${output}=  OperatingSystem.GetFile  uninstall.log
    ${passed}=  Run Keyword And Return Status  Should Match Regexp  ${output}  exited successfully
    Run Keyword Unless  ${passed}  Move File  uninstall.log  uninstall-fail-uninstall-successfully.log
    Should Be True  ${passed}

Attempt To Uninstall Plugin That Is Already Gone
    Set Fileserver And Thumbprint In Configs
    Uninstall Vicui  ${TEST_VC_IP}  ${TEST_VC_USERNAME}  ${TEST_VC_PASSWORD}
    ${output}=  OperatingSystem.GetFile  uninstall.log
    ${passed}=  Run Keyword And Return Status  Should Contain  ${output}  'com.vmware.vic.ui' is not registered
    ${passed2}=  Run Keyword And Return Status  Should Contain  ${output}  'com.vmware.vic' is not registered
    Run Keyword Unless  (${passed} and ${passed2})  Move File  uninstall.log  uninstall-fail-attempt-to-uninstall-plugin-that-is-already-gone.log
    Should Be True  ${passed}
