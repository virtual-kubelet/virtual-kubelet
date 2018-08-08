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
Documentation  Test 1-29 - Docker Checkpoint
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Docker checkpoint create
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} create --name=test-busybox ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} checkpoint create test-busybox new-checkpoint
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  vSphere Integrated Containers does not yet implement checkpointing

Docker checkpoint ls
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} checkpoint ls test-busybox
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  vSphere Integrated Containers does not yet implement checkpointing

Docker checkpoint rm
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} checkpoint rm test-busybox new-checkpoint
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such container
