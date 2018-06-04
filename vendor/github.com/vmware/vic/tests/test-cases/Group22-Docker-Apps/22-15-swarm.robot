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
Documentation  Test 22-15 - swarm
Resource  ../../resources/Util.robot
#Suite Setup  Install VIC Appliance To Test Server
#Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Create a docker swarm
    ${status}=  Get State Of Github Issue  6396
    Run Keyword If  '${status}' == 'closed'  Fail  Test 22-15-swarm.robot needs to be updated now that Issue #6396 has been resolved

#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull swarm
#    Should Be Equal As Integers  ${rc}  0

#    ${rc}  ${cluster}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --rm swarm create
#    Log  ${cluster}
#    Should Be Equal As Integers  ${rc}  0

#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d swarm join --addr=172.16.0.20:2375 token://${cluster}
#    Log  ${output}
#    Should Be Equal As Integers  ${rc}  0
#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d swarm join --addr=172.16.0.21:2375 token://${cluster}
#    Log  ${output}
#    Should Be Equal As Integers  ${rc}  0
#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d swarm join --addr=172.16.0.22:2375 token://${cluster}
#    Log  ${output}
#    Should Be Equal As Integers  ${rc}  0

#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -t -p 2380:2375 -d swarm manage token://${cluster}
#    Log  ${output}
#    Should Be Equal As Integers  ${rc}  0

#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-IP}:2380 info
#    Log  ${output}
#    Should Be Equal As Integers  ${rc}  0

#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-IP}:2380 run busybox ls
#    Log  ${output}
#    Should Be Equal As Integers  ${rc}  0
