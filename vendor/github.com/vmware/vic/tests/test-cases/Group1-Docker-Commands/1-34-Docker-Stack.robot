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
Documentation  Test 1-34 - Docker Stack
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
#Docker stack deploy
    #${rc}  ${output}=  Run And Return Rc And Output  wget #https://raw.githubusercontent.com/vfarcic/docker-flow-proxy/master/docker-compose-stack.yml
    #Should Be Equal As Integers  ${rc}  0
    #${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} stack deploy -c #./docker-compose-stack.yml proxy
    #Should Be Equal As Integers  ${rc}  1
    #Should Contain  ${output}  Docker Swarm is not yet supported

Docker stack ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} stack ls
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker stack ps
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} stack ps test-stack
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker stack rm
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} stack rm test-stack
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker stack services
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} stack services test-stack
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported
