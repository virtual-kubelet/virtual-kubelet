# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 1-12 - Docker RMI
Resource  ../../resources/Util.robot
Suite Setup  Run Keywords  Conditional Install VIC Appliance To Test Server  Remove All Containers
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Basic docker pull, restart, and remove image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${alpine}
    Should Be Equal As Integers  ${rc}  0

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -before-reboot-1
    Reboot VM  %{VCH-NAME}
    Wait For VCH Initialization  30x  10 seconds

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images ${busybox}:latest
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  busybox
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images ${alpine}:latest
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  alpine

Remove image with a removed container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images ${busybox}:latest
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  busybox

Remove image with a container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${busybox}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Failed to remove image "${busybox}"
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${busybox}

    # Cleanup container for future test-cases that use ${busybox}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${container}
    Should Be Equal As Integers  ${rc}  0

Remove a fake image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi fakeImage
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: Error parsing reference: "fakeImage" is not a valid repository/tag

Remove an image pulled by digest
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2

Remove images by short and long ID after VCH restart
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${alpine}
    Should Be Equal As Integers  ${rc}  0

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -before-reboot-2
    Reboot VM  %{VCH-NAME}
    Wait For VCH Initialization  30x  10 seconds

    # Remove image by short ID
    ${rc}  ${busybox-shortID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${busybox-shortID}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${busybox-shortID}

    # Remove image by long ID
    ${rc}  ${alpine-shortID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q ${alpine}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${alpineID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${alpine} | jq -r '.[0].Id'
    Should Be Equal As Integers  ${rc}  0
    ${alpine-longID}=  Fetch From Right  ${alpineID}  sha256:
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${alpine-longID}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q ${alpine}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${alpine-shortID}
