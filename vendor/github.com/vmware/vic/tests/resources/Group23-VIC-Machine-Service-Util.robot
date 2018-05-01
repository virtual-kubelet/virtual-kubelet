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


*** Variables ***
${HTTP_PORT}     1337
${HTTPS_PORT}    31337

${RC}            The return code of the last curl invocation
${OUTPUT}        The output of the last curl invocation
${STATUS}        The HTTP status of the last curl invocation


*** Keywords ***
Start VIC Machine Server
    Start Process    ./bin/vic-machine-server --port ${HTTP_PORT} --scheme http    shell=True    cwd=/go/src/github.com/vmware/vic


Get Path
    [Arguments]    ${path}
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "http://127.0.0.1:${HTTP_PORT}/container/${PATH}"
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Get Path Under Target
    [Arguments]    ${path}    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${auth}=    Evaluate    base64.b64encode("%{TEST_USERNAME}:%{TEST_PASSWORD}")    modules=base64
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "http://127.0.0.1:${HTTP_PORT}/container/target/%{TEST_URL}/${PATH}?${fullQuery}" -H "Accept: application/json" -H "Authorization: Basic ${auth}"
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Get Path Under Target Using Session
    [Arguments]    ${path}    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${ticket}=    Run    govc vm.console %{VCH-NAME} | awk -F'[:@]' '{print $3}'
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X GET "http://127.0.0.1:${HTTP_PORT}/container/target/%{TEST_URL}/${path}?${fullQuery}" -H "Accept: application/json" -H "X-VMWARE-TICKET: ${ticket}"
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Post Path Under Target
    [Arguments]    ${path}    ${data}    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${auth}=    Evaluate    base64.b64encode("%{TEST_USERNAME}:%{TEST_PASSWORD}")    modules=base64
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X POST "http://127.0.0.1:${HTTP_PORT}/container/target/%{TEST_URL}/${PATH}?${fullQuery}" -H "Accept: application/json" -H "Authorization: Basic ${auth}" -H "Content-Type: application/json" --data ${data}
    ${OUTPUT}    ${STATUS}=    Split String From Right    ${OUTPUT}    \n    1
    Set Test Variable    ${RC}
    Set Test Variable    ${OUTPUT}
    Set Test Variable    ${STATUS}

Delete Path Under Target
    [Arguments]    ${path}    ${data}=''    @{query}
    ${fullQuery}=    Catenate    SEPARATOR=&    thumbprint=%{TEST_THUMBPRINT}    @{query}
    ${auth}=    Evaluate    base64.b64encode("%{TEST_USERNAME}:%{TEST_PASSWORD}")    modules=base64
    ${RC}  ${OUTPUT}=    Run And Return Rc And Output    curl -s -w "\n\%{http_code}\n" -X DELETE "http://127.0.0.1:${HTTP_PORT}/container/target/%{TEST_URL}/${PATH}?${fullQuery}" -H "Accept: application/json" -H "Authorization: Basic ${auth}" -H "Content-Type: application/json" --data ${data}
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
