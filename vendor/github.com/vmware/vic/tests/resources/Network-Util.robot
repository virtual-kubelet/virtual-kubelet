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
Documentation  This resource contains any keywords related to networking

*** Keywords ***
Ping Host Successfully
    [Arguments]  ${host}
    ${rc}  ${output}=  Run And Return Rc And Output  ping -c 1 ${host}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  1 packets transmitted, 1 packets received, 0% packet loss
    [Return]  ${output}
