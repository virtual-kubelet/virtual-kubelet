# Copyright 2018 VMware, Inc. All Rights Reserved.
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
Documentation    This resource contains keywords which are helpful for testing DRS VM Groups.


*** Keywords ***
Cleanup
    Run Keyword And Continue On Failure    Remove Group     %{VCH-NAME}

    Cleanup VIC Appliance On Test Server


Create Group
    [Arguments]    ${name}

    ${rc}  ${out}=    Run And Return Rc And Output     govc cluster.group.create -name "${name}" -vm --json 2>&1
    Should Be Equal As Integers    ${rc}    0

Remove Group
    [Arguments]    ${name}

    ${rc}  ${out}=    Run And Return Rc And Output     govc cluster.group.remove -name "${name}" --json 2>&1
    Should Be Equal As Integers    ${rc}    0


Verify Group Not Found
    [Arguments]    ${name}

    ${out}=    Run     govc cluster.group.ls -name "${name}" --json 2>&1
    Should Be Equal As Strings    ${out}    govc: group "${name}" not found

Verify Group Empty
    [Arguments]    ${name}

    ${out}=    Run     govc cluster.group.ls -name "${name}" --json 2>&1
    Should Be Equal As Strings    ${out}    null

Verify Group Contains VMs
    [Arguments]    ${name}    ${count}

    ${out}=    Run    govc cluster.group.ls -name "${name}" --json | jq 'length'
    Should Be Equal As Integers    ${out}    ${count}


Create Three Containers
    ${POWERED_OFF_CONTAINER_NAME}=    Generate Random String  15
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} create --name ${POWERED_OFF_CONTAINER_NAME} ${busybox} /bin/top

    Set Test Variable    ${POWERED_OFF_CONTAINER_NAME}

    ${POWERED_ON_CONTAINER_NAME}=    Generate Random String  15
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} create --name ${POWERED_ON_CONTAINER_NAME} ${busybox} /bin/top
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} start ${out}

    Set Test Variable    ${POWERED_ON_CONTAINER_NAME}

    ${RUN_CONTAINER_NAME}=    Generate Random String  15
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} run -d --name ${RUN_CONTAINER_NAME} ${busybox} /bin/top

    Set Test Variable    ${RUN_CONTAINER_NAME}

Delete Containers
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} rm ${POWERED_OFF_CONTAINER_NAME}
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} rm -f ${POWERED_ON_CONTAINER_NAME}
    ${rc}  ${out}=    Run And Return Rc And Output    docker %{VCH-PARAMS} rm -f ${RUN_CONTAINER_NAME}


Configure VCH without modifying affinity
    ${rc}  ${out}=    Secret configure VCH for affinity    --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-conf:configure
    Log    ${out}
    Should Be Equal As Integers    ${RC}    0

Configure VCH to enable affinity
    ${rc}  ${out}=    Secret configure VCH for affinity    --affinity-vm-group=true
    Log    ${out}
    Should Be Equal As Integers    ${RC}    0

Configure VCH to disable affinity
    ${rc}  ${out}=    Secret configure VCH for affinity    --affinity-vm-group=false
    Log    ${out}
    Should Be Equal As Integers    ${RC}    0

Secret configure VCH for affinity
    [Tags]    secret
    [Arguments]    ${additional-args}=${EMPTY}
    ${rc}  ${out}=    Run And Return Rc And Output    bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} ${additional-args}
    [Return]    ${rc}  ${out}


