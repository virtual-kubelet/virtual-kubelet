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
Documentation  Test 6-15 - Verify remote syslog
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  additional-args=--syslog-address tcp://%{SYSLOG_SERVER}:514 --debug 1
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Variables ***
${SYSLOG_FILE}  /var/log/syslog

*** Keywords ***
Get Remote PID
    [Arguments]  ${proc}
    ${pid}=  Execute Command  ps -C ${proc} -o pid=
    ${pid}=  Strip String  ${pid}
    [Return]  ${pid}

*** Test Cases ***
Verify VCH remote syslog
    # enable ssh
    ${output}=  Run  bin/vic-machine-linux debug --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD}
    Should Contain  ${output}  Completed successfully

    # make sure we use ip address, and not fqdn
    ${ip}=  Run  dig +short %{VCH-IP}
    ${vch-ip}=  Set Variable If  '${ip}' == ''  %{VCH-IP}  ${ip}

    @{procs}=  Create List  port-layer-server  docker-engine-server  vic-init  vicadmin
    &{proc-pids}=  Create Dictionary
    &{proc-hosts}=  Create Dictionary

    ${vch-conn}=  Open Connection  ${vch-ip}
    Login  root  password
    :FOR  ${proc}  IN  @{procs}
    \     ${pid}=  Get Remote PID  ${proc}
    \     Set To Dictionary  ${proc-pids}  ${proc}  ${pid}
    \     Set To Dictionary  ${proc-hosts}  ${proc}  ${vch-ip}
    Close Connection
    Set To Dictionary  ${proc-hosts}  vic-init  Photon

    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0

    Run Regression Tests

    ${pull}=  Run  docker %{VCH-PARAMS} pull ${busybox}
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d ${busybox} ls /
    Should Be Equal As Integers  ${rc}  0
    ${shortID}=  Get container shortID  ${id}

    Wait Until Container Stops  ${id}  5

    ${syslog-conn}=  Open Connection  %{SYSLOG_SERVER}  encoding=unicode_escape
    Login  %{SYSLOG_USER}  %{SYSLOG_PASSWD}
    ${out}=  Wait Until Keyword Succeeds  10x  3s  Execute Command  cat ${SYSLOG_FILE}
    Close Connection
    Log  ${out}

    ${keys}=  Get Dictionary Keys  ${proc-pids}
    :FOR  ${proc}  IN  @{keys}
    \     ${pid}=  Get From Dictionary  ${proc-pids}  ${proc}
    \     ${host}=  Get From Dictionary  ${proc-hosts}  ${proc}
    \     Should Contain  ${out}  ${host} ${proc}[${pid}]:

    ${pid}=  Get From Dictionary  ${proc-pids}  docker-engine-server
    ${port-layer-pid}=  Get From Dictionary  ${proc-pids}  port-layer-server
    ${vic-admin-pid}=  Get From Dictionary  ${proc-pids}  vicadmin
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling GET /v\\d.\\d{2}/containers/json\\?all\\=1

    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling POST /v\\d.\\d{2}/containers/create
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling POST /v\\d.\\d{2}/images/create\\?fromImage\\=(\\S+)*busybox\\&tag\\=latest
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling POST /v\\d.\\d{2}/containers/\\w{64}/start
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling POST /v\\d.\\d{2}/containers/\\w{64}/stop

    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling GET /v\\d.\\d{2}/images/json
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling GET /v\\d.\\d{2}/containers/json
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling GET /info

    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling DELETE /v\\d.\\d{2}/containers/\\w{64}
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: Calling DELETE /v\\d.\\d{2}/images/(\\S+)*busybox

    # Check trace logger for docker-engine and port-layer
    Should Match Regexp  ${out}  ${vch-ip} docker-engine-server\\[${pid}\\]: op=${pid}.\\d+: Commit container \\w{64}
    Should Match Regexp  ${out}  ${vch-ip} port-layer-server\\[${port-layer-pid}\\]: op=${port-layer-pid}.\\d+: Creating base file structure on disk
    Should Match Regexp  ${out}  ${vch-ip} vicadmin\\[${vic-admin-pid}\\]: op=${vic-admin-pid}.\\d+: vSphere resource cache populating...

    Should Match Regexp  ${out}  ${shortID} ${shortID}\\[1\\]: bin
    Should Match Regexp  ${out}  ${shortID} ${shortID}\\[1\\]: home
    Should Match Regexp  ${out}  ${shortID} ${shortID}\\[1\\]: var
