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
Documentation  Test 22-09 - httpd
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Keywords ***
Curl httpd endpoint
    [Arguments]  ${endpoint}
    ${rc}  ${output}=  Run And Return Rc And Output  curl ${endpoint}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${output}

*** Test Cases ***
Httpd with a mapped volume folder
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=vol1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -v vol1:/mydata ${busybox} sh -c "echo '<p>HelloWorld</p>' > /mydata/test.html"
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --name httpd1 -v vol1:/usr/local/apache2/htdocs/ -p 8080:80 httpd:2.4
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${ip}=  Get IP Address of Container  httpd1

    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl httpd endpoint  %{VCH-IP}:8080/test.html
    Should Contain  ${output}  HelloWorld
