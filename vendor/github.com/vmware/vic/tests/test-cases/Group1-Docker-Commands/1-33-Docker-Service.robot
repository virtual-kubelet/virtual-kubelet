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
Documentation  Test 1-33 - Docker Service
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Docker service create 
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service create test-service
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker service ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service ls
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker service ps
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service ps test-service
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such service: test-service

Docker serivce rm
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service rm test-service
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker service scale
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service scale test-service=3
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such service: test-service

Docker service update
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service update test-service
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such service: test-service

Docker service logs
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} service logs test
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  only supported with experimental daemon

