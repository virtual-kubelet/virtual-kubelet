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
Documentation  Test 22-04 - mysql
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Check mysql container
    [Arguments]  ${ip}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --rm mysql sh -c 'mysql -h${ip} -P3306 -uroot -ppassword1 -e "show databases;"'
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  information_schema
    Should Contain  ${output}  performance_schema

*** Test Cases ***
Simple background mysql
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name mysql1 -e MYSQL_ROOT_PASSWORD=password1 -d mysql
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get IP Address of Container  mysql1
    Wait Until Keyword Succeeds  5x  6s  Check mysql container  ${ip}
