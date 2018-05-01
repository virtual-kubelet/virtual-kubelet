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
Documentation  Test 1-14 - Docker Kill
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Trap Signal Command
    # Container command runs an infinite loop, trapping and logging the given signal name
    [Arguments]  ${sig}
    [Return]  ${busybox} sh -c "trap 'echo KillSignal${sig}' ${sig}; echo READY; while true; do date && sleep 1; done"

Nested Trap Signal Command
    # Container command runs an infinite loop, trapping and logging the given signal name in a nested shell
    # This is to test process group behaviours - same command as above, but nested in another shell
    [Arguments]  ${sig}
    [Return]  ${busybox} sh -c "trap 'echo KillSignalParent${sig}' ${sig}; sh -c \\"trap 'echo KillSignalChild${sig}' ${sig}; echo READY; while true; do date && sleep 1; done\\""

Assert Container Output
    [Arguments]  ${id}  ${match}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${id}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${match}

Check That Container Was Killed
    [Arguments]  ${container}
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f {{.State.Running}} ${container}
    Log  ${out}
    Should Contain  ${out}  false
    Should Be Equal As Integers  ${rc}  0

*** Test Cases ***
Signal a container with default kill signal
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Trap Signal Command  HUP
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${trap}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Container Output  ${id}  READY
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill ${id}
    Should Be Equal As Integers  ${rc}  0
    # Wait for container VM to stop/powerOff
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${id}
    # Cannot send signal to a powered off container VM
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill ${id}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Cannot kill container ${id}

Signal a container with SIGHUP
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Trap Signal Command  HUP
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${trap}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Container Output  ${id}  READY
    # Expect failure with unknown signal name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s NOPE ${id}
    Should Be Equal As Integers  ${rc}  1
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s HUP ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Container Output  ${id}  KillSignalHUP
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s TERM ${id}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${id}

Confirm signal delivered to entire process group
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Nested Trap Signal Command  HUP
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${trap}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Container Output  ${id}  READY
    # Expect failure with unknown signal name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s NOPE ${id}
    Should Be Equal As Integers  ${rc}  1
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s HUP ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Container Output  ${id}  KillSignalChildHUP
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s TERM ${id}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${id}

Signal a non-existent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill fakeContainer
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No such container: fakeContainer

Signal a tough to kill container - nginx
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${nginx}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${nginx}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check That Container Was Killed  ${id}
