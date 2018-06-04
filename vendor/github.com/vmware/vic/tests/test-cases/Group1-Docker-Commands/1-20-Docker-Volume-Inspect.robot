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
Documentation  Test 1-20 - Docker Volume Inspect
Resource  ../../resources/Util.robot
Suite Setup  Run Keywords  Conditional Install VIC Appliance To Test Server  Remove All Volumes
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Simple docker volume inspect
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name test
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume inspect test
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Evaluate  json.loads(r'''${output}''')  json
    ${id}=  Get From Dictionary  ${output[0]}  Name
    Should Be Equal As Strings  ${id}  test

Docker volume inspect invalid object
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume inspect fakeVolume
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error: No such volume: fakeVolume