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
Documentation  Test 1-43 - Docker CP Offline
Resource  ../../resources/Util.robot
Suite Setup  Set up test files and install VIC appliance to test server
Suite Teardown  Clean up test files and VIC appliance to test server
Test Timeout  20 minutes

*** Keywords ***
Set up test files and install VIC appliance to test server
    Conditional Install VIC Appliance To Test Server
    Remove All Volumes
    Create File  ${CURDIR}/foo.txt   hello world
    Create File  ${CURDIR}/content   fake file content for testing only
    Create Directory  ${CURDIR}/bar
    Create Directory  ${CURDIR}/mnt
    Create Directory  ${CURDIR}/mnt/vol1
    Create Directory  ${CURDIR}/mnt/vol2
    Create File  ${CURDIR}/mnt/root.txt   rw layer file
    Create File  ${CURDIR}/mnt/vol1/v1.txt   vol1 file
    Create File  ${CURDIR}/mnt/vol2/v2.txt   vol2 file
    ${rc}  ${output}=  Run And Return Rc And Output  dd if=/dev/urandom of=${CURDIR}/largefile.txt count=4096 bs=4096
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create vol1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create vol2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create vol3
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create smallVol --opt Capacity=1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Clean up test files and VIC appliance to test server
    Run Keyword and Continue on Failure  Remove File  ${CURDIR}/foo.txt
    Run Keyword and Continue on Failure  Remove File  ${CURDIR}/content
    Run Keyword and Continue on Failure  Remove File  ${CURDIR}/largefile.txt
    Run Keyword and Continue on Failure  Remove Directory  ${CURDIR}/bar  recursive=True
    Run Keyword and Continue on Failure  Remove Directory  ${CURDIR}/mnt  recursive=True
    Cleanup VIC Appliance On Test Server

*** Test Cases ***
Try To Exploit VCH With Offline Copy of Malicious Tarball
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name exploitme ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  cat ${CURDIR}/../../resources/archive.tar.gz | docker %{VCH-PARAMS} cp - exploitme:/
    Should Not Contain  ${output}  No such file or directory

    Enable VCH SSH

    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -ppassword ssh %{VCH-IP} -lroot -C -oStrictHostKeyChecking=no "ls /tmp | grep pingme"

    Log  ${output}
    Should Not Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  pingme

Copy a file from host to offline container root dir
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i --name offline ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/foo.txt offline:/
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${output}=  Start Container and Exec Command  offline  ls /
    Should Contain  ${output}  foo.txt
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec offline sh -c 'rm /foo.txt'
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Copy a directory from offline container to host cwd
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec offline sh -c 'mkdir testdir && echo "file content" > /testdir/fakefile'
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop offline
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp offline:/testdir ${CURDIR}/
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    OperatingSystem.Directory Should Exist  ${CURDIR}/testdir
    OperatingSystem.File Should Exist  ${CURDIR}/testdir/fakefile
    Remove Directory  ${CURDIR}/testdir  recursive=True

Copy a directory from host to offline container, dst path doesn't exist
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/bar offline:/bar
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${output}=  Start Container and Exec Command  offline  ls /
    Should Contain  ${output}   bar
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop offline
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Copy a non-existent file out of an offline container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp offline:/dne/dne ${CURDIR}
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error

Copy a non-existent directory out of an offline container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp offline:/dne/. ${CURDIR}
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error

Copy a non-existent directory into an offline container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/dne/ offline:/
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  no such file or directory
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f offline
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Copy a large file that exceeds the container volume into an offline container
    ${rc}  ${cid}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i -v smallVol:/small ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${cid}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/largefile.txt ${cid}:/small
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${cid}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Copy a file from host to offline container, dst is a volume
    ${rc}  ${cid}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i -v vol1:/vol1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${cid}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/foo.txt ${cid}:/vol1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${output}=  Start Container and Exec Command  ${cid}  ls /vol1
    Should Contain  ${output}  foo.txt
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${cid}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Copy a file from host to offline container, dst is a nested volume with 2 levels
    ${rc}  ${cid}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i -v vol1:/vol1 -v vol2:/vol1/vol2 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${cid}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/foo.txt ${cid}:/vol1/vol2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${output}=  Start Container and Exec Command  ${cid}  ls /vol1/vol2
    Should Contain  ${output}  foo.txt
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${cid}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Copy a file from host to offline container, dst is a nested volume with 3 levels
    ${rc}  ${cid}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i -v vol1:/vol1 -v vol2:/vol1/vol2 -v vol3:/vol1/vol2/vol3 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${cid}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/foo.txt ${cid}:/vol1/vol2/vol3
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${output}=  Start Container and Exec Command  ${cid}  ls /vol1/vol2/vol3
    Should Contain  ${output}  foo.txt
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${cid}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Concurrent copy: create processes to copy a small file from host to offline container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i --name concurrent -v vol1:/vol1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${pids}=  Create List
    Log To Console  \nIssue 10 docker cp commands for small file
    :FOR  ${idx}  IN RANGE  0  10
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} cp ${CURDIR}/foo.txt concurrent:/foo-${idx}  shell=True
    \   Append To List  ${pids}  ${pid}
    Log To Console  \nWait for them to finish and check their RC
    :FOR  ${pid}  IN  @{pids}
    \   Log To Console  \nWaiting for ${pid}
    \   ${res}=  Wait For Process  ${pid}
    \   Log  ${res.stdout}
    \   Should Be Equal As Integers  ${res.rc}  0
    ${output}=  Start Container and Exec Command  concurrent  ls /
    Log To Console  \nCheck if the copy operations succeeded
    :FOR  ${idx}  IN RANGE  0  10
    \   Should Contain  ${output}  foo-${idx}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop concurrent
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Concurrent copy: repeat copy a large file from host to offline container several times
    ${pids}=  Create List
    Log To Console  \nIssue 10 docker cp commands for large file
    :FOR  ${idx}  IN RANGE  0  10
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} cp ${CURDIR}/largefile.txt concurrent:/vol1/lg-${idx}  shell=True
    \   Append To List  ${pids}  ${pid}
    Log To Console  \nWait for them to finish and check their RC
    :FOR  ${pid}  IN  @{pids}
    \   Log To Console  \nWaiting for ${pid}
    \   ${res}=  Wait For Process  ${pid}
    \   Log  ${res.stdout}
    \   Should Be Equal As Integers  ${res.rc}  0
    ${output}=  Start Container and Exec Command  concurrent  ls /vol1
    Log To Console  \nCheck if the copy operations succeeded
    :FOR  ${idx}  IN RANGE  0  10
    \   Should Contain  ${output}  lg-${idx}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop concurrent
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

# NOTE: this test depends on the prior test passing as it uses the copied files from that test as the source files for this test
Concurrent copy: repeat copy a large file from offline container to host several times
    ${pids}=  Create List
    Log To Console  \nIssue 10 docker cp commands for large file
    :FOR  ${idx}  IN RANGE  0  10
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} cp concurrent:/vol1/lg-${idx} ${CURDIR}  shell=True
    \   Append To List  ${pids}  ${pid}
    Log To Console  \nWait for them to finish and check their RC
    :FOR  ${pid}  IN  @{pids}
    \   Log To Console  \nWaiting for ${pid}
    \   ${res}=  Wait For Process  ${pid}
    \   Log  ${res.stdout}
    \   Should Be Equal As Integers  ${res.rc}  0
    Log To Console  \nCheck if the copy operations succeeded
    :FOR  ${idx}  IN RANGE  0  10
    \   OperatingSystem.File Should Exist  ${CURDIR}/lg-${idx}
    \   Remove File  ${CURDIR}/lg-${idx}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f concurrent
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Sub volumes: copy from host to offline container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -i -v vol1:/mnt/vol1 -v vol2:/mnt/vol2 --name subVol ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/mnt subVol:/
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${output}=  Start Container and Exec Command  subVol  find /mnt
    Should Contain  ${output}  /mnt/root.txt
    Should Contain  ${output}  /mnt/vol1/v1.txt
    Should Contain  ${output}  /mnt/vol2/v2.txt
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop subVol
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Sub volumes: copy from offline container to host
    Operating System.List Directory  ${CURDIR}/
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp subVol:/mnt ${CURDIR}/result
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    # Needed to help diagnose failures
    ${rc}  ${output}=  Run And Return Rc And Output  find ${CURDIR}/result -ls
    Log  ${output}
    Remove Directory  ${CURDIR}/result  recursive=True
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp subVol:/mnt ${CURDIR}/result
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    OperatingSystem.Directory Should Exist  ${CURDIR}/result/vol1
    OperatingSystem.Directory Should Exist  ${CURDIR}/result/vol2
    OperatingSystem.File Should Exist  ${CURDIR}/result/root.txt
    OperatingSystem.File Should Exist  ${CURDIR}/result/vol1/v1.txt
    OperatingSystem.File Should Exist  ${CURDIR}/result/vol2/v2.txt
    Remove Directory  ${CURDIR}/result  recursive=True
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f subVol
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
