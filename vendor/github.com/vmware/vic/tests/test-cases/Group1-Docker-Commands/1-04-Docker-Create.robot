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
Documentation  Test 1-04 - Docker Create
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Simple creates
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -t -i ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name test1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Create with anonymous volume
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -v /var/log ${busybox} ls /var/log
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs --follow ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Create with named volume
    Run  docker %{VCH-PARAMS} volume rm test-named-vol
    ${disk-size}=  Run  docker %{VCH-PARAMS} logs $(docker %{VCH-PARAMS} start $(docker %{VCH-PARAMS} create -v test-named-vol:/testdir ${busybox} /bin/df -Ph) && sleep 10) | grep by-label | awk '{print $2}'
    Should Contain  ${disk-size}  975.9M

Create with a directory as a volume
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -v /dir:/dir ${busybox}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: Bad request error from portlayer: mounting directories as a data volume is not supported.

Create with complex volume topology - overriding image volume with named volume
    # Verify that only anonymous volumes are removed when superseding an image volume with a named volume
    ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
    Set Test Variable  ${namedImageVol}  non-anonymous-image-volume-${suffix}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume rm ${namedImageVol}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name ${namedImageVol}
    Should Be Equal As Integers  ${rc}  0
    Set Test Variable  ${imageVolumeContainer}  I-Have-Two-Anonymous-Volumes-${suffix}
    ${rc}  ${c5}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${imageVolumeContainer} -v ${namedImageVol}:/data/db -v /I/AM/ANONYMOOOOSE mongo
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f "{{.HostConfig.Binds}}" ${imageVolumeContainer}
    Should Contain  ${output}  ${namedImageVol}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f "{{.Config.Volumes}}" ${imageVolumeContainer}
    Should Contain  ${output}  ${namedImageVol}

Create simple top example
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Create fakeimage image
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create fakeimage
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error: image library/fakeimage not found

Create fakeImage repository
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create fakeImage
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error parsing reference: "fakeImage" is not a valid repository/tag

Create and start named container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name busy1 ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start busy1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Create linked containers that can ping
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${debian}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --link busy1:busy1 --name busy2 ${debian} ping -c2 busy1
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start busy2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait busy2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs busy2
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2 packets transmitted, 2 packets received

Create a container after the last container is removed
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${cid}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${cid}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${cid}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${cid2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${cid2}  Error

Create a container from an image that has not been pulled yet
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${alpine} bash
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Create a container with no command specified
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull centos:6.6
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create centos:6.6
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: No command specified

Create a container with custom CPU count
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it --cpuset-cpus 3 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${id} |awk '/CPU:/ {print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  3

Create a container with custom amount of memory in GB
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -m 4G ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${id} |awk '/Memory:/ {print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  4096MB

Create a container with custom amount of memory in MB
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -m 2048M ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${id} |awk '/Memory:/ {print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2048MB

Create a container with custom amount of memory in KB
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -m 2097152K ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${id} |awk '/Memory:/ {print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2048MB

Create a container with custom amount of memory in Bytes
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -m 2147483648B ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${id} |awk '/Memory:/ {print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  2048MB

Create a container using rest api call without HostConfig in the form data
    ${status}=  Run Keyword And Return Status  Environment Variable Should Be Set  DOCKER_CERT_PATH
    ${certs}=  Set Variable If  ${status}  --cert %{DOCKER_CERT_PATH}/cert.pem --key %{DOCKER_CERT_PATH}/key.pem  ${EMPTY}

    ${output}=  Run  curl -sk ${certs} -H "Content-Type: application/json" -d '{"Image": "${busybox}", "Cmd": ["ping", "127.0.0.1"], "NetworkMode": "bridge"}' https://%{VCH-IP}:%{VCH-PORT}/containers/create
    Log  ${output}
    Should contain  ${output}  "Warnings":null

Create a container and check the VM display name and datastore folder name
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it --name busy3 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${vmName}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${vmName}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${vmName}
    ${rc}  ${output}=  Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Run And Return Rc And Output  govc datastore.ls | grep ${vmName}
    Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Should Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Should contain  ${output}  ${vmName}
    ${rc}  ${output}=  Run Keyword If  '%{DATASTORE_TYPE}' == 'Non_VSAN'  Run And Return Rc And Output  govc datastore.ls | grep ${id}
    Run Keyword If  '%{DATASTORE_TYPE}' == 'Non_VSAN'  Should Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{DATASTORE_TYPE}' == 'Non_VSAN'  Should Contain  ${output}  ${id}

Create disables VC destroy
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get VM display name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -json ${id} | jq .VirtualMachines[].DisabledMethod
    Should Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Contain  ${output}  Destroy_Task
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Not Contain  ${output}  Destroy_Task

Parallel creates with same container name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${suffix}=  Evaluate  '%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
    Set Test Variable  ${name}  testDuplicates-${suffix}
    ${pid1}=  Start Process  docker %{VCH-PARAMS} create --name ${name} ${busybox}  shell=True
    ${pid2}=  Start Process  docker %{VCH-PARAMS} create --name ${name} ${busybox}  shell=True
    ${res1}=  Wait For Process  ${pid1}
    ${res2}=  Wait For Process  ${pid2}

    # Only one process should succeed
    Run Keyword If  ${res1.rc} == 0  Should Not Be Equal As Integers  ${res2.rc}  0
    Run Keyword If  ${res2.rc} == 0  Should Not Be Equal As Integers  ${res1.rc}  0

    ${status1}  ${out1}=  Run Keyword And Ignore Error  Should Contain  ${res1.stderr}  is already in use
    ${status2}  ${out2}=  Run Keyword And Ignore Error  Should Contain  ${res2.stderr}  is already in use
    # Only and only one process's stderr should contain the error message
    Run Keyword If  '${status1}' == 'PASS'  Should Not Be Equal  '${status2}'  'PASS'
    Run Keyword If  '${status2}' == 'PASS'  Should Not Be Equal  '${status1}'  'PASS'

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -f status=created
    Should Be Equal As Integers  ${rc}  0
    Should Contain X Times  ${output}  ${name}  1

    # Verify that remove and re-create works for the same name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${name}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} ${busybox}
    Should Be Equal As Integers  ${rc}  0
