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
Documentation  Test 3-02 - Docker Compose Voting App
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  certs=${True}
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Compose Voting App
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login --username=victest --password=%{REGISTRY_PASSWORD}
    Should Be Equal As Integers  ${rc}  0

    Set Environment Variable  COMPOSE_HTTP_TIMEOUT  300

    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --skip-hostname-check -f ${CURDIR}/../../../demos/compose/voting-app/docker-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f {{.State.Running}} vote
    Log  ${out}
    Should Contain  ${out}  true
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f {{.State.Running}} result
    Log  ${out}
    Should Contain  ${out}  true
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f {{.State.Running}} worker
    Log  ${out}
    Should Contain  ${out}  true
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f {{.State.Running}} db
    Log  ${out}
    Should Contain  ${out}  true
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f {{.State.Running}} redis
    Log  ${out}
    Should Contain  ${out}  true
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range $key, $value := .NetworkSettings.Networks}}{{$key}}{{end}}' vote
    Log  ${out}
    Should Not Be Empty  ${out}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range $key, $value := .NetworkSettings.Networks}}{{index $value "Aliases"}}{{end}}' vote
    Log  ${out}
    Should Contain  ${out}  vote
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} inspect -f '{{range $key, $value := .NetworkSettings.Networks}}{{index $value "IPAddress"}}{{end}}' vote
    Log  ${out}
    Should Not Be Empty  ${out}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  wget %{VCH-IP}:5000
    Should Be Equal As Integers  ${rc}  0
