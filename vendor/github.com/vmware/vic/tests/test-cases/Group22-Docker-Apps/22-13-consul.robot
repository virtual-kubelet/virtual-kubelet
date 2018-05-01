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
Documentation  Test 22-13 - consul
Resource  ../../resources/Util.robot
#Suite Setup  Install VIC Appliance To Test Server
#Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Check consul members
    [Arguments]  ${server}  ${count}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -t ${server} consul members
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain X Times  ${output}  alive  ${count}

*** Test Cases ***
Multi-agent consul topology
    ${status}=  Get State Of Github Issue  6393
    Run Keyword If  '${status}' == 'closed'  Fail  Test 22-13-consul.robot needs to be updated now that Issue #6393 has been resolved
    ${status}=  Get State Of Github Issue  6394
    Run Keyword If  '${status}' == 'closed'  Fail  Test 22-13-consul.robot needs to be updated now that Issue #6394 has been resolved
    ${status}=  Get State Of Github Issue  6395
    Run Keyword If  '${status}' == 'closed'  Fail  Test 22-13-consul.robot needs to be updated now that Issue #6395 has been resolved
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name=dev-consul -e CONSUL_BIND_INTERFACE=eth0 consul
    #Log  ${output}
    #Should Be Equal As Integers  ${rc}  0
    #${ip}=  Get IP Address of Container  dev-consul

    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -e CONSUL_BIND_INTERFACE=eth0 consul agent -dev -join=${ip}
    #Log  ${output}
    #Should Be Equal As Integers  ${rc}  0
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -e CONSUL_BIND_INTERFACE=eth0 consul agent -dev -join=${ip}
    #Log  ${output}
    #Should Be Equal As Integers  ${rc}  0

    #Wait Until Keyword Succeeds  10x  6s  Check consul members  dev-consul  3