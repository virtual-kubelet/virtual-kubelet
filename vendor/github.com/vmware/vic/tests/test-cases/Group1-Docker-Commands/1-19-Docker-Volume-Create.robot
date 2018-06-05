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
Documentation  Test 1-19 - Docker Volume Create
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Simple docker volume create
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create
    Should Be Equal As Integers  ${rc}  0
    Set Suite Variable  ${ContainerName}  unnamedSpecVolContainer
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v ${output}:/mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${rc}  ${disk-size}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${disk-size}  975.9M

Simple volume mounted over managed files
    ${status}=  Get State Of Github Issue  5731
    Run Keyword If  '${status}' == 'closed'  Fail  Test should pass now that Issue #5731 has been resolved

    #${rc}  ${target}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit busybox
    #Should Be Equal As Integers  ${rc}  0
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v /etc busybox ping -c2 ${target}
    #Should Be Equal As Integers  ${ContainerRC}  0
    #Should Contain  ${output}  2 packets transmitted, 2 packets received

Docker volume create named volume
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${output}  test
    Set Suite Variable  ${ContainerName}  specVolContainer
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v ${output}:/mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${rc}  ${disk-size}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${disk-size}  975.9M

Docker volume create image volume
    Set Suite Variable  ${ContainerName}  imageVolContainer
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d mongo /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${rc}  ${disk-size}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${disk-size}  976M

Docker volume create anonymous volume
    Set Suite Variable  ${ContainerName}  anonVolContainer
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v /mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${rc}  ${disk-size}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${disk-size}  975.9M

Docker volume create already named volume
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: A volume named test already exists. Choose a different volume name.

Docker volume create volume with bad driver
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create -d fakeDriver --name=test2
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  error looking up volume plugin fakeDriver: plugin not found

Docker volume create with bad volumestore
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test3 --opt VolumeStore=fakeStore
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  No volume store named (fakeStore) exists

Docker volume create with bad driver options
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test3 --opt bogus=foo
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  bogus is not a supported option

Docker volume create with mis-capitalized valid driver option
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test4 --opt cAPACITy=10000
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${output}  test4
    Set Suite Variable  ${ContainerName}  capacityVolContainer
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v ${output}:/mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${rc}  ${disk-size}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${disk-size}  9.5G

Docker volume create with specific capacity no units
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test5 --opt Capacity=100000
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${output}  test5
    Set Suite Variable  ${ContainerName}  capacityVolContainer2
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v ${output}:/mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${rc}  ${disk-size}=  Run And Return Rc And Output  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${disk-size}  96.0G

Docker volume create large volume specifying units
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=unitVol1 --opt Capacity=10G
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${output}  unitVol1
    Set Suite Variable  ${ContainerName}  unitContainer
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v ${output}:/mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${disk-size}=  Run  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Contain  ${disk-size}  9.5G
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=unitVol2 --opt Capacity=10000
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal As Strings  ${output}  unitVol2
    Set Suite Variable  ${ContainerName}  unitContainer2
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${ContainerName} -d -v ${output}:/mydata ${busybox} /bin/df -Ph
    Should Be Equal As Integers  ${rc}  0
    ${ContainerRC}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} wait ${ContainerName}
    Should Be Equal As Integers  ${ContainerRC}  0
    Should Not Contain  ${output}  Error response from daemon
    ${disk-size}=  Run  docker %{VCH-PARAMS} logs ${ContainerName} | grep by-label | awk '{print $2}'
    Should Contain  ${disk-size}  9.5G

Docker volume create with zero capacity
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test5 --opt Capacity=0
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: bad driver value - Invalid size: 0

Docker volume create with negative one capacity
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test6 --opt Capacity=-1
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: bad driver value - Invalid size: -1

Docker volume create with capacity too big
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test7 --opt Capacity=9223372036854775808
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: bad driver value - Capacity value too large: 9223372036854775808

Docker volume create with capacity exceeding int size
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test8 --opt Capacity=9999999999999999999
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Error response from daemon: bad driver value - Capacity value too large: 9999999999999999999

Docker volume create with possibly invalid name
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=test???
    Should Be Equal As Integers  ${rc}  1
    Should Be Equal As Strings  ${output}  Error response from daemon: volume name "test???" includes invalid characters, only "[a-zA-Z0-9][a-zA-Z0-9_.-]" are allowed

Docker volume verify anonymous volume contains base image files
    ${status}=  Get State Of Github Issue  7365
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-19-Docker-Volume-Create.robot needs to be updated now that Issue #7365 has been resolved
#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name verify-anon-1 jakedsouza/group-1-19-docker-verify-volume-files:1.0 ls /etc/example
#    Should Be Equal As Integers  ${rc}  0
#    Should Contain  ${output}  thisshouldexist
#    Should Contain  ${output}  testfile.txt

#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name verify-anon-2 jakedsouza/group-1-19-docker-verify-volume-files:1.0 cat /etc/example/testfile.txt
#    Should Be Equal As Integers  ${rc}  0
#    Should Contain  ${output}  TestFile

#Docker volume verify named volume contains base image files
#	${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name verify-named-1 -v test15:/etc/example jakedsouza/group-1-19-docker-verify-volume-files:1.0 cat /etc/example/testfile.txt
#    Should Be Equal As Integers  ${rc}  0
#    Should Contain  ${output}  TestFile

	# Verify file is copied to volumeA
#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name verify-named-2 -v test15:/mnt/test15 jakedsouza/group-1-19-docker-verify-volume-files:1.0 cat /mnt/test15/testfile.txt
#    Should Be Equal As Integers  ${rc}  0
#    Should Contain  ${output}  TestFile

#Docker volume verify files are not copied again in a non empty volume
#    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v test16:/etc/example jakedsouza/group-1-19-docker-verify-volume-files:1.0 sh -c "echo test16modified >> /etc/example/testfile.txt"
#    Should Be Equal As Integers  ${rc}  0
    # Verify modified file remains
#	${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -v test16:/etc/example jakedsouza/group-1-19-docker-verify-volume-files:1.0 cat /etc/example/testfile.txt
#	Should Be Equal As Integers  ${rc}  0
#	Should Contain  ${output}  test16modified

Docker volume conflict in new container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create
    Should Be Equal As Integers  ${rc}  0
    Set Suite Variable  ${volID}  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit -v ${volID}:/mydata ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit -v ${volID}:/mydata ${busybox}
    Should Be Equal As Integers  ${rc}  125
    Should Contain  ${output}  Error response from daemon
    Should Contain  ${output}  device ${volID} in use
