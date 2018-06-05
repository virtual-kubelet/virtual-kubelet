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
Documentation  Test 6-02 - Verify default parameters
Resource  ../../resources/Util.robot
Suite Teardown  Run Keyword And Ignore Error  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Should Not Have VMOMI Session
    [Arguments]  ${thumbprint}
    ${output}=  Run  govc session.ls | grep vic-machine | grep ${thumbprint} | wc -l
    Should Be Equal As Integers  ${output}  0

Get Thumbprint From Log
    [Arguments]  ${output}
    ${logline}=  Get Lines Containing String  ${output}  Creating VMOMI session with thumbprint
    Should Not Be Equal As Strings  ${logline}  ${EMPTY}
    ${match}  ${msg}=  Should Match Regexp  ${logline}  msg\="([^"]*)"
    ${rest}  ${thumbprint}=  Split String From Right  ${msg}  ${SPACE}  1
    [Return]  ${thumbprint}

*** Test Cases ***
Delete with defaults
    Set Test Environment Variables

    ${ret}=  Run  bin/vic-machine-linux delete --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD}
    Should Contain  ${ret}  vic-machine-linux delete failed:  resource pool
    Should Contain  ${ret}  /Resources/virtual-container-host' not found

Wrong Password No Panic
    Set Test Environment Variables
    ${ret}=  Run  bin/vic-machine-linux create --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=INCORRECT
    Should Contain  ${ret}  vic-machine-linux create failed
    Should Not Contain  ${ret}  panic:

    ${ret}=  Run  bin/vic-machine-linux delete --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=INCORRECT
    Should Contain  ${ret}  vic-machine-linux delete failed
    Should Not Contain  ${ret}  panic:

    ${ret}=  Run  bin/vic-machine-linux inspect --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=INCORRECT
    Should Contain  ${ret}  vic-machine-linux inspect  failed
    Should Not Contain  ${ret}  panic:

    ${ret}=  Run  bin/vic-machine-linux ls --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=INCORRECT
    Should Contain  ${ret}  vic-machine-linux ls failed
    Should Not Contain  ${ret}  panic:

    ${ret}=  Run  bin/vic-machine-linux upgrade --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=INCORRECT
    Should Contain  ${ret}  vic-machine-linux upgrade failed
    Should Not Contain  ${ret}  panic:

    ${ret}=  Run  bin/vic-machine-linux configure --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=INCORRECT
    Should Contain  ${ret}  vic-machine-linux configure failed
    Should Not Contain  ${ret}  panic:

Check That VMOMI Sessions Don't Leak From VIC Machine
    Set Test Environment Variables
    ${output}=  Run  bin/vic-machine-linux ls --target %{TEST_URL} --debug=1 --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD}
    Log  ${output}
    ${thumbprint}=  Get Thumbprint From Log  ${output}
    Should Not Have VMOMI Session  ${thumbprint}

    ${output}=  Install VIC Appliance To Test Server
    Log  ${output}
    ${thumbprint}=  Get Thumbprint From Log  ${output}
    Should Not Have VMOMI Session  ${thumbprint}

    ${output}=  Run  bin/vic-machine-linux inspect --target %{TEST_URL} --debug=1 --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME}
    Log  ${output}
    ${thumbprint}=  Get Thumbprint From Log  ${output}
    Should Not Have VMOMI Session  ${thumbprint}

    ${output}=  Run  bin/vic-machine-linux upgrade --target %{TEST_URL} --debug=1 --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD}
    # upgrade fails since but that's ok for this test -- it still creates a session before failing
    Log  ${output}
    ${thumbprint}=  Get Thumbprint From Log  ${output}
    Should Not Have VMOMI Session  ${thumbprint}

    ${output}=  Run  bin/vic-machine-linux configure --target %{TEST_URL} --debug=1 --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME}
    Log  ${output}
    ${thumbprint}=  Get Thumbprint From Log  ${output}
    Should Not Have VMOMI Session  ${thumbprint}

    ${output}=  Run  bin/vic-machine-linux delete --target %{TEST_URL} --debug=1 --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME}
    Log  ${output}
    ${thumbprint}=  Get Thumbprint From Log  ${output}
    Should Not Have VMOMI Session  ${thumbprint}

    Run Keyword And Ignore Error  Cleanup VCH Bridge Network  %{VCH-NAME}
