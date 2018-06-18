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
Documentation  Test 22-11 - memcache
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Check memcache status
    [Arguments]  ${port}
    ${result}=  Run Process  echo 'stats' | nc -q 3 %{VCH-IP} ${port}  shell=True
    Should Be Equal As Integers  ${result.rc}  0
    Should Contain  ${result.stdout}  STAT pid
    Should Contain  ${result.stdout}  STAT time_in_listen_disabled_us 0
    Should Contain  ${result.stdout}  STAT lru_bumps_dropped 0

*** Test Cases ***
Standard memcache container in background
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name my-memcache -d -p 11211:11211 memcached
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Wait Until Keyword Succeeds  10x  6s  Check memcache status  11211

Memcache container with additional memory
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name my-memcache2 -d -p 11212:11211 memcached memcached -m 64
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Wait Until Keyword Succeeds  10x  6s  Check memcache status  11212
