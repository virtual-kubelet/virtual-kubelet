# Copyright 2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

*** Settings ***
Documentation  Test 11-03 - Upgrade-UpdateInProgress
Suite Setup  Install VIC with version to Test Server  v1.3.1
Suite Teardown  Clean up VIC Appliance And Local Binary
Resource  ../../resources/Util.robot

*** Test Cases ***
Upgrade VCH with UpdateInProgress
    Run  govc vm.change -vm=%{VCH-NAME} -e=UpdateInProgress=true
    Check UpdateInProgress  true
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
    Should Contain  ${output}  Upgrade failed: another upgrade/configure operation is in progress
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --reset-progress --name=%{VCH-NAME} --target=%{TEST_URL} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
    Should Contain  ${output}  Reset UpdateInProgress flag successfully
    Check UpdateInProgress  false

Upgrade and inspect VCH
    Start Process  bin/vic-machine-linux upgrade --debug 1 --name %{VCH-NAME} --target %{TEST_URL} --user %{TEST_USERNAME} --password %{TEST_PASSWORD} --force --compute-resource %{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}  shell=True  alias=UpgradeVCH
    Wait Until Keyword Succeeds  20x  5s  Inspect VCH   Upgrade/configure in progress
    Wait For Process  UpgradeVCH
    Inspect VCH  Completed successfully
    Check UpdateInProgress  false
