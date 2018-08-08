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
Documentation  Test 1-11 - Docker RM
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Check That VM Is Removed
    [Arguments]  ${container}
    ${id}=  Get container shortID  ${container}
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${id}

Check That Datastore Is Cleaned
    [Arguments]  ${container}
    ${rc}  ${output}=  Run And Return Rc And Output  govc datastore.ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${container}

*** Test Cases ***
Basic docker remove container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} dmesg
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check That VM Is Removed  ${container}
    Wait Until Keyword Succeeds  10x  6s  Check That Datastore Is Cleaned  ${container}

Remove a stopped container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} ls
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Container Stops  ${container}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check That VM Is Removed  ${container}
    Wait Until Keyword Succeeds  10x  6s  Check That Datastore Is Cleaned  ${container}

Remove a running container
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: You cannot remove a running container. Stop the container before attempting removal or use -f

Force remove a running container
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check That VM Is Removed  ${container}
    Wait Until Keyword Succeeds  10x  6s  Check That Datastore Is Cleaned  ${container}

Remove a fake container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm fakeContainer
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: No such container: fakeContainer

Remove a container deleted out of band
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name testRMOOB -p 80:8080 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    # Remove container VM out-of-band
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.destroy "testRMOOB*"
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Not Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Contain  ${output}  govc: ServerFaultCode: The method is disabled by 'VIC'
    Pass Execution If  '%{HOST_TYPE}' == 'VC'  Remaining steps not applicable on VC - skipping

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm testRMOOB
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: No such container: testRMOOB
    # now recreate the same container to ensure it's completely deleted
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name testRMOOB -p 80:8080 ${busybox}
    Should Be Equal As Integers  ${rc}  0

Remove a container created with unknown executable
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} xxxx
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${container}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check That VM Is Removed  ${container}

Remove a container and its anonymous volumes
    ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
    Set Test Variable  ${namedvol}  namedvol-${suffix}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    # Verify that for a container with an anon and a named vol, only the anon vol gets removed
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create ${namedvol}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${c1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -v /foo -v ${namedvol}:/bar ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${c1} | jq -c '.[0].Mounts'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${vol1}=  Run And Return Rc And Output  echo '${output}' | jq -r '.[0].Name'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${vol2}=  Run And Return Rc And Output  echo '${output}' | jq -r '.[1].Name'
    Should Be Equal As Integers  ${rc}  0
    ${anonvol}=  Set Variable If  '${vol1}' == '${namedvol}'  ${vol2}  ${vol1}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -v ${c1}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${anonvol}
    Should Contain  ${output}  ${namedvol}

    # Verify that for a container with an anon vol and another container with that vol as a named vol, the vol isn't removed
    ${rc}  ${c2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -v /foo ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${anonvol}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${c2} | jq -r '.[0].Mounts[0].Name'
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${c3}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -v ${anonvol}:/bar ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -v ${c2}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Contain  ${output}  ${anonvol}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -v ${c3}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Contain  ${output}  ${anonvol}

    # Verify that the above volume can be used by containers
    ${rc}  ${c4}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -v ${anonvol}:/bar ${busybox} /bin/ls
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${c4}
    Should Be Equal As Integers  ${rc}  0

    # Verify that only anonymous volumes are removed when superseding an image volume with a named volume
    ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
    Set Test Variable  ${namedImageVol}  non-anonymous-image-volume-${suffix}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name ${namedImageVol}
    Should Be Equal As Integers  ${rc}  0
    Set Test Variable  ${imageVolumeContainer}  I-Have-Two-Anonymous-Volumes-${suffix}
    ${rc}  ${c5}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${imageVolumeContainer} -v ${namedImageVol}:/data/db -v /I/AM/ANONYMOOOOSE mongo
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Contain  ${output}  ${namedImageVol}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -v ${imageVolumeContainer}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Contain  ${output}  ${namedImageVol}

