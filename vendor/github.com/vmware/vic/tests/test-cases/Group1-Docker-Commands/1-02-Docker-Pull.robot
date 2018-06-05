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

*** Keywords ***
Get And Run MITMProxy Container
    # Need to change this container? Read README.md in vic/tests/resources/dockerfiles/docker-pull-mitm-proxy
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  victest/docker-layer-injection-proxy:latest
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --network public -itd --name=mitm -p 8080:8080 victest/docker-layer-injection-proxy
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Get And Run Prepared Registry
    # Need to change this container? Read README.md in vic/tests/resources/dockerfiles/docker-pull-mitm-proxy
    Wait Until Keyword Succeeds  5x  15 seconds  Pull image  victest/registry-busybox:latest
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd --network public --name=registry -p 5000:5000 victest/registry-busybox
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Run Docker Ps
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${output}

Get Container Address
    [Arguments]  ${container-name}  ${docker-ps}
    ${rc}  ${container}=  Run And Return Rc And Output  echo '${docker-ps}' | grep ${container-name} | awk '{print $(NF-1)}' | cut -d- -f1
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${container}

Pull And MITM Prepared Image
    [Arguments]  ${vch2-params}  ${registry}
    ${rc}  ${output}=  Run And Return Rc And Output  docker ${vch2-params} pull ${registry}/busybox
    Log  ${output}

Enable SSH On MITMed VCH
    ${rc}  ${thumbprint}=  Run And Return Rc And Output  govc about.cert -k -json | jq -r .ThumbprintSHA1
    Log  ${thumbprint}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux debug --rootpw password --target %{TEST_URL}%{TEST_DATACENTER} --password %{TEST_PASSWORD} --name VCH-XPLT --user %{TEST_USERNAME} --compute-resource %{TEST_RESOURCE} --enable-ssh --thumbprint=${thumbprint}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Check For Injected Binary
    [Arguments]  ${vch2-IP}
    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -ppassword ssh ${vch2-IP} -lroot -C -oStrictHostKeyChecking=no "ls /tmp | grep pingme"
    Log  ${output}
    Should Not Contain  ${output}  pingme

Deploy Proxified VCH
    [Arguments]  ${registry}  ${mitm}
    ${br1}=  Get Environment Variable  BRIDGE_NETWORK
    Create Unique Bridge Network
    # BRIDGE_NETWORK gets overwritten by Create Unique Bridge Network. Assign original value to BRIDGE_NETWORK_2 for removal during suite teardown

    # Run VIC Machine Command assumes %{VCH-NAME}
    # Install VIC Appliance on Test Server assumes a bunch of environment variables
    # We're just going to eschew helpers and install this VCH manually to avoid mutating hidden environmental state which is difficult to debug
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux create --name=VCH-XPLT --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --bridge-network=%{BRIDGE_NETWORK} --container-network=%{PUBLIC_NETWORK}:public --public-network=%{PUBLIC_NETWORK} ${vicmachinetls} --image-store=%{TEST_DATASTORE} --insecure-registry=http://${registry} --http-proxy http://${mitm}
    Log  ${output}

    ${br2}=  Get Environment Variable  BRIDGE_NETWORK
    Set Environment Variable  BRIDGE_NETWORK_2  ${br2}
    # suite teardown fails if we don't set this back, and we're informed not to edit the Suite Teardown at the top of the file, so
    Set Environment Variable  BRIDGE_NETWORK  ${br1}

    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${vch2-params}=  Run And Return Rc And Output  echo '${output}' | grep -A1 "Connect to docker" | tail -n1 | cut -d' ' -f4- | sed 's/ info" $//g'

    # this comment fixes syntax highlighting "
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${vch2-IP}=  Run And Return Rc And Output  echo '${output}' | grep -A1 "Published ports" | tail -n1 | awk '{print $NF}' | cut -d= -f2
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${vch2-IP}  ${vch2-params}

Destroy Proxified VCH
    Run  bin/vic-machine-linux delete --name=VCH-XPLT --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true
    ${br1}=  Get Environment Variable  BRIDGE_NETWORK
    Set Environment Variable  BRIDGE_NETWORK  %{BRIDGE_NETWORK_2}
    Cleanup VCH Bridge Network
    Set Environment Variable  BRIDGE_NETWORK  ${br1}

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

Attempt docker pull mitm
    Get And Run MITMProxy Container
    Get And Run Prepared Registry
    ${ps}=  Run Docker Ps
    ${mitm}=  Get Container Address  mitm  ${ps}
    ${registry}=  Get Container Address  registry  ${ps}
    ${vch2-IP}  ${vch2-params}=  Deploy Proxified VCH  ${registry}  ${mitm}
    Pull And MITM Prepared Image  ${vch2-params}  ${registry}
    Enable SSH on MITMed VCH
    Check For Injected Binary  ${vch2-IP}
    [Teardown]  Destroy Proxified VCH
