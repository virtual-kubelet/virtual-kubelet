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
Documentation  This resource provides helper functions for docker operations
Library  OperatingSystem
Library  Process

*** Keywords ***
Run Docker Info
    [Arguments]  ${docker-params}
    ${rc}=  Run And Return Rc  docker ${docker-params} info
    Should Be Equal As Integers  ${rc}  0

Pull image
    [Arguments]  ${image}
    [Timeout]  10 minutes
    Log To Console  \nRunning docker pull ${image}...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Digest:
    Should Contain  ${output}  Status:
    Should Not Contain  ${output}  No such image:

Wait Until Container Stops
    [Arguments]  ${container}  ${sleep-time}=1
    :FOR  ${idx}  IN RANGE  0  60
    \   ${out}=  Run  docker %{VCH-PARAMS} inspect -f '{{.State.Running}}' ${container}
    \   Return From Keyword If  '${out}' == 'false'
    \   Sleep  ${sleep-time}
    Fail  Container did not stop within 60 seconds

Hit Nginx Endpoint
    [Arguments]  ${vch-ip}  ${port}
    ${rc}  ${output}=  Run And Return Rc And Output  wget ${vch-ip}:${port}
    Should Be Equal As Integers  ${rc}  0

Get Container IP
    [Arguments]  ${docker-params}  ${id}  ${network}=bridge  ${dockercmd}=docker
    ${rc}  ${ip}=  Run And Return Rc And Output  ${dockercmd} ${docker-params} inspect --format='{{(index .NetworkSettings.Networks "${network}").IPAddress}}' ${id}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${ip}

Get IP Address of Container
    [Arguments]  ${container}
    ${ip}=  Run  docker %{VCH-PARAMS} inspect ${container} | jq -r ".[].NetworkSettings.Networks.bridge.IPAddress"
    [Return]  ${ip}

# The local dind version is embedded in Dockerfile
# docker:1.13-dind
# If you are running this keyword in a container, make sure it is run with --privileged turned on
Start Docker Daemon Locally
    [Arguments]  ${dockerd-params}  ${dockerd-path}=/usr/local/bin/dockerd-entrypoint.sh  ${log}=./daemon-local.log
    OperatingSystem.File Should Exist  ${dockerd-path}
    Log To Console  Starting docker daemon locally
    ${pid}=  Run  pidof dockerd
    Run Keyword If  '${pid}' != '${EMPTY}'  Run  kill -9 ${pid}
    Run Keyword If  '${pid}' != '${EMPTY}'  Log To Console  \nKilling local dangling dockerd process: ${pid}
    ${handle}=  Start Process  ${dockerd-path} ${dockerd-params} >${log} 2>&1  shell=True
    Process Should Be Running  ${handle}
    :FOR  ${IDX}  IN RANGE  5
    \   ${pid}=  Run  pidof dockerd
    \   Run Keyword If  '${pid}' != '${EMPTY}'  Set Test Variable  ${dockerd-pid}  ${pid}
    \   Exit For Loop If  '${pid}' != '${EMPTY}'
    \   Sleep  1s
    Should Not Be Equal  '${dockerd-pid}'  '${EMPTY}'
    :FOR  ${IDX}  IN RANGE  10
    \   ${rc}=  Run And Return Rc  DOCKER_API_VERSION=1.23 docker -H unix:///var/run/docker-local.sock ps
    \   Return From Keyword If  '${rc}' == '0'  ${handle}  ${dockerd-pid}
    \   Sleep  1s
    Fail  Failed to initialize local dockerd
    [Return]  ${handle}  ${dockerd-pid}

Kill Local Docker Daemon
    [Arguments]  ${handle}  ${dockerd-pid}
    Terminate Process  ${handle}
    Process Should Be Stopped  ${handle}
    ${rc}=  Run And Return Rc  kill -9 ${dockerd-pid}
    Should Be Equal As Integers  ${rc}  0

Get container shortID
    [Arguments]  ${id}
    ${shortID}=  Get Substring  ${id}  0  12
    [Return]  ${shortID}

Get container name
    [Arguments]  ${id}
    ${rc}  ${name}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect --format='{{.Name}}' ${id}
    Should Be Equal As Integers  ${rc}  0
    ${name}=  Get Substring  ${name}  1
    [Return]  ${name}

Get VM display name
    [Arguments]  ${id}
    ${name}=  Get container name  ${id}
    ${shortID}=  Get container shortID  ${id}
    [Return]  ${name}-${shortID}

Verify Container Rename
    [Arguments]  ${oldname}  ${newname}  ${contID}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${newname}
    Should Not Contain  ${output}  ${oldname}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.Name}}' ${newname}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${newname}
    ${vmName}=  Get VM display name  ${contID}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${vmname}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${vmName}

Run Regression Tests
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    # Pull an image that has been pulled already
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  busybox
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  /bin/top
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Container Stops  ${container}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Exited

    ${vmName}=  Get VM Display Name  ${container}
    Wait Until Keyword Succeeds  5x  10s  Check For The Proper Log Files  ${vmName}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  /bin/top

    # Check for regression for #1265
    ${rc}  ${container1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it ${busybox} /bin/top
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${shortname}=  Get Substring  ${container2}  1  12
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Log  ${output}
    ${lines}=  Get Lines Containing String  ${output}  ${shortname}
    Should Not Contain  ${lines}  /bin/top
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} rm ${container1}
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} rm ${container2}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${busybox}

    Scrape Logs For The Password

Launch Container
    [Arguments]  ${name}  ${network}=default  ${dockercmd}=docker
    ${rc}  ${output}=  Run And Return Rc And Output  ${dockercmd} %{VCH-PARAMS} run --name ${name} --net ${network} -itd busybox
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get Line  ${output}  -1
    ${ip}=  Get Container IP  %{VCH-PARAMS}  ${id}  ${network}  ${dockercmd}
    [Return]  ${id}  ${ip}

Start Container and Exec Command
    [Arguments]  ${containerName}  ${cmd}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${containerName}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec ${containerName} ${cmd}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    [Return]  ${output}

Verify Volume Inspect Info
    [Arguments]  ${inspectedWhen}  ${volTestContainer}  ${checkList}
    Log To Console  \nContainer Mount Inspected ${inspectedWhen}
    ${rc}  ${mountInfo}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.Mounts}}' ${volTestContainer}
    Should Be Equal As Integers  ${rc}  0

    :FOR  ${item}  IN  @{checkList}
    \   Should Contain  ${mountInfo}  ${item}

Log All Containers
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Run Keyword If  ${rc} != 0  Log To Console  Remaining containers - ${out}

Do Images Exist
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q
    ${len}=  Get Length  ${output}
    Return From Keyword If  ${len} == 0  ${false}
    [Return]  ${true}

Do Containers Exist
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a -q
    ${len}=  Get Length  ${output}
    Return From Keyword If  ${len} == 0  ${false}
    [Return]  ${true}

Do Volumes Exist
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls -q
    ${len}=  Get Length  ${output}
    Return From Keyword If  ${len} == 0  ${false}
    [Return]  ${true}

Do Networks Exist
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls -q
    ${len}=  Get Length  ${output}
    Return From Keyword If  ${len} == 0  ${false}
    [Return]  ${true}

Remove All Containers
    ${exist}=  Do Containers Exist
    Run Keyword If  ${exist}  Log To Console  Stopping and removing all containers from %{VCH-NAME}
    Return From Keyword If  ${exist} == ${false}  0

    # Run Keyword If  ${exist}  Kill All Containers
    Run Keyword If  ${exist}  Run  docker %{VCH-PARAMS} rm -f $(docker %{VCH-PARAMS} ps -a -q)

    ${exist}=  Do Containers Exist
    Return From Keyword If  ${exist} == ${false}  0
    Run Keyword If  ${exist}  Log All Containers
    [Return]  1

Stop All Containers
    Run  docker %{VCH-PARAMS} stop $(docker %{VCH-PARAMS} ps -q)

Kill All Containers
    Run  docker %{VCH-PARAMS} kill $(docker %{VCH-PARAMS} ps -q)

Remove All Images
    ${exist}=  Do Images Exist
    Run Keyword If  ${exist}  Log To Console  Removing all images from %{VCH-NAME}

    Return From Keyword If  ${exist} == ${false}  0
    Run Keyword If  ${exist}  Remove All Containers
    Run Keyword If  ${exist}  Run  docker %{VCH-PARAMS} rmi $(docker %{VCH-PARAMS} images -q)

    ${exist}=  Do Images Exist
    Return From Keyword If  ${exist} == ${false}  0
    [Return]  1

Remove All Volumes
    ${exist}=  Do Volumes Exist
    Run Keyword If  ${exist}  Log To Console  Removing all volumes from %{VCH-NAME}

    Return From Keyword If  ${exist} == ${false}  0
    Run Keyword If  ${exist}  Run  docker %{VCH-PARAMS} volume rm $(docker %{VCH-PARAMS} volume ls -q)

    ${exist}=  Do Volumes Exist
    Return From Keyword If  ${exist} == ${false}  0
    [Return]  1

Remove All Container Networks
    ${exist}=  Do Networks Exist
    Run Keyword If  ${exist}  Log To Console  Removing all container networks from %{VCH-NAME}

    Return From Keyword If  ${exist} == ${false}  0
    Run Keyword If  ${exist}  Run  docker %{VCH-PARAMS} network rm $(docker %{VCH-PARAMS} network ls -q)

    ${exist}=  Do Networks Exist
    Return From Keyword If  ${exist} == ${false}  0
    [Return]  1

Add List To Dictionary
    [Arguments]  ${dict}  ${list}
    : FOR  ${item}  IN  @{list}
    \    Set To Dictionary  ${dict}  ${item}  1

List Existing Images On VCH
    # Get list of image IDs
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images -q
    ${len}=  Get Length  ${output}
    Run Keyword If  ${len} != 0  Log To Console  Found images on %{VCH-NAME}:
    ${image_ids}=  Run Keyword If  ${rc} == 0  Split String  ${output}
    ${tags_dict}=  Create Dictionary
    : FOR  ${id}  IN  @{image_ids}
    \    ${rc}  ${repotags}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect --format='{{.RepoTags}}' --type=image ${id}
    \    ${clean_tags}  Strip String  ${repotags}  characters=[]
    \    ${tags}=  Split String  ${clean_tags}
    \    Add List To Dictionary  ${tags_dict}  ${tags}

    : FOR  ${tag}  IN  @{tags_dict.keys()}
    \    Log To Console  \t${tag}

List Running Containers On VCH
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -q
    Log To Console  ${EMPTY}
    ${len}=  Get Length  ${output}
    Run Keyword If  ${len} != 0  Log To Console  Found running containers on %{VCH-NAME}:
    ...  ELSE  Log To Console  No running containers on %{VCH-NAME}
    Return From Keyword If  ${len} == 0

    ${cids}=  Run Keyword If  ${len} != 0  Split String  ${output}
    : FOR  ${id}  IN  @{cids}
    \    Log To Console  \t${id}
