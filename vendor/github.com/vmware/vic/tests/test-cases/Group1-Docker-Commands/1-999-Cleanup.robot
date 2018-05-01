# Copyright 2017 VMware, Inc. All Rights Reserved.
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
Documentation  Test 1-100 - Cleanup
Resource  ../../resources/Util.robot
Suite Setup  Cleanup Docker Commands Tests
Suite Teardown  Finalize Docker Commands Tests
Test Timeout  20 minutes

*** Keywords ***
Is Single VCH Mode
    # If TARGET_VCH not present when running group, track that we're installing a single VCH here.
    # This allows the cleanup suite to know that it can clean up the VCH
    ${multi-vch}=  Get Environment Variable  MULTI_VCH  ${EMPTY}

    ${single-vch-mode}=  Run Keyword If  '${multi-vch}' == '1'  Set Variable  ${False}
    ...  ELSE  Set Variable  ${True}

    [Return]  ${single-vch-mode}

Cleanup Docker Commands Tests
    Log To Console  Cleaning up docker command tests

    ${single-vch-mode}=  Run Keyword  Is Single VCH Mode

    Run Keyword If  ${single-vch-mode} == ${True}  Conditional Install VIC Appliance To Test Server

Finalize Docker Commands Tests
    Log To Console  Finalizing up docker command tests

    ${single-vch-mode}=  Run Keyword  Is Single VCH Mode

    Run Keyword If  ${single-vch-mode} == ${True}  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Prepare to Cleanup Group
    ${single-vch-mode}=  Run Keyword  Is Single VCH Mode
 
    # Helps cleans up the Group 1 test suite instead of solely depending on vic-machine
    Run Keyword If  ${single-vch-mode} == ${True}  Remove All Containers
    Run Keyword If  ${single-vch-mode} == ${True}  Remove All Images

    # This step allows the teardown to remove the VCH.  This only occurs if CLEANUP_GROUP was set
    # by an initializer suite.
    ${cleanup-group}=  Get Environment Variable  CLEANUP_GROUP  ${EMPTY}
    Run Keyword If  ${single-vch-mode} == ${True}  Run Keyword If  '${cleanup-group}' == '1'  Remove VCH From Removal Exception List  vch=%{VCH-NAME}  
    