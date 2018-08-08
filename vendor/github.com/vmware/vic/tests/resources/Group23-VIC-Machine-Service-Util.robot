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
Documentation    This resource contains keywords which are helpful for using curl to test the vic-machine API.
Library  Process

*** Variables ***
${RC}            The return code of the last curl invocation
${OUTPUT}        The output of the last curl invocation
${STATUS}        The HTTP status of the last curl invocation

${VIC_MACHINE_SERVER_LOG}    vic-machine-server.log
${SERVING_AT_TEXT}           Serving vic machine at


*** Keywords ***
Start VIC Machine Server
    ${dir_name}=  Evaluate  'group23_log_dir' + str(random.randint(1000,9999))  modules=random
    Set Suite Variable    ${SERVER_LOG_FILE}    ${dir_name}/${VIC_MACHINE_SERVER_LOG}

    ${handle}=    Start Process    ./bin/vic-machine-server --scheme http --log-directory ${dir_name}/    shell=True    cwd=/go/src/github.com/vmware/vic
    Set Suite Variable    ${SERVER_HANDLE}    ${handle}
    Process Should Be Running    ${handle}
    Sleep  5sec

    ${output}=    Run    cat ${SERVER_LOG_FILE}
    @{output}=    Split To Lines    ${output}
    :FOR    ${line}    IN    @{output}
    \   ${status}=    Run Keyword And Return Status    Should Contain    ${line}    ${SERVING_AT_TEXT}
    \   ${server_url}=    Run Keyword If      ${status}    Fetch From Right    ${line}    ${SPACE}
    \   Run Keyword If    ${status}    Set Suite Variable    ${VIC_MACHINE_SERVER_URL}    ${server_url}

Stop VIC Machine Server
    Run Keyword And Ignore Error    Copy File    ${SERVER_LOG_FILE}    ${SUITE NAME}-${VIC_MACHINE_SERVER_LOG}

    Terminate Process    ${SERVER_HANDLE}    kill=true
    Process Should Be Stopped    ${SERVER_HANDLE}

Get Path
    [Arguments]    ${path}
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "${VIC_MACHINE_SERVER_URL}/container/${PATH}"
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Get Path Under Target
    [Arguments]    ${path}    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${auth}=    Evaluate    base64.b64encode("%{TEST_USERNAME}:%{TEST_PASSWORD}")    modules=base64
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "${VIC_MACHINE_SERVER_URL}/container/target/%{TEST_URL}/${PATH}?${fullQuery}" -H "Accept: application/json" -H "Authorization: Basic ${auth}"
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Get Path Under Target Using Session
    [Arguments]    ${path}    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${ticket}=    Run    govc vm.console %{VCH-NAME} | awk -F'[:@]' '{print $3}'
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "${VIC_MACHINE_SERVER_URL}/container/target/%{TEST_URL}/${path}?${fullQuery}" -H "Accept: application/json" -H "X-VMWARE-TICKET: ${ticket}"
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Post Path Under Target
    [Arguments]    ${path}    ${data}    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${auth}=    Evaluate    base64.b64encode("%{TEST_USERNAME}:%{TEST_PASSWORD}")    modules=base64
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X POST "${VIC_MACHINE_SERVER_URL}/container/target/%{TEST_URL}/${PATH}?${fullQuery}" -H "Accept: application/json" -H "Authorization: Basic ${auth}" -H "Content-Type: application/json" --data ${data}
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Delete Path Under Target
    [Arguments]    ${path}    ${data}=''    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${auth}=    Evaluate    base64.b64encode("%{TEST_USERNAME}:%{TEST_PASSWORD}")    modules=base64
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X DELETE "${VIC_MACHINE_SERVER_URL}/container/target/%{TEST_URL}/${PATH}?${fullQuery}" -H "Accept: application/json" -H "Authorization: Basic ${auth}" -H "Content-Type: application/json" --data ${data}
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Verify Return Code
    Should Be Equal As Integers    ${RC}    0

Verify Status
    [Arguments]    ${expected}
    Should Be Equal As Integers    ${expected}    ${STATUS}

Verify Status Ok
    Verify Status    200

Verify Status Created
    Verify Status    201

Verify Status Accepted
    Verify Status    202

Verify Status Bad Request
    Verify Status    400

Verify Status Not Found
    Verify Status    404

Verify Status Unprocessable Entity
    Verify Status    422

Verify Status Internal Server Error
    Verify Status    500

Output Should Contain
    [Arguments]    ${expected}
    Should Contain    ${OUTPUT}    ${expected}

Output Should Not Contain
    [Arguments]    ${expected}
    Should Not Contain    ${OUTPUT}    ${expected}

Output Should Match Regexp
    [Arguments]    ${expected}
    Should Match Regexp    ${OUTPUT}    ${expected}

Property Should Be Equal
    [Arguments]    ${jq}    ${expected}

    ${actual}=  Run    echo '${OUTPUT}' | jq -r '${jq}'
    Should Be Equal    ${actual}    ${expected}

Property Should Not Be Equal
    [Arguments]    ${jq}    ${expected}

    ${actual}=  Run    echo '${OUTPUT}' | jq -r '${jq}'
    Should Not Be Equal    ${actual}    ${expected}

Property Should Contain
    [Arguments]    ${jq}    ${expected}

    ${actual}=  Run    echo '${OUTPUT}' | jq -r '${jq}'
    Should Contain    ${actual}    ${expected}

Property Should Not Be Empty
    [Arguments]    ${jq}

    ${actual}=  Run    echo '${OUTPUT}' | jq -r '${jq}'
    Should Not Be Empty    ${actual}

Property Length Should Be
    [Arguments]    ${jq}    ${expected}

    ${actual}=  Run    echo '${OUTPUT}' | jq -r '${jq} | length'
    Should Be Equal    ${actual}    ${expected}


Get Service Version String
    ${rc}  ${output}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "${VIC_MACHINE_SERVER_URL}/container/version"
    ${output}    ${status}=    Split String From Right    ${output}    \n    1
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Integers    ${status}    200
    [Return]  ${output}

Verify VCH List Empty
    ${vchs}=  Run  echo '${OUTPUT}' | jq -r '.vchs[]'
    Log  ${vchs}
    Length Should Be  ${vchs}  0

Get Docker Params API
    [Arguments]    ${vch_name}
    Get Path Under Target    vch
    ${docker_host}=    Run    echo '${OUTPUT}' | jq -r '.vchs[] | select(.name=="${vch_name}").docker_host'
    Set Test Variable    ${docker_host}
    Should Not Be Empty    ${docker_host}
    Should Not Be Equal    ${docker_host}    null
