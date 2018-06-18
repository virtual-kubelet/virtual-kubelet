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
Documentation  Test 10-01 - VCH Restart
Resource  ../../resources/Util.robot
Test Setup  Install VIC Appliance To Test Server
Test Teardown  Cleanup VIC Appliance On Test Server
Default Tags

*** Keywords ***
Get Container IP
    [Arguments]  ${id}  ${network}=default
    ${rc}  ${ip}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect ${network} | jq '.[0].Containers."${id}".IPv4Address'
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${ip}

Launch Container
    [Arguments]  ${name}  ${network}=default  ${command}=sh
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${name} --net ${network} -itd ${busybox} ${command}
    Should Be Equal As Integers  ${rc}  0
    ${id}=  Get Line  ${output}  -1
    [Return]  ${id}

Launch Container With Port Forwarding
    [Arguments]  ${name}  ${port1}  ${port2}  ${network}=default
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p ${port1}:80 -p ${port2}:80 --name ${name} --net ${network} ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${name}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error

Check Nginx Port Forwarding
    [Arguments]  ${port1}  ${port2}
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  ${port1}
    Wait Until Keyword Succeeds  20x  5 seconds  Hit Nginx Endpoint  %{VCH-IP}  ${port2}

*** Test Cases ***
Created Network And Images Persists As Well As Containers Are Discovered With Correct IPs
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${nginx}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create foo
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create bar
    Should Be Equal As Integers  ${rc}  0

    ${bridge-exited}=  Launch Container  vch-restart-bridge-exited  bridge  ls
    ${bridge-running}=  Launch Container  vch-restart-bridge-running  bridge
    ${bridge-running-ip}=  Get Container IP  ${bridge-running}  bridge
    ${bar-exited}=  Launch Container  vch-restart-bar-exited  bar  ls
    ${bar-running}=  Launch Container  vch-restart-bar-running  bar
    ${bar-running-ip}=  Get Container IP  ${bar-running}  bar

    Launch Container  foo-c1  foo
    Launch Container  foo-c2  foo
    Launch Container  bar-c1  bar
    Launch Container  bar-c2  bar
    # name resolution should work on the foo and bar networks
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec foo-c1 ping -c3 foo-c2
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec foo-c2 ping -c3 foo-c1
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec bar-c1 ping -c3 bar-c2
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec bar-c2 ping -c3 bar-c1
    Should Be Equal As Integers  ${rc}  0

    Launch Container With Port Forwarding  webserver  10000  10001
    Check Nginx Port Forwarding  10000  10001

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -before-reboot-1

    Reboot VM  %{VCH-NAME}
    Wait For VCH Initialization  20x  10 seconds

    # name resolution should work on the foo and bar networks
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec foo-c1 ping -c3 foo-c2
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec foo-c2 ping -c3 foo-c1
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec bar-c1 ping -c3 bar-c2
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} exec bar-c2 ping -c3 bar-c1
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} images
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  nginx
    Should Contain  ${output}  busybox

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  foo
    Should Contain  ${output}  bar
    Should Contain  ${output}  bridge
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect bridge
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect bar
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect foo
    Should Be Equal As Integers  ${rc}  0

    ${ip}=  Get Container IP  ${bridge-running}  bridge
    Should Be Equal  ${ip}  ${bridge-running-ip}
    ${ip}=  Get Container IP  ${bar-running}  bar
    Should Be Equal  ${ip}  ${bar-running-ip}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${bridge-running} | jq '.[0].State.Status'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  \"running\"
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${bar-running} | jq '.[0].State.Status'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  \"running\"
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${bridge-exited} | jq '.[0].State.Status'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  \"exited\"
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${bar-exited} | jq '.[0].State.Status'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  \"exited\"
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${bar-exited}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${bridge-exited}
    Should Be Equal As Integers  ${rc}  0

    Check Nginx Port Forwarding  10000  10001

    # one of the ports collides
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create -it -p 10001:80 -p 10002:80 --name webserver1 ${nginx}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start webserver1
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  port 10001 is not available

    # docker pull should work
    # if this fails, very likely the default gateway on the VCH is not set
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${alpine}
    Should Be Equal As Integers  ${rc}  0


Container on Open Network And Port Forwarding Persist After Reboot
    [Setup]     NONE

    Log To Console  Create Port Groups For Container network
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.add -vswitch vSwitchLAN open-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Add VC Distributed Portgroup  test-ds  open-net
    Log  ${out}

    Install VIC Appliance To Test Server  additional-args=--container-network=open-net --container-network-firewall=open-net:open

    # Create a container on the open network
    ${open-running}=  Launch Container  vch-restart-open-running  open-net
    ${open-exited}=  Launch Container  vch-restart-open-exited  open-net  ls

    # Create nginx on the open network and bridge network
    Launch Container With Port Forwarding  webserver-open  10000  10001  open-net
    Launch Container with Port Forwarding  webserver-bridge  10002  10003  bridge
    Check Nginx Port Forwarding  10002  10003

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -open-network

    # Reboot VCH
    Reboot VM  %{VCH-NAME}
    Wait For VCH Initialization  20x  10 seconds

    # Check if the open container is persisted
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${open-running} | jq '.[0].State.Status'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  \"running\"
    # ensure that there isn't a mapping entry for unspecified ports - they are all open but we are not listing them as bindings
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${open-running} | jq '.[0].HostConfig.PortBindings'
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  $[output}  \"0/tcp\":
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect ${open-exited} | jq -r '.[0].State.Status'
    Should Be Equal As Integers  ${rc}  0
    Should Be Equal  ${output}  exited
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${open-exited}
    Should Be Equal As Integers  ${rc}  0

    # Check port forwarding on bridge container after reboot
    Check Nginx Port Forwarding  10002  10003

    # Check port forwarding on open container not reachable from endpoint VM
    ${rc1}  ${output1}=  Run And Return Rc And Output  wget %{VCH-IP}:10000
    ${rc2}  ${output2}=  Run And Return Rc And Output  wget %{VCH-IP}:10001
    Should Not Be Equal As Integers  ${rc1}  0
    Should Not Be Equal As Integers  ${rc2}  0

    Log To Console  Cleanup Port Groups For Container network
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.remove open-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Remove VC Distributed Portgroup  open-net
    Log ${out}

Create VCH attach disk and reboot
    ${rc}=  Run And Return Rc  govc vm.disk.create -vm=%{VCH-NAME} -name=%{VCH-NAME}/deleteme -size "16M"
    Should Be Equal As Integers  ${rc}  0

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -before-reboot-2

    Reboot VM  %{VCH-NAME}

    # wait for docker info to succeed
    Wait Until Keyword Succeeds  20x  5 seconds  Run Docker Info  %{VCH-PARAMS}
    ${rc}=  Run And Return Rc  govc device.ls -vm=%{VCH-NAME} | grep disk
    Should Be Equal As Integers  ${rc}  1

Docker inspect mount and cmd data after reboot
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=named-volume
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name=mount-data-test -v /mnt/test -v named-volume:/mnt/named busybox /bin/ls -la /
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.Mounts}}' mount-data-test
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${out}  /mnt/test
    Should Contain  ${out}  /mnt/named

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.Config.Cmd}}' mount-data-test
    Should Be Equal As Integers  ${rc}  0
    Should Contain X Times  ${out}  /bin/ls  1
    Should Contain X Times  ${out}  -la  1
    Should Contain X Times  ${out}  ${SPACE}/  1

    # Gather logs before rebooting
    Run Keyword And Continue On Failure  Gather Logs From Test Server  -before-reboot-3

    Reboot VM  %{VCH-NAME}

    # wait for docker info to succeed
    Wait Until Keyword Succeeds  20x  5 seconds  Run Docker Info  %{VCH-PARAMS}
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.Mounts}}' mount-data-test
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${out}  /mnt/test
    Should Contain  ${out}  /mnt/named

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{.Config.Cmd}}' mount-data-test
    Should Be Equal As Integers  ${rc}  0
    Should Contain X Times  ${out}  /bin/ls  1
    Should Contain X Times  ${out}  -la  1
    Should Contain X Times  ${out}  ${SPACE}/  1
