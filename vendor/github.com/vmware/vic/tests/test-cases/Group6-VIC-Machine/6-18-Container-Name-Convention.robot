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
Documentation  Test 6-18 - Container Name Convention
Resource  ../../resources/Util.robot
Test Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Container name convention with id
    Set Test Environment Variables
    Install VIC Appliance To Test Server With Current Environment Variables  additional-args=--container-name-convention %{VCH-NAME}-{id}
    Run  docker %{VCH-PARAMS} pull ${busybox}
    ${containerID}=  Run  docker %{VCH-PARAMS} run -d ${busybox}
    ${shortId}=  Get container shortID  ${containerID}
    ${output}=  Run  govc ls vm/%{VCH-NAME}
    Should Contain  ${output}  %{VCH-NAME}-${shortID}

    Run  docker %{VCH-PARAMS} rename ${containerID} renamed-container
    ${output}=  Run  govc ls vm/%{VCH-NAME}
    # confirm that the cnc is still in force
    Should Contain  ${output}  %{VCH-NAME}-${shortID}

    Run  docker %{VCH-PARAMS} rm -f ${containerID}
    Run Regression Tests
 
Container name convention with name
    Set Test Environment Variables
    Install VIC Appliance To Test Server With Current Environment Variables  additional-args=--container-name-convention %{VCH-NAME}-{name}
    Run  docker %{VCH-PARAMS} pull ${busybox}
    ${containerID}=  Run  docker %{VCH-PARAMS} run -d ${busybox}
    ${name}=  Get container name  ${containerID}
    ${output}=  Run  govc ls vm/%{VCH-NAME}
    Should Contain  ${output}  %{VCH-NAME}-${name}

    Run  docker %{VCH-PARAMS} rename ${containerID} renamed-container
    ${output}=  Run  govc ls vm/%{VCH-NAME}
    # confirm that the cnc is still in force but updated for new container name
    Should Contain  ${output}  %{VCH-NAME}-renamed-container

    Run  docker %{VCH-PARAMS} rm -f ${containerID}
    Run Regression Tests

Container name convention with invalid argument
    ${rc}  ${output}=  Run Keyword And Ignore Error  Install VIC Appliance To Test Server  additional-args=--container-name-convention 192.168.1.1-mycontainer
    Should Contain  ${output}  Container name convention must include {id} or {name} token
    [Teardown]  Log To Console  Test passed no need to run cleanup
