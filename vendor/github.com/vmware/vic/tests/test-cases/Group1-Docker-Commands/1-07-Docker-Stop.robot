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
Documentation  Test 1-07 - Docker Stop
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Trap Signal Command
    # Container command runs an infinite loop, trapping and logging the given signal name
    [Arguments]  ${sig}
    [Return]  ${busybox} sh -c "trap 'echo StopSignal${sig}' ${sig}; echo READY; while true; do sleep 1; done"

Assert Ready
    # Assert the docker stop signal trap has been set
    [Arguments]  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${id}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  READY

Assert Stop Signal
    # Assert the docker stop signal was trapped by checking the container output log file
    [Arguments]  ${id}  ${sig}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${id}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  StopSignal${sig}

Assert Kill Signal
    # Assert SIGKILL was sent or not by checking the tether debug log file
    [Arguments]  ${id}  ${expect}
    ${vmName}=  Get VM display name  ${id}
    Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Set Test Variable  ${id}  ${vmName}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -json ${vmName} | jq -r .VirtualMachines[].Runtime.PowerState
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  poweredOff

    ${rc}  ${dir}=  Run And Return Rc And Output  govc datastore.ls ${id}*
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  govc datastore.download ${dir}/tether.debug -
    Should Be Equal As Integers  ${rc}  0
    Run Keyword If  ${expect}  Should Contain  ${output}  sending signal KILL
    Run Keyword Unless  ${expect}  Should Not Contain  ${output}  sending signal KILL

*** Test Cases ***
Stop an already stopped container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} ls
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0

Basic docker container stop
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} sleep 30
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    Assert Kill Signal  ${container}  False

Basic docker stop w/ unclean exit from running process
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${container} | jq '.[]|.["State"]|.["ExitCode"]'
    Should Be Equal As Integers  ${rc}  0
    ${status}=  Get State Of Github Issue  6614
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-07-Docker-Stop.robot needs to be updated now that Issue #6614 has been resolved
    #Should Be Equal As Integers  ${output}  143

Stop a container with SIGKILL using default grace period
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Trap Signal Command  HUP
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${trap}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Ready  ${container}
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    Assert Kill Signal  ${container}  False

Stop a container with SIGKILL using specific stop signal
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Trap Signal Command  USR1
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --stop-signal USR1 ${trap}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Ready  ${container}
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    Assert Stop Signal  ${container}  USR1
    Assert Kill Signal  ${container}  True

Stop a container with SIGKILL using specific grace period
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Trap Signal Command  HUP
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --stop-signal HUP ${trap}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Assert Ready  ${container}
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} stop -t 2 ${container}
    Should Be Equal As Integers  ${rc}  0
    Assert Stop Signal  ${container}  HUP
    Assert Kill Signal  ${container}  True

Stop a non-existent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop fakeContainer
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: No such container: fakeContainer

Attempt to stop a container that has been started out of band
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${name}=  Generate Random String  15
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0

    Power On VM OOB  ${name}-*
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  3s  Assert Kill Signal  ${container}  False

Restart a stopped container
    ${status}=  Get State Of Github Issue  6700
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-07-Docker-Stop.robot needs to be updated now that Issue #6700 has been resolved
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it ${busybox} /bin/ls
    #Should Be Equal As Integers  ${rc}  0
    #Should Not Contain  ${output}  Error:
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${output}
    #Should Be Equal As Integers  ${rc}  0
    #Should Not Contain  ${output}  Error:
    #${shortID}=  Get container shortID  ${output}
    #Wait Until VM Powers Off  *-${shortID}
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${output}
    #Should Be Equal As Integers  ${rc}  0
    #Should Not Contain  ${output}  Error:

Stop a container with Docker 1.13 CLI
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${trap}=  Trap Signal Command  HUP
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} stop ${container}
    Should Be Equal As Integers  ${rc}  0
