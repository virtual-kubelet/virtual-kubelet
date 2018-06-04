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
Documentation  Test 6515
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server


*** Test Cases ***
Check hosts contains base entries
    # basic confirmation of function
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${name}=  Generate Random String  15
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} ${busybox} sleep 600
    Should Be Equal As Integers  ${rc}  0
    ${shortid}=  Get container shortID  ${id}

    # update the base hosts file with an alias
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp ${CURDIR}/6515.resource ${id}:/etc/hosts
    Should Be Equal As Integers  ${rc}  0

    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} start ${name}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${hosts}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${id} cat /etc/hosts
    Should Be Equal As Integers  ${rc}  0
    Log  ${hosts}
    Should Contain  ${hosts}  robot-test-alias

    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} restart ${id}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${hosts}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${id} cat /etc/hosts
    Should Be Equal As Integers  ${rc}  0
    Log  ${hosts}
    Should Contain  ${hosts}  robot-test-alias
