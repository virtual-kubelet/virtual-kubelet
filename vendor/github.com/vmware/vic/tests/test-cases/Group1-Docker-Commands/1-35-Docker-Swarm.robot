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
Documentation  Test 1-35 - Docker Swarm
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Docker swarm init
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm init
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker swarm join
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm join 127.0.0.1:2375
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker swarm join-token
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm join-token worker
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm join-token manager
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker swarm leave
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm leave
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker swarm unlock-key
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm unlock-key
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker swarm update
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} swarm update --autolock
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

