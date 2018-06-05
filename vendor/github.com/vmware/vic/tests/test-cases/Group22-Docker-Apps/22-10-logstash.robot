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
Documentation  Test 22-10 - logstash
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Check logstash logs
    [Arguments]  ${container}  ${message}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${container} 
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${message}

*** Test Cases ***
Logstash with stdin and stdout mapped
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name log1 -dit logstash -e 'input { stdin { } } output { stdout { } }'
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check logstash logs  log1  Successfully started Logstash API endpoint

Logstash with mapped volume log file
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=vol1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --rm -v vol1:/mydata ${busybox} sh -c "echo 'Initial log message' > /mydata/my.log"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${logstash_container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit -v vol1:/logs logstash -e 'input { file { path => "/logs/my.log" start_position => "beginning" } } output { stdout { } }'
    Log  ${logstash_container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check logstash logs  ${logstash_container}  Initial log message

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${logstash_container} sh -c "echo 'Another exciting log message' >> /logs/my.log"
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check logstash logs  ${logstash_container}  Another exciting log message
