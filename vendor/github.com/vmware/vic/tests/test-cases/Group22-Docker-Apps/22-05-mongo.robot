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
Documentation  Test 22-05 - mongo
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Check mongo container
    [Arguments]  ${ip}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --rm mongo sh -c 'mongo "${ip}/27017" --quiet --eval "db.adminCommand( { listDatabases: 1 } )"'
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  "name" : "admin"
    Should Contain  ${output}  "name" : "local"

*** Test Cases ***
Simple background mongo
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name mongo1 -d mongo
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get IP Address of Container  mongo1
    Wait Until Keyword Succeeds  5x  6s  Check mongo container  ${ip}
