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
Documentation  Test 1-08 - Docker Logs
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Grep Logs And Count Lines
    [Arguments]  ${id}  ${match}  ${total}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${id}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${match}
    ${linecount}=  Get Line Count  ${output}
    Should Be Equal As Integers  ${linecount}  ${total}

Upgrade
  ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
  Should Contain  ${output}  Completed successfully
  Should Not Contain  ${output}  Rolling back upgrade
  Should Be Equal As Integers  ${rc}  0

Check Upgraded Version
  ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux version
  @{vers}=  Split String  ${output}
  ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE}
  Should Contain  ${output}  Completed successfully
  Should Contain  ${output}  @{vers}[2]
  Should Not Contain  ${output}  %{INITIAL-VERSION}
  Should Be Equal As Integers  ${rc}  0
  Log  ${output}
  Get Docker Params  ${output}  ${true}

*** Test Cases ***
Docker logs with tail
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} sh -c 'seq 1 5000'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Grep Logs And Count Lines  ${id}  2500  5000
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --tail=all ${id}
    ${linecount}=  Get Line Count  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Integers  ${linecount}  5000
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --tail=200 ${id}
    ${linecount}=  Get Line Count  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Integers  ${linecount}  200
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --tail=0 ${id}
    Should Be Equal As Integers  ${rc}  0
    ${linecount}=  Get Line Count  ${output}
    Should Be Equal As Integers  ${linecount}  0

Docker logs with follow
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} sh -c 'for i in $(seq 1 5) ; do sleep 1 && echo line $i; done'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${id}
    Should Be Equal As Integers  ${rc}  0
    ${linecount}=  Get Line Count  ${output}
    Should Be Equal As Integers  ${linecount}  5
    ${lastline}=  Get Line  ${output}  4
    Should Contain  ${lastline}  line 5
    # Container is stopped at this point, verify that --follow does not block.
    ${rc}  ${output2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${id}
    Should Be Equal  ${output}  ${output2}

Docker logs with follow and tail
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} sh -c 'trap "seq 11 20; exit" HUP; seq 1 10; while true; do sleep 1; done'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    # Wait for the first 10 lines to be logged
    Wait Until Keyword Succeeds  20x  200 milliseconds  Grep Logs And Count Lines  ${id}  5  10
    # kill -HUP will create another 5 lines of log output
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} kill -s HUP ${id}
    Should Be Equal As Integers  ${rc}  0
    # --tail=5 to skip the first 5 lines and --follow to wait for the rest
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --tail 5 --follow ${id}
    Should Be Equal As Integers  ${rc}  0
    ${linecount}=  Get Line Count  ${output}
    Should Be True  ${linecount} >= 5

Docker logs follow shutdown
    # Test that logs --follow reads all data following a close (shutdown) event.
    # Keys to this test:
    # - The container VM shutdown event happens while the (HTTP) log follow poller is asleep.
    # - The container VM log accumulates > interaction layer buffer size data while log follow poller was alseep.
    # Note that the interaction layer currently uses an extra super tiny buffer size of 64 bytes.
    ${rc}  ${buffer}=  Run And Return Rc And Output  bash -c "printf '=%.0s' {1..65}"
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} sh -c 'echo ${buffer}; sleep .5; echo ${buffer}'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${id}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${id}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  ${buffer}\n${buffer}

Docker binary logs
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${ubuntu}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${ubuntu} /bin/cat /bin/hostname >/tmp/hostname
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a |grep ${ubuntu} |awk '{print $1}'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${id} >/tmp/hostname-log
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${h1}=  Run And Return Rc And Output  sha256sum /tmp/hostname |awk '{print $1}'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${h2}=  Run And Return Rc And Output  sha256sum /tmp/hostname-log |awk '{print $1}'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${h1}  ${h2}
    ${rc}  ${output}=  Run And Return Rc And Output  rm /tmp/hostname*
    Should Be Equal As Integers  ${rc}  0

Docker text logs
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${ubuntu} /bin/ls >/tmp/ls
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a |grep /bin/ls |awk '{print $1}'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${id} >/tmp/ls-log
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${h1}=  Run And Return Rc And Output  sha256sum /tmp/ls |awk '{print $1}'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${h2}=  Run And Return Rc And Output  sha256sum /tmp/ls-log |awk '{print $1}'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${h1}  ${h2}
    ${rc}  ${output}=  Run And Return Rc And Output  rm /tmp/ls*
    Should Be Equal As Integers  ${rc}  0

Docker logs with timestamps and since certain time
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} run ${busybox} sh -c 'for i in $(seq 0 9) ; do sleep 1 && echo line $i; done'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a -q |head --lines=1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${lines}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --since=5s ${containerID} |wc -l
    Should Be Equal As Integers  ${rc}  0
    Should Not Be Equal As Integers  ${lines}  10
    ${rc}  ${lines}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --since=1h ${containerID} |wc -l
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Integers  ${lines}  10
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${containerID} |wc -l
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Integers  ${output}  10
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --timestamps ${containerID}
    Should Be Equal As Integers  ${rc}  0
    ${date}=  Fetch From Left  ${output}  ${SPACE}
    Should Match Regexp  ${date}  \\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d{9}Z

Docker logs with no flags
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d ${busybox} sh -c "seq 1 128 | xargs -n1 echo"
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  20x  200 milliseconds  Grep Logs And Count Lines  ${id}  42  128

Docker logs non-existent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs fakeContainer
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error: No such container: fakeContainer
