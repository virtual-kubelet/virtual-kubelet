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
Documentation  Group 1 Shared VCH Initializer
Resource  ../../resources/Util.robot
Suite Setup  Setup Docker Commands Tests
Test Timeout  20 minutes

*** Keywords ***
Setup Docker Commands Tests
    # If TARGET_VCH not present when running group, track that we're installing a single VCH here.
    # This allows the cleanup suite to know that it can clean up the VCH
    ${target-vch}=  Get Environment Variable  TARGET_VCH  ${EMPTY}
    ${multi-vch}=  Get Environment Variable  MULTI_VCH  ${EMPTY}

    ${single-vch-mode}=  Run Keyword If  '${multi-vch}' == '1'  Set Variable  ${False}
    ...  ELSE  Set Variable  ${True}

    Set Environment Variable  CLEANUP_GROUP  0

    Run Keyword If  '${target-vch}' == '${EMPTY}'  Run Keyword If  ${single-vch-mode} == ${True}  Set Environment Variable  CLEANUP_GROUP  1

    Run Keyword If  '${target-vch}' == '${EMPTY}'  Run Keyword If  ${single-vch-mode} != ${True}  Log To Console  Bypassing shared VCH install because tests are in multi-VCH mode.
    Run Keyword If  '${target-vch}' != '${EMPTY}'  Log To Console  Detected Target VCH ${target-vch}
    Run Keyword If  ${single-vch-mode} == ${True}  Conditional Install VIC Appliance To Test Server  init=${True}
