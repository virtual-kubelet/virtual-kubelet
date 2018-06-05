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
Documentation  Test 1-02 - Docker Pull
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Pull nginx
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  ${nginx}

Pull busybox
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  ${busybox}

Pull ubuntu
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  ${ubuntu}

Pull non-default tag
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  nginx:alpine

Pull images based on digest
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull nginx@sha256:7281cf7c854b0dfc7c68a6a4de9a785a973a14f1481bc028e2022bcd6a8d9f64
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  No such image:
    Should Contain  ${output}  Digest: sha256:7281cf7c854b0dfc7c68a6a4de9a785a973a14f1481bc028e2022bcd6a8d9f64
    Should Contain  ${output}  Status: Downloaded newer image for library/nginx:sha256:7281cf7c854b0dfc7c68a6a4de9a785a973a14f1481bc028e2022bcd6a8d9f64

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  No such image:
    Should Contain  ${output}  Digest: sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2
    Should Contain  ${output}  Status: Downloaded newer image for library/ubuntu:sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2

Pull an image with the full docker registry URL
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  registry.hub.docker.com/library/hello-world

Pull an image with all tags
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  --all-tags nginx

Pull non-existent image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull fakebadimage
    Log  ${output}
    Should Be Equal As Integers  ${rc}  1
    Should contain  ${output}  image library/fakebadimage not found

Pull image from non-existent repo
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull fakebadrepo.com:9999/ubuntu
    Log  ${output}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  no such host

Pull image with a tag that doesn't exist
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox:faketag
    Log  ${output}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Tag faketag not found in repository library/busybox

Pull image that already has been pulled
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  ${alpine}
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  ${alpine}

Pull the same image concurrently
    ${pids}=  Create List

    # Create 5 processes to pull the same image at once
    :FOR  ${idx}  IN RANGE  0  5
    \   ${pid}=  Start Process  docker %{VCH-PARAMS} pull ${redis}  shell=True
    \   Append To List  ${pids}  ${pid}

    # Wait for them to finish and check their output
    :FOR  ${pid}  IN  @{pids}
    \   ${res}=  Wait For Process  ${pid}
    \   Log  ${res.stdout}
    \   Log  ${res.stderr}
    \   Should Be Equal As Integers  ${res.rc}  0
    \   Should Contain  ${res.stdout}  Downloaded newer image for default-project/redis:latest

Pull two images that share layers concurrently
     ${pid1}=  Start Process  docker %{VCH-PARAMS} pull golang:1.7  shell=True
     ${pid2}=  Start Process  docker %{VCH-PARAMS} pull golang:1.6  shell=True

    # Wait for them to finish and check their output
    ${res1}=  Wait For Process  ${pid1}
    ${res2}=  Wait For Process  ${pid2}
    Should Be Equal As Integers  ${res1.rc}  0
    Should Be Equal As Integers  ${res2.rc}  0
    Should Contain  ${res1.stdout}  Downloaded newer image for library/golang:1.7
    Should Contain  ${res2.stdout}  Downloaded newer image for library/golang:1.6

Re-pull a previously rmi'd image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images |grep ${ubuntu}
    ${words}=  Split String  ${output}
    ${id}=  Get From List  ${words}  2
    ${size}=  Get From List  ${words}  -2
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${ubuntu}
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  ${ubuntu}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images |grep ${ubuntu}
    ${words}=  Split String  ${output}
    ${newid}=  Get From List  ${words}  2
    ${newsize}=  Get From List  ${words}  -2
    Should Be Equal  ${id}  ${newid}
    Should Be Equal  ${size}  ${newsize}

Pull image by multiple tags
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  busybox:1.25.1
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  busybox:1.25
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images |grep -E busybox.*1.25
    Should Be Equal As Integers  ${rc}  0
    ${lines}=  Split To Lines  ${output}
    # one for 1.25.1 and one for 1.25
    Length Should Be  ${lines}  2
    ${line1}=  Get From List  ${lines}  0
    ${line2}=  Get From List  ${lines}  -1
    ${words1}=  Split String  ${line1}
    ${words2}=  Split String  ${line2}
    ${id1}=  Get From List  ${words1}  2
    ${id2}=  Get From List  ${words2}  2
    Should Be Equal  ${id1}  ${id2}

Issue docker pull on digest outputted by previous pull
    ${status}=  Get State Of Github Issue  5187
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-02-Docker-Pull.robot needs to be updated now that Issue #5187 has been resolved

    # ${rc}  Run And Return Rc  docker %{VCH-PARAMS} rmi busybox
    # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox | grep Digest | awk '{print $2}'
    # Log  ${output}
    # Should Be Equal As Integers  ${rc}  0
    # Should Not Be Empty  ${output}
    # ${rc}  Run And Return Rc  docker %{VCH-PARAMS} rmi busybox
    # ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox@${output}
    # Log  ${output}
    # Should Be Equal As Integers  ${rc}  0
    # Should Contain  ${output}  Downloaded

Pull images from gcr.io
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  gcr.io/google_containers/hyperkube:v1.6.2
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  gcr.io/google_samples/gb-redisslave:v1
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  gcr.io/google_samples/cassandra:v11
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  gcr.io/google_samples/cassandra:v12

Verify image manifest digest against vanilla docker
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull busybox:1.26
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  sha256:be3c11fdba7cfe299214e46edc642e09514dbb9bbefcd0d3836c05a1e0cd0642
