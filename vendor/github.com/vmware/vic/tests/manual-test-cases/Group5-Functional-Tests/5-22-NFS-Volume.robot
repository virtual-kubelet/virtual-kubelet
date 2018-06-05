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
Documentation  Test 5-22 - NFS Volume
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Setup ESX And NFS Suite
Suite Teardown  Run Keyword And Ignore Error  NFS Volume Cleanup
Test Teardown  Gather NFS Logs

*** Variables ***
${nfsVolumeStore}=  nfsVolumeStore
${nfsFakeVolumeStore}=  nfsFakeVolumeStore
${nfsReadOnlyVolumeStore}=  nfsReadOnlyVolumeStore
${unnamedNFSVolContainer}  unnamedNFSvolContainer
${namedNFSVolContainer}  namednfsVolContainer
${createFileContainer}=  createFileContainer
${nfs_bogon_ip}=  198.51.100.1
${mntDataTestContainer}=  mount-data-test
${mntTest}=  /mnt/test
${mntNamed}=  /mnt/named


*** Keywords ***
Setup ESX And NFS Suite
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    Log To Console  \nStarting test...

    ${esx1}  ${esx1_ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Open Connection  %{NIMBUS_GW}
    Wait Until Keyword Succeeds  2 min  30 sec  Login  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    ${POD}=  Fetch POD  ${esx1}
    Log To Console  ${POD}
    Close Connection

    ${nfs}  ${nfs_ip}=  Deploy Nimbus NFS Datastore  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  additional-args=--nimbus ${POD}

    ${nfs_readonly}  ${nfs_readonly_ip}=  Deploy Nimbus NFS Datastore  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  additional-args=--disk 5000000 --disk 5000000 --mountOpt ro --nfsOpt ro --mountPoint=storage1 --mountPoint=storage2 --nimbus ${POD}

    Set Suite Variable  @{list}  ${esx1}  ${nfs}  ${nfs_readonly}
    Set Suite Variable  ${ESX1}  ${esx1}
    Set Suite Variable  ${ESX1_IP}  ${esx1_ip}
    Set Suite Variable  ${NFS_IP}  ${nfs_ip}
    Set Suite Variable  ${NFS}  ${nfs}
    Set Suite Variable  ${NFS_READONLY_IP}  ${nfs_readonly_ip}

    # Enable logging on the nfs servers
    ${out}=  Run Keyword And Ignore Error  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_IP} rpcdebug -m nfsd -s all
    ${out}=  Run Keyword And Ignore Error  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_IP} rpcdebug -m rpc -s all
    ${out}=  Run Keyword And Ignore Error  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_IP} service rpcbind restart
    ${out}=  Run Keyword And Ignore Error  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_READONLY_IP} rpcdebug -m nfsd -s all
    ${out}=  Run Keyword And Ignore Error  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_READONLY_IP} rpcdebug -m rpc -s all
    ${out}=  Run Keyword And Ignore Error  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_READONLY_IP} service rpcbind restart

Setup ENV Variables for VIC Appliance Install
    Log To Console  \nSetup Environment Variables for VIC Appliance To ESX\n

    Set Environment Variable  TEST_URL_ARRAY  ${ESX1_IP}
    Set Environment Variable  TEST_URL  ${ESX1_IP}
    Set Environment Variable  TEST_USERNAME  root
    Set Environment Variable  TEST_PASSWORD  ${NIMBUS_ESX_PASSWORD}
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  HOST_TYPE  ESXi
    Remove Environment Variable  TEST_DATACENTER
    Remove Environment Variable  TEST_RESOURCE
    Remove Environment Variable  BRIDGE_NETWORK
    Remove Environment Variable  PUBLIC_NETWORK

Verify NFS Volume Basic Setup
    [Arguments]  ${volumeName}  ${containerName}  ${nfsIP}  ${rwORro}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${containerName} -v ${volumeName}:/mydata ${busybox} mount
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${nfsIP}://store/volumes/${volumeName}
    Should Contain  ${output}  /mydata type nfs (${rwORro}

    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${containerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm ${containerName}
    Should Be Equal As Integers  ${rc}  0

Verify NFS Volume Already Created
    [Arguments]  ${containerVolName}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=${containerVolName} --opt VolumeStore=${nfsVolumeStore}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: A volume named ${containerVolName} already exists. Choose a different volume name.

Reboot VM and Verify Basic VCH Info
    Log To Console  Rebooting VCH\n - %{VCH-NAME}
    Reboot VM  %{VCH-NAME}

    Wait For VCH Initialization  24x

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${busybox}

Gather NFS Logs
    ${out}=  Run Keyword And Continue On Failure  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_IP} dmesg -T
    Log  ${out}
    ${out}=  Run Keyword And Continue On Failure  Run  sshpass -p %{DEPLOYED_PASSWORD} ssh -o StrictHostKeyChecking\=no root@${NFS_READONLY_IP} dmesg -T
    Log  ${out}

NFS Volume Cleanup
    Gather NFS Logs
    Nimbus Cleanup  ${list}

*** Test Cases ***
VIC Appliance Install with Read Only NFS Volume
    Setup ENV Variables for VIC Appliance Install

    # Will only produce a warning in VCH creation output
    ${output}=  Install VIC Appliance To Test Server  certs=${false}  additional-args=--volume-store="nfs://${NFS_READONLY_IP}/exports/storage1?uid=0&gid=0:${nfsReadOnlyVolumeStore}"
    Should Contain  ${output}  Installer completed successfully
    Should Contain  ${output}  VolumeStore (${nfsReadOnlyVolumeStore}) cannot be brought online - check network, nfs server, and --volume-store configurations
    Should Contain  ${output}  Not all configured volume stores are online - check port layer log via vicadmin

    ${rc}  ${volumeOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --opt VolumeStore=${nfsReadOnlyVolumeStore}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${volumeOutput}  Error response from daemon: No volume store named (${nfsReadOnlyVolumeStore}) exists

VIC Appliance Install With Fake NFS Server
    Setup ENV Variables for VIC Appliance Install

    # Will only produce a warning in VCH creation output
    ${output}=  Install VIC Appliance To Test Server  certs=${false}  additional-args=--volume-store="nfs://${nfs_bogon_ip}/store?uid=0&gid=0:${nfsFakeVolumeStore}"
    Should Contain  ${output}  VolumeStore (${nfsFakeVolumeStore}) cannot be brought online - check network, nfs server, and --volume-store configurations

VIC Appliance Install With Correct NFS Server
    Setup ENV Variables for VIC Appliance Install
    Log To Console  \nDeploy VIC Appliance To ESX

    # Should succeed
    ${output}=  Install VIC Appliance To Test Server  certs=${false}  additional-args=--volume-store="nfs://${NFS_IP}/store?uid=0&gid=0:${nfsVolumeStore}"
    Should Contain  ${output}  Installer completed successfully

Simple Docker Volume Create
    #Pull image  ${busybox}

    ${rc}  ${volumeOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --opt VolumeStore=${nfsVolumeStore}
    Should Be Equal As Integers  ${rc}  0

    Set Suite Variable  ${nfsUnNamedVolume}  ${volumeOutput}

    Verify NFS Volume Basic Setup  ${nfsUnNamedVolume}  ${unnamedNFSVolContainer}  ${NFS_IP}  rw

Docker Volume Create Named Volume
    ${rc}  ${volumeOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name nfs-volume_%{VCH-NAME} --opt VolumeStore=${nfsVolumeStore}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${volumeOutput}  nfs-volume_%{VCH-NAME}

    Set Suite Variable  ${nfsNamedVolume}  ${volumeOutput}

    Verify NFS Volume Basic Setup  nfs-volume_%{VCH-NAME}  ${namedNFSVolContainer}  ${NFS_IP}  rw

Docker Volume Create Already Named Volume
    Run Keyword And Ignore Error  Verify NFS Volume Already Created  ${nfsUnNamedVolume}

    Run Keyword And Ignore Error  Verify NFS Volume Already Created  ${nfsNamedVolume}

Docker Volume Create with possibly Invalid Name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name="test!@\#$%^&*()" --opt VolumeStore=${nfsVolumeStore}
    Should Be Equal As Integers  ${rc}  1
    Should Be Equal As Strings  ${output}  Error response from daemon: volume name "test!@\#$%^&*()" includes invalid characters, only "[a-zA-Z0-9][a-zA-Z0-9_.-]" are allowed

Docker Single Write and Read to/from File from one Container using NFS Volume
    # Done with the same container for this test.
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${createFileContainer} -d -v ${nfsNamedVolume}:/mydata ${busybox} /bin/top -d 600
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -i ${createFileContainer} sh -c "echo 'The Texas and Chile flag look similar.\n' > /mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -i ${createFileContainer} sh -c "ls mydata/"
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  test_nfs_file.txt

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -i ${createFileContainer} sh -c "cat mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  The Texas and Chile flag look similar.

Docker Multiple Writes from Multiple Containers (one at a time) and Read from Another
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "echo 'The Chad and Romania flag look the same.\n' >> /mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "echo 'The Luxembourg and the Netherlands flag look exactly the same.\n' >> /mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "echo 'Norway and Iceland have flags that are basically inverses of each other.\n' >> /mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "cat mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  The Texas and Chile flag look similar.
    Should Contain  ${output}  The Chad and Romania flag look the same.
    Should Contain  ${output}  The Luxembourg and the Netherlands flag look exactly the same.
    Should Contain  ${output}  Norway and Iceland have flags that are basically inverses of each other.

Docker Read and Remove File
    ${rc}  ${catID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "cat mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${catID}
    Should Contain  ${output}  Norway and Iceland have flags that are basically inverses of each other.

    ${rc}  ${removeID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "rm mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "cat mydata/test_nfs_file.txt"
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  cat: can't open 'mydata/test_nfs_file.txt': No such file or directory

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${catID}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${catID}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  cat: can't open 'mydata/test_nfs_file.txt': No such file or directory

Simultaneous Container Write to File
    @{inputList}=  Create List  These flags also look similar to each other.  Senegal and Mali.  Indonesia and Monaco.  New Zealand and Australia.  Venezuela, Ecuador, and Colombia.  Slovenia, Russia, and Slovakia.
    ${containers}=  Create List

    Log To Console  \nSpin up Write Containers
    :FOR  ${item}  IN  @{inputList}
    \   ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "while true; do echo ${item} >> /mydata/test_nfs_mult_write.txt; sleep 5; done"
    \   Should Be Equal As Integers  ${rc}  0
    \   Append To List  ${containers}  ${id}

    ${rc}  ${catOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "tail -40 /mydata/test_nfs_mult_write.txt"
    Should Be Equal As Integers  ${rc}  0

    Log To Console  \nCheck tail output for write items
    :FOR  ${item}  IN  @{inputList}
    \   Should Contain  ${catOutput}  ${item}

    Log To Console  \nStop Write Containers
    :FOR  ${id}  IN  @{containers}
    \   ${rc}  ${stopOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${id}
    \   Should Be Equal As Integers  ${rc}  0


Simple Docker Volume Inspect
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume inspect ${nfsNamedVolume}
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Evaluate  json.loads(r'''${output}''')  json
    ${id}=  Get From Dictionary  ${output[0]}  Name
    Should Be Equal As Strings  ${id}  ${nfsNamedVolume}

Simple Volume ls Test
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  vsphere
    Should Contain  ${output}  ${nfsNamedVolume}
    Should Contain  ${output}  ${nfsUnNamedVolume}

    Should Contain  ${output}  DRIVER
    Should Contain  ${output}  VOLUME NAME

Volume rm Tests
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume rm ${nfsUnNamedVolume}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${nfsUnNamedVolume}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume rm ${nfsNamedVolume}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: volume ${nfsNamedVolume} in use by

Docker Inspect Mount Data after Reboot
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${mntDataTestContainer} -v ${mntTest} -v ${nfsNamedVolume}:${mntNamed} ${busybox}
    Should Be Equal As Integers  ${rc}  0

    # Create check list for Volume Inspect
    @{checkList}=  Create List  ${mntTest}  ${mntNamed}  ${nfsNamedVolume}

    Verify Volume Inspect Info  Before VM Reboot  ${mntDataTestContainer}  ${checkList}

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -before-reboot

    Reboot VM and Verify Basic VCH Info

    Verify Volume Inspect Info  After VM Reboot  ${mntDataTestContainer}  ${checkList}


Kill NFS Server
    Sleep  5 minutes
    ${rc}  ${runningContainer}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "while true; do echo 'Still here...\n' >> /mydata/test_nfs_kill.txt; sleep 2; done"
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${tailOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "tail -5 /mydata/test_nfs_kill.txt"
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${tailOutput}  Still here...

    Kill Nimbus Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}  ${NFS}

    ${status}=  Get State Of Github Issue  5946
    Run Keyword If  '${status}' == 'closed'  Fail  Test 5-22-NFS-Volume.robot needs to be updated now that Issue #5946 has been resolved
    # Issue 5946 should provide a better error message for the next three tests

    ${rc}  ${tailOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "tail -5 /mydata/test_nfs_kill.txt"
    Should Be Equal As Integers  ${rc}  125
    #Should Contain  ${tailOutput}  Server error from portlayer: unable to wait for process launch status:

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "echo 'Where am I writing to?...\n' >> /mydata/test_nfs_kill.txt"
    Should Be Equal As Integers  ${rc}  125

    ${rc}  ${lsOutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v ${nfsNamedVolume}:/mydata ${busybox} sh -c "ls mydata"
    Should Be Equal As Integers  ${rc}  125
    #Should Contain  ${lsOutput}  Server error from portlayer: unable to wait for process launch status:
