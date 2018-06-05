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
Documentation  Test 1-36 - Docker Rename
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Rename a non-existent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename foo bar
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  No such container: foo

Rename a created container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${contID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name cont1-name1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${contID}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont1-name1 cont1-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify Container Rename  cont1-name1  cont1-name2  ${contID}

Rename a running container
    ${rc}  ${contID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --name cont2-name1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${contID}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont2-name1 cont2-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify Container Rename  cont2-name1  cont2-name2  ${contID}

Rename a stopped container
    ${rc}  ${contID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --name cont3-name1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${contID}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop cont3-name1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont3-name1 cont3-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start cont3-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify Container Rename  cont3-name1  cont3-name2  ${contID}

Rename a container with an empty name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name cont4 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont4 ""
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Neither old nor new names may be empty

Rename a container with a claimed name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name cont5 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name cont6 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont5 cont5
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont5 cont6
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error

Name resolution for a created container after renaming+starting it
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name cont7-name1 ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont7-name1 cont7-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start cont7-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --link cont7-name2:cont7alias ${busybox} ping -c2 cont7alias
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} ping -c2 cont7-name2
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

Name resolution for a running container after renaming+restarting it
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --name cont8-name1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont8-name1 cont8-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop cont8-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start cont8-name2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --link cont8-name2:cont8alias ${busybox} ping -c2 cont8alias
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} ping -c2 cont8-name2
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

Name resolution for a running container after renaming it
    ${status}=  Get State Of Github Issue  4375
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-35-Docker-Rename needs to be updated now that #4375 is closed

    # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --name cont9-name1 busybox
    # Should Be Equal As Integers  ${rc}  0
    # Should Not Contain  ${output}  Error
    # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename cont9-name1 cont9-name2
    # Should Be Equal As Integers  ${rc}  0
    # Should Not Contain  ${output}  Error
    # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --link cont9-name2:cont9alias busybox ping -c2 cont9alias
    # Should Be Equal As Integers  ${rc}  0
    # Should Contain  ${output}  2 packets transmitted, 2 packets received
    # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run busybox ping -c2 cont9-name2
    # Should Be Equal As Integers  ${rc}  0
    # Should Contain  ${output}  2 packets transmitted, 2 packets received
