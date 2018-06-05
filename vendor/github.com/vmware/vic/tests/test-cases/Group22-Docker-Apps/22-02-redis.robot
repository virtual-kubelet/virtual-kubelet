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
Documentation  Test 22-02 - redis
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Simple background redis
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name redis1 -d ${redis}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get IP Address of Container  redis1
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${redis} sh -c "redis-cli -h ${ip} -p 6379 ping"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  PONG

Redis with appendonly option
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name redis2 -d ${redis} redis-server --appendonly yes
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get IP Address of Container  redis2
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${redis} sh -c "redis-cli -h ${ip} -p 6379 info"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  redis_mode:standalone
    Should Contain  ${output}  executable:/data/redis-server
    