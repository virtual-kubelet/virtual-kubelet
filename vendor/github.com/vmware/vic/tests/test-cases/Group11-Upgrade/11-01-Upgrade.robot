# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

*** Settings ***
Documentation  Test 11-01 - Upgrade
Resource  ../../resources/Util.robot
Suite Setup  Disable Ops User And Install VIC To Test Server
Suite Teardown  Re-Enable Ops User And Clean Up VIC Appliance
Default Tags

*** Variables ***
${namedVolume}=  named-volume
${mntDataTestContainer}=  mount-data-test
${mntTest}=  /mnt/test
${mntNamed}=  /mnt/named
${run-as-ops-user}=  ${EMPTY}

*** Keywords ***
Disable Ops User And Install VIC To Test Server
    ${run-as-ops-user}=  Get Environment Variable  RUN_AS_OPS_USER  0
    Set Environment Variable  RUN_AS_OPS_USER  0
    Install VIC with version to Test Server  1.2.1

Re-Enable Ops User And Clean Up VIC Appliance
    Set Environment Variable  RUN_AS_OPS_USER  ${run-as-ops-user}
    Clean up VIC Appliance And Local Binary

Run Docker Checks
    # wait for docker info to succeed
    Log To Console  Verify Containers...
    Wait Until Keyword Succeeds  20x  5 seconds  Run Docker Info  %{VCH-PARAMS}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  bar
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect bridge
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get Container IP  %{VCH-PARAMS}  %{ID1}  bridge
    Should Be Equal  ${ip}  %{IP1}
    ${ip}=  Get Container IP  %{VCH-PARAMS}  %{ID2}  bridge
    Should Be Equal  ${ip}  %{IP2}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect vch-restart-test1
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  "Id"
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop vch-restart-test1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0


    ${status}=  Get State Of Github Issue  5653
    Run Keyword If  '${status}' == 'closed'  Should Contain  ${output}  Exited (143)
    Run Keyword If  '${status}' == 'closed'  Fail  Exit code check below needs to be updated now that Issue #5653 has been resolved
    # Disabling the precise check for error code until https://github.com/vmware/vic/issues/5653 is fixed - we can get rid of the
    # conditional around the exit code check once the issue is closed
    #Should Contain  ${output}  Exited (143)

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start vch-restart-test1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0

    ${status}=  Get State Of Github Issue  7534
    Run Keyword If  '${status}' == 'closed'  Fail  Exit code check below needs to be updated now that Issue #7534 has been resolved
    #Should Not Contain  ${output}  Exited (0)

    # Check that rename works on a container from a VCH that supports rename
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${contID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name new-vch-cont1 ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${contID}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rename new-vch-cont1 new-vch-cont2
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Verify Container Rename  new-vch-cont1  new-vch-cont2  ${contID}

    # check the display name and datastore folder name of an existing container
    ${id1shortID}=  Get container shortID  %{ID1}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info *-${id1shortID}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  vch-restart-test1-${id1shortID}
    ${rc}  ${output}=  Run And Return Rc And Output  govc datastore.ls | grep %{ID1}
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  %{ID1}

    # check the display name and datastore folder name of a new container
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    ${vmName}=  Get VM Display Name  ${id}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info ${vmName}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${vmName}
    ${rc}  ${output}=  Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Run And Return Rc And Output  govc datastore.ls | grep ${vmName}
    Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Should Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{DATASTORE_TYPE}' == 'VSAN'  Should contain  ${output}  ${vmName}
    ${rc}  ${output}=  Run Keyword If  '%{DATASTORE_TYPE}' == 'Non_VSAN'  Run And Return Rc And Output  govc datastore.ls | grep ${id}
    Run Keyword If  '%{DATASTORE_TYPE}' == 'Non_VSAN'  Should Be Equal As Integers  ${rc}  0
    Run Keyword If  '%{DATASTORE_TYPE}' == 'Non_VSAN'  Should Contain  ${output}  ${id}

    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  10000
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  10001

    # one of the ports collides
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 10001:80 -p 10002:80 --name webserver1 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start webserver1
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  port 10001 is not available

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs -n1 docker %{VCH-PARAMS} stop
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -aq | xargs -n1 docker %{VCH-PARAMS} rm
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Create Docker Containers
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.11 %{VCH-PARAMS} network create bar
    Should Be Equal As Integers  ${rc}  0
    Comment  Launch container on bridge network
    ${id1}  ${ip1}=  Launch Container  vch-restart-test1  bridge  docker1.11
    ${id2}  ${ip2}=  Launch Container  vch-restart-test2  bridge  docker1.11
    Set Environment Variable  ID1  ${id1}
    Set Environment Variable  ID2  ${id2}
    Set Environment Variable  IP1  ${ip1}
    Set Environment Variable  IP2  ${ip2}

    ${rc}  ${output}=  Run And Return Rc And Output  docker1.11 %{VCH-PARAMS} create -it -p 10000:80 -p 10001:80 --name webserver nginx
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.11 %{VCH-PARAMS} start webserver
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  10000
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  10001

Create Container with Named Volume
    Log To Console  \nCreate a named volume and mount it to a container\n
    ${rc}  ${container}=  Run And Return Rc And Output  docker1.11 %{VCH-PARAMS} volume create --name=${namedVolume}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${container}  ${namedVolume}

    ${rc}  ${output}=  Run And Return Rc And Output  docker1.11 %{VCH-PARAMS} create --name=${mntDataTestContainer} -v ${mntTest} -v ${namedVolume}:${mntNamed} busybox
    Should Be Equal As Integers  ${rc}  0
    Set Suite Variable  ${TestContainerVolume}  ${output}

Check Container Create Timestamps
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${newc}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create ${busybox}
    Should Be Equal As Integers  ${rc}  0

    # Container created with the older VCH should have timestamp in seconds
    ${rc}  ${oldoutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${TestContainerVolume} | jq -r '.[0].Created'
    Should Be Equal As Integers  ${rc}  0
    ${rest}  ${oldtstamp}=  Split String From Right  ${oldoutput}  :  1
    ${oldlen}=  Get Length  ${oldtstamp}
    Should Be Equal As Integers  ${oldlen}  3

    # Container created with the upgraded VCH should have timestamp in nanoseconds
    ${rc}  ${newoutput}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${newc} | jq -r '.[0].Created'
    Should Be Equal As Integers  ${rc}  0
    ${rest}  ${newtstamp}=  Split String From Right  ${newoutput}  :  1
    ${newlen}=  Get Length  ${newtstamp}
    Should Be True  ${newlen} >= 3

    # Containers should show a valid human-readable duration in ps output. This tests the
    # data migration plugin - the timestamp should be converted correctly to seconds
    # before sending the ps response to the docker client.

    # Pause for 2 seconds to allow for the time check below
    Sleep  2
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
    Should Be Equal As Integers  ${rc}  0
    ${oldcshortID}=  Get container shortID  ${TestContainerVolume}
    ${lines}=  Get Lines Containing String  ${output}  ${oldcshortID}
    Should Not Contain  ${lines}  years ago
    ${newcshortID}=  Get container shortID  ${newc}
    ${lines}=  Get Lines Containing String  ${output}  ${newcshortID}
    Should Not Contain  ${lines}  Less than a second ago

*** Test Cases ***
Upgrade Present in vic-machine
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux
    Should Contain  ${output}  upgrade

Upgrade VCH with unreasonably short timeout and automatic rollback after failure
    Log To Console  \nUpgrading VCH with 1s timeout ...
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout 1s
    Should Contain  ${output}  Upgrading VCH exceeded time limit
    Should Not Contain  ${output}  Completed successfully
    # we should have no snapshots
    ${rc}  ${output}=  Run And Return Rc And Output  govc snapshot.tree -vm=%{VCH-NAME}
    Should Be Empty  ${output}
    # the appliance is restarting - attempt to wait until it's ready
    # version of appliance we rolled back to is old, so we have to set DOCKER_API_VERSION
    Set Environment Variable  DOCKER_API_VERSION  1.23
    # keyword will call docker info until response or timeout
    Wait For VCH Initialization  30x
    # confirm that the rollback took effect
    Check Original Version
    Remove Environment Variable  DOCKER_API_VERSION

Upgrade VCH
    Create Docker Containers

    Create Container with Named Volume

    # Create check list for Volume Inspect
    @{checkList}=  Create List  ${mntTest}  ${mntNamed}  ${namedVolume}

    Upgrade
    Check Upgraded Version
    Check Container Create Timestamps

    Verify Volume Inspect Info  After Upgrade and Before Rollback  ${TestContainerVolume}  ${checkList}

    Rollback
    Check Original Version

    Upgrade with ID
    Check Upgraded Version

    Verify Volume Inspect Info  After Upgrade with ID  ${TestContainerVolume}  ${checkList}

    Run Docker Checks


    Log To Console  Regression Tests...
    Run Regression Tests
