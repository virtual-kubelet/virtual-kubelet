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
Documentation  Test 1-41 - Docker Commit
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Commit nano to image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name commit1 ${debian} tail -f /dev/null
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec commit1 apt-get update
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec commit1 apt-get install nano
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop -t1 commit1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} commit commit1 debian-nano
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run debian-nano whereis nano
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  /bin/nano

Commit env variable to image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name commit2 ${debian} tail -f /dev/null
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop -t1 commit2
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} commit -c "ENV TEST commitEnvTest" commit2 debian-env
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run debian-env env
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  TEST\=commitEnvTest

Unsupported commit command
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name commit3 ${debian} tail -f /dev/null
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop -t1 commit3
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} commit -c "RUN apt-get install nginx" commit3 debian-env
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error response from daemon: run is not a valid change command

Commit with author and message
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d --name commit4 ${debian} tail -f /dev/null
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop -t1 commit4
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} commit -a Robot -m "Robot made a commit" -c "ENV TEST commitEnvTest" commit4 debian-auth
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect debian-auth
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  "Author": "Robot"
    Should Contain  ${output}  "Comment": "Robot made a commit"

Commit to nonexistent container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} commit -c "ENV TEST commitEnvTest" fakeContainer image-fake
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Error response from daemon: No such container: fakeContainer
