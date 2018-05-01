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
Documentation  This resource provides keywords to interact with Github

*** Keywords ***
Get State Of Github Issue
    [Arguments]  ${num}
    [Tags]  secret
    :FOR  ${idx}  IN RANGE  0  5
    \   ${status}  ${result}=  Run Keyword And Ignore Error  Get  https://api.github.com/repos/vmware/vic/issues/${num}?access_token\=%{GITHUB_AUTOMATION_API_KEY}
    \   Exit For Loop If  '${status}'
    \   Sleep  1
    Should Be Equal  ${result.status_code}  ${200}
    ${status}=  Get From Dictionary  ${result.json()}  state
    [Return]  ${status}

Post Comment To Github Issue
    [Arguments]  ${num}  ${comment}
    [Tags]  secret
    :FOR  ${idx}  IN RANGE  0  5
    \   ${status}  ${result}=  Run Keyword And Ignore Error  Post  https://api.github.com/repos/vmware/vic/issues/${num}/comments?access_token\=%{GITHUB_AUTOMATION_API_KEY}  data={"body": "${comment}"}
    \   Exit For Loop If  '${status}'
    \   Sleep  1
    Should Be Equal  ${result.status_code}  ${201}

Check VMware Organization Membership
    [Arguments]  ${username}
    [Tags]  secret
    :FOR  ${idx}  IN RANGE  0  5
    \   ${status}  ${result}=  Run Keyword And Ignore Error  Get  https://api.github.com/orgs/vmware/members/${username}?access_token\=%{GITHUB_AUTOMATION_API_KEY}
    \   Exit For Loop If  '${status}'
    \   Sleep  1
    ${isMember}=  Run Keyword And Return Status  Should Be Equal  ${result.status_code}  ${204}
    [Return]  ${isMember}
