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
Documentation  Test 22-08 - node
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Variables ***
${package}  '{"name": "docker_web_app","version": "1.0.0","description": "Node.js on Docker","author": "VMware VIC <vic@vmware.com>","main": "server.js","scripts": {"start": "node server.js"},"dependencies": {"express": "^4.13.3"}}'
${server}  "'use strict';const express = require('express');const app = express();app.get('/', (req, res) => {res.send('Hello world');});app.listen('8080', '0.0.0.0');console.log('Running on http://0.0.0.0:8080');"

*** Keywords ***
Check node container
    [Arguments]  ${ip}
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ${busybox} wget -O- ${ip}:8080
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Hello world

*** Test Cases ***
Simple background node application
    Create Directory  app
    Run  echo ${package} > app/package.json
    Run  echo ${server} > app/server.js
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=vol1
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name copier -v vol1:/mydata ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp app copier:/mydata
    Should Be Equal As Integers  ${rc}  0
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name node1 -v vol1:/usr/src -d node sh -c "cd /usr/src/app && npm install && npm start"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get IP Address of Container  node1
    
    Wait Until Keyword Succeeds  10x  12s  Check node container  ${ip}
    
    [Teardown]  Remove Directory  app  recursive=${true}

Simple background node application on alpine
    Create Directory  app
    Run  echo ${package} > app/package.json
    Run  echo ${server} > app/server.js
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create --name=vol2
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name copier2 -v vol2:/mydata ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} cp app copier2:/mydata
    Should Be Equal As Integers  ${rc}  0
    
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name node2 -v vol2:/usr/src -d node:alpine sh -c "cd /usr/src/app && npm install && npm start"
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${ip}=  Get IP Address of Container  node2
    
    Wait Until Keyword Succeeds  10x  12s  Check node container  ${ip}
    
    [Teardown]  Remove Directory  app  recursive=${true}