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
Documentation  Test 6-12 - Verify vic-machine version command
Resource  ../../resources/Util.robot
Test Timeout  20 minutes

*** Test Cases ***
VIC-machine - Version check
    Set Test Environment Variables

    ${output}=  Run  bin/vic-machine-linux version
    @{gotVersion}=  Split String  ${output}  ${SPACE}
    ${version}=  Remove String  @{gotVersion}[2]
    Log To Console  VIC machine version: ${version}
    
    ${result}=  Run  git rev-parse HEAD
    @{gotVersion}=  Split String  ${result}  ${SPACE}
    ${commithash}=  Remove String  @{gotVersion}[0]
    
    Log To Console  Last commit hash from git: ${commithash}

    ${hash_result} =    Fetch From Right  ${version}  -
    Log To Console  Commit Hash from vic-machine version: ${hash_result}

    Should Contain  ${commithash}  ${hash_result}
