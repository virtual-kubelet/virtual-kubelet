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
Documentation  Test 1-31 - Docker Node
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Docker node demote
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node demote self
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such node: self

Docker node ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node ls
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker node promote
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node promote self
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such node: self

Docker node rm
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node rm self
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Docker Swarm is not yet supported

Docker node update
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node update self
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such node: self

Docker node ps
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node ps
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such node

Docker node inspect
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} node inspect self
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such node

