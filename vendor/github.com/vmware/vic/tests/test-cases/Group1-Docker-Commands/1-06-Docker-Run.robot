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
Documentation  Test 1-06 - Docker Run
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Make sure container starts
    :FOR  ${idx}  IN RANGE  0  60
    \   ${out}=  Run  docker %{VCH-PARAMS} ps
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${out}  /bin/top
    \   Return From Keyword If  ${status}
    \   Sleep  1
    Fail  Container failed to start

Verify container is running and remove it
    [Arguments]  ${containerName}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Should Contain  ${output}  ${containerName}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${containerName}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Verify container is removed
    [Arguments]  ${containerName}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a --format '{{.Names}}'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output2}=  Run And Return Rc And Output  govc find / -type m
    Should Be Equal As Integers  ${rc}  0
    # Verify docker persona cleaned up properly
    Should Not Contain  ${output}  ${containerName}
    # Verify that vSphere VMs were cleaned up properly
    Should Not Contain  ${output2}  ${containerName}

*** Test Cases ***
Simple docker run
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} /bin/ash -c "dmesg;echo END_OF_THE_TEST"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  END_OF_THE_TEST

Docker run with -t
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -t ${busybox} /bin/ash -c "dmesg;echo END_OF_THE_TEST"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  END_OF_THE_TEST

Simple docker run with app that doesn't exit
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs -n1 docker %{VCH-PARAMS} rm -f
    ${result}=  Start Process  docker %{VCH-PARAMS} run -d ${busybox} /bin/top  shell=True  alias=top

    Make sure container starts
    ${containerID}=  Run  docker %{VCH-PARAMS} ps -q
    ${out}=  Run  docker %{VCH-PARAMS} logs ${containerID}
    Should Contain  ${out}  Mem:
    Should Contain  ${out}  CPU:
    Should Contain  ${out}  Load average:
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs -n1 docker %{VCH-PARAMS} rm -f

Docker run fake command
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} fakeCommand
    Should Be True  ${rc} > 0
    Should Contain  ${output}  docker: Error response from daemon:
    Should Contain  ${output}  fakeCommand: no such executable in PATH.

Docker run fake image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run fakeImage /bin/bash
    Should Be True  ${rc} > 0
    Should Contain  ${output}  docker: Error parsing reference:
    Should Contain  ${output}  "fakeImage" is not a valid repository/tag

Docker run named container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name busy3 ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0

Docker run linked containers
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${debian}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --link busy3:busy3 ${debian} ping -c2 busy3
    Should Be Equal As Integers  ${rc}  0

Docker run -d unspecified host port
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -p 6379 redis:alpine
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Docker run check exit codes
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} true
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} false
    Should Be Equal As Integers  ${rc}  1

Docker run ps password check
    [Tags]  secret
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} ps auxww
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ps auxww
    ${output}=  Split To Lines  ${output}
    :FOR  ${line}  IN  @{output}
    \   ${line}=  Strip String  ${line}
    \   ${command}=  Split String  ${line}  max_split=3
    \   ${len}=  Get Length  ${command}
    \   Continue For Loop If  ${len} <= 4
    \   Should Not Contain  @{command}[4]  %{TEST_USERNAME}
    \   Should Not Contain  @{command}[4]  %{TEST_PASSWORD}

Docker run immediate exit
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${output}

Docker run verify container start and stop time
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${cmdStart}=  Run  date +%s
    Sleep  3
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name startStop ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${output}
    ${rc}  ${containerStart}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.State.StartedAt}}' startStop | xargs date +%s -d
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${containerStop}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.State.FinishedAt}}' startStop | xargs date +%s -d
    Should Be Equal As Integers  ${rc}  0
    ${startStatus}=  Run Keyword And Return Status  Should Be True  ${cmdStart} <= ${containerStart}
    Run Keyword Unless  ${startStatus}  Fail  container start time before command start
    ${stopStatus}=  Run Keyword And Return Status  Should Be True  ${cmdStart} < ${containerStop}
    Run Keyword Unless  ${stopStatus}  Fail  container stop time before command start
    ${timeDiff}=  Evaluate  ${containerStop}-${cmdStart}
    Should Be True  0 < ${timeDiff} < 60000

Docker run verify name and id are not conflated
    ${rc}  ${container1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${shortID1}=  Get container shortID  ${container1}
    ${rc}  ${container2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --name ${shortID1} ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${container2}  Conflict

Docker run and auto remove
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    ${count}=  Get Length  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --rm ${busybox} date
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    Length Should Be  ${output}  ${count}

Docker run and auto remove with anonymous volumes and named volumes
       ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
       Should Be Equal As Integers  ${rc}  0
       ${output}=  Split To Lines  ${output}
       ${count}=  Get Length  ${output}
       ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
       Set Test Variable  ${namedImageVol}  non-anonymous-image-volume-${suffix}
       Should Be Equal As Integers  ${rc}  0
       Set Test Variable  ${imageVolumeContainer}  I-Have-Two-Anonymous-Volumes-${suffix}
       ${rc}  ${c5}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --rm --name ${imageVolumeContainer} -v ${namedImageVol}:/data/db -v /I/AM/ANONYMOOOOSE mongo bash
       Should Be Equal As Integers  ${rc}  0
       ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
       Should Be Equal As Integers  ${rc}  0
       ${output}=  Split To Lines  ${output}
       Length Should Be  ${output}  ${count}
       ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
       Should Contain  ${output}  ${namedImageVol}


Docker run mysql container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -v vol:/var/lib/mysql -e MYSQL_ROOT_PASSWORD=pw --name test-mysql mysql
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify container is running and remove it  test-mysql

Docker run mariadb container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -e MYSQL_ROOT_PASSWORD=pw --name test-mariadb mariadb
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify container is running and remove it  test-mariadb

Docker run postgres container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name test-postgres postgres
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify container is running and remove it  test-postgres

Docker run --hostname to set hostname and domainname
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --hostname vic.test ${busybox} hostname
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  vic.test
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --hostname vic.test ${busybox} hostname -d
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  test
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --hostname vic.test ${busybox} cat /etc/hosts
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  vic.test

Docker run --rm concurrent
    ${IN_HAAS}=  Run Keyword And Return Status  Should Contain  %{HAAS_URL_ARRAY}  %{TEST_URL}
    Run Keyword Unless  ${IN_HAAS}  Pass Execution  This test is too resource intensive for Nimbus currently

    # Make sure all old containers are cleaned up first, to maximize likelihood of not hitting insufficient resources issue
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs docker %{VCH-PARAMS} rm -f
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${ubuntu}
    Should Be Equal As Integers  ${rc}  0

    ${pids}=  Create List
    :FOR  ${idx}  IN RANGE  0  16
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} run -d --rm --name rm-concurrent-${idx} --cpuset-cpus 1 --memory 1GB ubuntu /bin/sh -c'a\=0; while [ $a -lt 75 ]; do echo "line $a"; a\=expr $a + 1; sleep 2; done;'  shell=True
    \   Append To List  ${pids}  ${pid}

    :FOR  ${pid}  IN  @{pids}
    \   Log To Console  \nWaiting for ${pid}
    \   ${res}=  Wait For Process  ${pid}
    \   Log  ${res.stdout}
    \   Should Be Equal As Integers  ${res.rc}  0

    :FOR  ${idx}  IN RANGE  0  16
    \   Wait Until Keyword Succeeds  10x  3s  Verify container is removed  rm-concurrent-${idx}
