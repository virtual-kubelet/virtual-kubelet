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
Documentation  Test 1-38 - Docker Exec
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Verify Poweroff During Exec Error Message
       [Arguments]  ${error}  ${containerID}  ${containerName}
       Set Test Variable  ${msg1}  Container (${containerName}) is not running
       Set Test Variable  ${msg2}  container (${containerID}) has been stopped
       Set Test Variable  ${msg3}  Unable to wait for task when container ${containerID} is not running
       Set Test Variable  ${msg4}  the Container(${containerID}) has been shutdown during execution of the exec operation
       Set Test Variable  ${msg5}  container(${containerID}) must be powered on in order to perform the desired exec operation
       Set Test Variable  ${msg6}  the container has been stopped

       Should Contain Any  ${error}  ${msg1}  ${msg2}  ${msg3}  ${msg4}  ${msg5}

Verify No Poweroff During Exec Error Message
       [Arguments]  ${error}  ${containerID}  ${containerName}
       Set Test Variable  ${msg1}  Container (${containerName}) is not running
       Set Test Variable  ${msg2}  container (${containerID}) has been stopped
       Set Test Variable  ${msg3}  Unable to wait for task when container ${containerID} is not running
       Should Not Contain Any  ${error}  ${msg1}  ${msg2}  ${msg3}

Verify LS Output For Busybox
       [Arguments]  ${output}
       Should Contain  ${output}  bin
       Should Contain  ${output}  dev
       Should Contain  ${output}  etc
       Should Contain  ${output}  home
       Should Contain  ${output}  lib
       Should Contain  ${output}  lost+found
       Should Contain  ${output}  mnt
       Should Contain  ${output}  proc
       Should Contain  ${output}  root
       Should Contain  ${output}  run
       Should Contain  ${output}  sbin
       Should Contain  ${output}  sys
       Should Contain  ${output}  tmp
       Should Contain  ${output}  usr
       Should Contain  ${output}  var

Wait Until Detached Exec Occurs
     [Arguments]  ${name}
     :FOR  ${idx}  IN RANGE  1  5
     \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/ls -al /tmp/force
     \   ${Status1}=  Run Keyword And Return Status  Should Be Equal As Integers  ${rc}  0
     \   ${Status2}=  Run Keyword And Return Status  Should Contain  ${output}  force
     \   Return From Keyword If  ${status1} & ${status2}
     \   Sleep  2s
     Fail  Detached exec did not succeed. It either took to long, or failed.

*** Test Cases ***

Standard Exec Exit Codes
    ${name}=  Set Variable  'exit-codes'
    ${rc} =  Run And Return Rc  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0

    # rigorously test exit codes for a standard docker exec.
    :For  ${idx}  IN RANGE  1  10
    \    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/sh -c \"exit ${idx}\"
    \    Should Be Equal As Integers  ${rc}  ${idx}

    # some other tests using true and false
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/false
    Should Be Equal As Integers  ${rc}  1

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/true
    Should Be Equal As Integers  ${rc}  0

Exec -d
    # Confirm that a proper exec -d does indeed do as it should.
    ${name}=  Set Variable  'detach-test'
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -d ${name} /bin/touch tmp/force
    Should Be Equal As Integers  ${rc}  0

    Wait Until Detached Exec Occurs  ${name}

Exec Echo
    ${name}=  Set Variable  'echo-test'
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0
    :FOR  ${idx}  IN RANGE  0  5
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/echo "Help me, Obi-Wan Kenobi. You're my only hope."
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Be Equal As Strings  ${output}  Help me, Obi-Wan Kenobi. You're my only hope.

Exec Echo -i
    ${name}=  Set Variable  'interactive-echo-test'
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0
    :FOR  ${idx}  IN RANGE  0  5
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -i ${name} /bin/echo "Your eyes can deceive you. Don't trust them."
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Be Equal As Strings  ${output}  Your eyes can deceive you. Don't trust them.

Exec Echo -t
    ${name}=  Set Variable  'tty-echo-test'
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0
    :FOR  ${idx}  IN RANGE  0  5
    \   ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -t ${name} /bin/echo "Do. Or do not. There is no try."
    \   Should Be Equal As Integers  ${rc}  0
    \   Should Be Equal As Strings  ${output}  Do. Or do not. There is no try.

Exec Sort
    # setup filesystem for the test
    ${rc}  ${tmp}=  Run And Return Rc And Output  mktemp -d -p /tmp
    Should Be Equal As Integers  ${rc}  0
    ${fifo}=  Catenate  SEPARATOR=/  ${tmp}  fifo
    ${rc}  ${output}=  Run And Return Rc And Output  mkfifo ${fifo}
    Should Be Equal As Integers  ${rc}  0

    ${name}=  Set Variable  'sort-test'
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0
    :FOR  ${idx}  IN RANGE  0  5
    \     Start Process  docker %{VCH-PARAMS} exec ${name} /bin/sort < ${fifo}  shell=True  alias=custom
    \     Run  echo one > ${fifo}
    \     ${ret}=  Wait For Process  custom
    \     Log  ${ret.stderr}
    \     Should Be Empty  ${ret.stdout}
    \     Should Be Equal As Integers  ${ret.rc}  0
    \     Should Be Empty  ${ret.stderr}
    Run  rm -rf ${tmp}

Exec Sort -i
    # Setup filesystem for the test
    ${rc}  ${tmp}=  Run And Return Rc And Output  mktemp -d -p /tmp
    Should Be Equal As Integers  ${rc}  0
    ${fifo}=  Catenate  SEPARATOR=/  ${tmp}  fifo
    ${rc}  ${output}=  Run And Return Rc And Output  mkfifo ${fifo}
    Should Be Equal As Integers  ${rc}  0

    ${name}=  Set Variable  'interactive-sort-test'
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0
    :FOR  ${idx}  IN RANGE  0  5
    \     Start Process  docker %{VCH-PARAMS} exec -i ${name} /bin/sort < ${fifo}  shell=True  alias=custom
    \     Run  echo one > ${fifo}
    \     ${ret}=  Wait For Process  custom
    \     Log  ${ret.stderr}
    \     Should Be Equal  ${ret.stdout}  one
    \     Should Be Equal As Integers  ${ret.rc}  0
    \     Should Be Empty  ${ret.stderr}
    Run  rm -rf ${tmp}

Exec NonExisting
    ${name}=  Set Variable  'does-not-exist'
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0

    # standard case
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /NonExisting
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  no such file or directory

    # detach error case
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -d ${name} /does/not/exist
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${output}  no such executable

Exec Permission Denied
     ${name}=  Set Variable  'exec-permission-denied'
     ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
     Should Be Equal As Integers  ${rc}  0

     # standard case
     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} touch /bin/fake
     Should Be Equal As Integers  ${rc}  0

     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/fake
     Should Be Equal As Integers  ${rc}  126
     Should Contain  ${output}  permission denied

     # detach error case
     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -d ${name} /bin/fake
     Should Be Equal As Integers  ${rc}  1

Exec Non Binary
     ${name}=  Set Variable  'exec-non-binary'
     ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name ${name} ${busybox} /bin/top -d 600
     Should Be Equal As Integers  ${rc}  0

     # standard case
     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} touch /bin/fake
     Should Be Equal As Integers  ${rc}  0

     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} chmod 777 /bin/fake
     Should Be Equal As Integers  ${rc}  0

     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${name} /bin/fake
     Should Be Equal As Integers  ${rc}  126

     # detach error case
     ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -d ${name} /bin/fake
     Should Be Equal As Integers  ${rc}  1

Concurrent Simple Exec
     ${status}=  Get State Of Github Issue  7410
     Run Keyword If  '${status}' == 'closed'  Fail  Test 1-38-Docker-Exec.robot needs to be updated now that Issue #7410 has been resolved
     # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
     # Should Be Equal As Integers  ${rc}  0
     # Should Not Contain  ${output}  Error

     # ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
     # Set Test Variable  ${ExecSimpleContainer}  Exec-simple-${suffix}
     # ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --name ${ExecSimpleContainer} ${busybox} /bin/top
     # Should Be Equal As Integers  ${rc}  0

     # :FOR  ${idx}  IN RANGE  1  3
     # \   Start Process  docker %{VCH-PARAMS} exec ${id} /bin/ls  alias=exec-simple-%{VCH-NAME}-${idx}  shell=true

     # :FOR  ${idx}  IN RANGE  1  3
     # \   ${result}=  Wait For Process  exec-simple-%{VCH-NAME}-${idx}  timeout=40s
     # \   Should Be Equal As Integers  ${result.rc}  0
     # \   Verify LS Output For Busybox  ${result.stdout}
     # # stop the container now that we have a successful series of concurrent execs
     # ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} stop ${id}
     # Should Be Equal As Integers  ${rc}  0


Exec During Poweroff Of A Container Performing A Long Running Task
     ${status}=  Get State Of Github Issue  7410
     Run Keyword If  '${status}' == 'closed'  Fail  Test 1-38-Docker-Exec.robot needs to be updated now that Issue #7410 has been resolved
     # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
     # Should Be Equal As Integers  ${rc}  0
     # Should Not Contain  ${output}  Error

     # ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
     # Set Test Variable  ${ExecPowerOffContainerLong}  Exec-Poweroff-${suffix}
     # ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --name ${ExecPoweroffContainerLong} ${busybox} /bin/top
     # Should Be Equal As Integers  ${rc}  0

     # :FOR  ${idx}  IN RANGE  1  15
     # \   Start Process  docker %{VCH-PARAMS} exec ${id} /bin/ls  alias=exec-%{VCH-NAME}-${idx}  shell=true


     # Sleep  1s
     # ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${id}
     # Should Be Equal As Integers  ${rc}  0

     # ${combinedErr}=  Set Variable
     # ${combinedOut}=  Set Variable

     # :FOR  ${idx}  IN RANGE  1  15
     # \   ${result}=  Wait For Process  exec-%{VCH-NAME}-${idx}  timeout=2 mins
     # \   ${combinedErr}=  Catenate  ${combinedErr}  ${result.stderr}${\n}
     # \   ${combinedOut}=  Catenate  ${combinedOut}  ${result.stdout}${\n}

     # # We combine err and out into err since exec can return errors on both.
     # ${combinedErr}=  Catenate  ${combinedErr}  ${combinedOut}
     # Verify Poweroff During Exec Error Message  ${combinedErr}  ${id}  ${ExecPowerOffContainerLong}

Exec During Poweroff Of A Container Performing A Short Running Task
     ${status}=  Get State Of Github Issue  7410
     Run Keyword If  '${status}' == 'closed'  Fail  Test 1-38-Docker-Exec.robot needs to be updated now that Issue #7410 has been resolved
     # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
     # Should Be Equal As Integers  ${rc}  0
     # Should Not Contain  ${output}  Error

     # ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
     # Set Test Variable  ${ExecPoweroffContainerShort}  Exec-Poweroff-${suffix}
     # ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --name ${ExecPoweroffContainerShort} ${busybox} sleep 20
     # Should Be Equal As Integers  ${rc}  0

     # ## the /bin/top should stay open the entire life of the container from start of the exec.
     # ${rc}  ${output}=  Run And Return Rc And output  docker %{VCH-PARAMS} exec ${id} /bin/top
     # Should Be Equal As Integers  ${rc}  0

     # # We should see tether every time since it is required to run the container.
     # Should Contain  ${output}  /.tether/tether
     # Verify No Poweroff During Exec Error Message  ${output}  ${id}  ${ExecPoweroffContainerShort}
