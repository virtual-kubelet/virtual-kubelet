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
Documentation  Test 4-01 - Docker Integration
Resource  ../../resources/Util.robot
#Suite Setup  Install VIC Appliance To Test Server
#Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases *** 
Docker Integration Tests
    [Tags]  docker
    Log To Console  \nStarting Docker integration tests... NOT EXECUTING NOW.
    # Currently blocked on issue https://github.com/vmware/vic/issues/2549
    #Set Environment Variable  GOPATH  /go:/go/src/github.com/docker/docker/vendor
    #${ip}=  Remove String  %{VCH-PARAMS}  -H
    #${ip}=  Strip String  ${ip}
    #Run  go get github.com/docker/docker
    #${status}  ${out}=  Run Keyword And Ignore Error  Run Process  DOCKER_HOST\=tcp://${ip} go test  shell=True  cwd=/go/src/github.com/docker/docker/integration-cli  timeout=10min  on_timeout=kill
    #Log  ${out.stdout}
    #Log  ${out.stderr}
    #Should Contain  ${out.stdout}  DockerSuite.Test