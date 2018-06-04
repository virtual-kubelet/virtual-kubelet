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
Documentation  Test 1-28 - Docker Secret
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Variables ***
${fake-secret}  test

*** Test Cases ***
Docker secret ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} secret ls
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker secret create
    Run  echo '${fake-secret}' > secret.file
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} secret create mysecret ./secret.file
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker secret inspect
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} secret inspect my_secret
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker secret rm
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} secret rm my_secret
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported
	
