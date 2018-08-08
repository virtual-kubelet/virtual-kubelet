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
Documentation  Test 3-03 - Docker Compose Basic
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  certs=${true}
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Variables ***
${yml}  version: "2"\nservices:\n${SPACE}web:\n${SPACE}${SPACE}image: python:2.7\n${SPACE}${SPACE}ports:\n${SPACE}${SPACE}- "5000:5000"\n${SPACE}${SPACE}depends_on:\n${SPACE}${SPACE}- redis\n${SPACE}redis:\n${SPACE}${SPACE}image: redis\n${SPACE}${SPACE}ports:\n${SPACE}${SPACE}- "5001:5001"
${link-yml}  version: "2"\nservices:\n${SPACE}redis1:\n${SPACE}${SPACE}image: redis:alpine\n${SPACE}${SPACE}container_name: redis1\n${SPACE}${SPACE}ports: ["6379"]\n${SPACE}web1:\n${SPACE}${SPACE}image: busybox\n${SPACE}${SPACE}container_name: a.b.c\n${SPACE}${SPACE}links:\n${SPACE}${SPACE}- redis1:aaa\n${SPACE}${SPACE}command: ["ping", "aaa"]
${rename-yml-1}  version: "2"\nservices:\n${SPACE}web:\n${SPACE}${SPACE}image: busybox\n${SPACE}${SPACE}command: ["/bin/top"]
${rename-yml-2}  version: "2"\nservices:\n${SPACE}web:\n${SPACE}${SPACE}image: ubuntu\n${SPACE}${SPACE}command: ["date"]
${hello-yml}  version: "2"\nservices:\n${SPACE}top:\n${SPACE}${SPACE}image: busybox\n${SPACE}${SPACE}container_name: top\n${SPACE}${SPACE}command: ["echo", "hello, world"]

*** Keywords ***
Check Compose Logs
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file link-compose.yml logs
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  PING aaa
    Should Not Contain  ${output}  bad address 'aaa'

*** Test Cases ***
Compose basic
    Set Environment Variable  COMPOSE_HTTP_TIMEOUT  300

    Run  echo '${yml}' > basic-compose.yml
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network create vic_default
    Log  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml create
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml start
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml logs
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml stop
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compose kill
    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f basic-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f basic-compose.yml kill redis
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f basic-compose.yml down
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0

Compose Up while another container is running (ps filtering related)
    ${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -d busybox /bin/top
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${out}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f basic-compose.yml up -d
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml down
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compose Up with link
    Run  echo '${link-yml}' > link-compose.yml
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file link-compose.yml up -d
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  10s  Check Compose Logs
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file link-compose.yml down
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compose bundle creation
    ${rc}  Run And Return Rc  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml pull
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml bundle
    Log  ${output}
    Should Contain  ${output}  Wrote bundle
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file basic-compose.yml down
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compose up -d --force-recreate
    Run  echo '${rename-yml-1}' > compose-rename.yml
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file compose-rename.yml up -d
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file compose-rename.yml up -d --force-recreate
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compose up -d with a new image
    Run  echo '${rename-yml-2}' > compose-rename.yml
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file compose-rename.yml up -d   
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} --file compose-rename.yml down
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compose up in foreground (attach path)   
    Run  echo '${hello-yml}' > hello-compose.yml
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f hello-compose.yml pull
    Should Be Equal As Integers  ${rc}  0
    Log  ${output}

    # Bring up the compose app and wait till they're up and running
    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f hello-compose.yml up
    Log  ${output}
    Should Contain  ${output}  hello, world

    ${rc}  ${output}=  Run And Return Rc And Output  docker-compose %{COMPOSE-PARAMS} -f hello-compose.yml down
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
