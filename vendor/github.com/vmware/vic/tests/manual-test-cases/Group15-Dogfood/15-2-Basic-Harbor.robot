# Copyright 2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

*** Settings ***
Documentation  Test 15-2 - Basic Harbor
Resource  ../../resources/Util.robot
Test Teardown  Harbor Test Cleanup

*** Keywords ***
Harbor Test Cleanup
    Cleanup VIC Appliance On Test Server
    ${out}=  Run  govc vm.destroy harbor

Pull image
    [Arguments]  ${image}
    Log To Console  \nRunning docker pull ${image}...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Digest:
    Should Contain  ${output}  Status:
    Should Not Contain  ${output}  No such image:

*** Test Cases ***
Basic Harbor Install
    Install Harbor To Test Server
    Install VIC Appliance To Test Server  vol=default --insecure-registry %{HARBOR_IP}
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  %{HARBOR_IP}/library/photon:1.0
