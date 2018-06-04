# Copyright 2017-2018 VMware, Inc. All Rights Reserved.
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
Documentation   Test 1-40 - Docker Restart
Resource        ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  30 minutes

*** Keywords ***

Create test containers
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d busybox /bin/top
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  RUNNER  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create busybox /bin/top
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  CREATOR  ${output}

*** Test Cases ***
Restart Running Container
    Create test containers
    # grab the containerVM ip address - will compare after restart to ensure it remains the same
    ${rc}  ${originalIP}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${originalIP}  172
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} restart %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  %{RUNNER}
    # verify that IP address didn't chnage during restart
    ${rc}  ${restartIP}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${originalIP}  ${restartIP}
    # verify that the containerVM started
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -f id=%{RUNNER} -f status=running
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    Length Should Be  ${output}  2

Restart Created Container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} restart %{CREATOR}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  %{CREATOR}
    # verify that the containerVM started
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -f id=%{CREATOR} -f status=running
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    Length Should Be  ${output}  2

Restart Stopped Container
    # grab the containerVM ip address - will compare after restart to ensure it remains the same
    ${rc}  ${originalIP}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${originalIP}  172
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    # verify that the containerVM exited
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -f id=%{RUNNER} -f status=exited
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    Length Should Be  ${output}  2
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} restart %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  %{RUNNER}
    # verify that the containerVM started
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -f id=%{RUNNER} -f status=running
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    Length Should Be  ${output}  2
    # verify that IP address didn't chnage during restart
    ${rc}  ${restartIP}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' %{RUNNER}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${originalIP}  ${restartIP}

Restart with start-stop stress
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit ${busybox}
    # Set the timeout to 5s - this should be enough to allow the docker stop to pre-empt the stop portion of restart
    # but not so long we have massive variance based on which process wins the race for each iteration
    ${restart-pid}=  Start Process  while true; do docker %{VCH-PARAMS} restart -t 5 ${container}; done  shell=${true}
    ${restart-pid2}=  Start Process  while true; do docker %{VCH-PARAMS} restart -t 5 ${container}; done  shell=${true}
    ${loopOutput}=  Create List
    :FOR  ${idx}  IN RANGE  0  150
    \   ${out}=  Run  (docker %{VCH-PARAMS} start ${container} && docker %{VCH-PARAMS} stop -t1 ${container})
    \   Append To List  ${loopOutput}  ${out}
    Terminate Process  ${restart-pid}
    Terminate Process  ${restart-pid2}
    Log  ${loopOutput}
    Should Not Contain Match  ${loopOutput}  *EOF*
