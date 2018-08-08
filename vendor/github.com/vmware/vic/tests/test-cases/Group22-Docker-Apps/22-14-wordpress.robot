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
Documentation  Test 22-14 - wordpress
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Check wordpress container
    [Arguments]  ${url}
    Remove File  index.html*
    ${output}=  Run  wget ${url} && cat index.html
    Should Contain  ${output}  <title>WordPress &rsaquo; Setup Configuration File</title>

*** Test Cases ***
Simple wordpress container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name mysql1 -e MYSQL_ROOT_PASSWORD=password1 -d mysql
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name wordpress1 --link mysql1:mysql -d wordpress
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${ip}=  Get IP Address of Container  wordpress1
    Remove File  index.html*
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} sh -c "wget ${ip} && cat index.html"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  <title>WordPress &rsaquo; Setup Configuration File</title>

Wordpress container with published ports
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name wordpress2 --link mysql1:mysql -p 8080:80 -d wordpress
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    Wait Until Keyword Succeeds  10x  6s  Check wordpress container  %{VCH-IP}:8080
