# Copyright 2017 VMware, Inc. All Rights Reserved.
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
Documentation  Test 3-04 - Compose fixtures
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Command
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/command/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/command/docker-compose.yml down

Container Name
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/container_name/docker-compose.yml up
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${out}  my-web-container exited with code 0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/container_name/docker-compose.yml down

Depends On
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/depends_on/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/depends_on/docker-compose.yml down

Env File
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/env-file/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/env-file/docker-compose.yml down

Environment
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/environment-composefile/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/environment-composefile/docker-compose.yml down

Extends
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/extends/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/extends/docker-compose.yml down

Group Add
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/group_add/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/group_add/docker-compose.yml down

Labels
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/labels/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/labels/docker-compose.yml down

Links
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/links-composefile/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/links-composefile/docker-compose.yml down

Networks
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/docker-compose.yml down

Networks- Default
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/default-network-config.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/default-network-config.yml down

Networks- External Default
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{COMPOSE-PARAMS} network create composetest_external_network
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/external-default.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/external-default.yml down

Networks-Bridge
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/bridge.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/bridge.yml down

Networks-External
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{COMPOSE-PARAMS} network create networks_foo
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{COMPOSE-PARAMS} network create networks_bar
    Should Be Equal As Integers  ${rc}  0    

    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/external-networks.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/external-networks.yml down

Networks-Label
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/network-label.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/network-label.yml down

Networks-mode
    ${status}=  Get State Of Github Issue  4541
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-1-Docker-Info.robot needs to be updated now that Issue #4541 has been resolved
    
    #${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/network-mode.yml up -d
    #Log  ${out}
    #Should Be Equal As Integers  ${rc}  0
    #${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/network-mode.yml down

Networks-static-address
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/network-static-addresses.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/networks/network-static-addresses.yml down

Ports
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/ports-composefile/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/ports-composefile/docker-compose.yml down

Stop Signal
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/stop-signal-composefile/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/stop-signal-composefile/docker-compose.yml down

Volumes
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/volumes/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/volumes/docker-compose.yml down

Volumes - External volumes
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{COMPOSE-PARAMS} volume create --name foo
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{COMPOSE-PARAMS} volume create --name bar
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{COMPOSE-PARAMS} volume create --name some_bar
    Should Be Equal As Integers  ${rc}  0
    
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/volumes/external-volumes.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/volumes/external-volumes.yml down

Volumes - Label
    ${status}=  Get State Of Github Issue  4540
    Run Keyword If  '${status}' == 'closed'  Fail  Test 1-1-Docker-Info.robot needs to be updated now that Issue #4540 has been resolved

    #${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/volumes/volume-label.yml up -d
    #Log  ${out}
    #Should Be Equal As Integers  ${rc}  0
    #${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f %{GOPATH}/src/github.com/vmware/vic/tests/resources/dockerfiles/configs/volumes/volume-label.yml down


