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
Documentation  Test 9-02 - VICAdmin CertAuth
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  certs=${true}
Suite Teardown  Cleanup VIC Appliance On Test Server
Default Tags

*** Keywords ***
Skip Execution If Certs Not Available
    ${status}=  Run Keyword And Return Status  Environment Variable Should Not Be Set  DOCKER_CERT_PATH
    Pass Execution If  ${status}  This test is only applicable if using TLS with certs

Curl
    [Arguments]  ${path}
    ${output}=  Run  curl -sk --cert %{DOCKER_CERT_PATH}/cert.pem --key %{DOCKER_CERT_PATH}/key.pem %{VIC-ADMIN}${path}
    Should Not Be Equal As Strings  ''  ${output}
    [Return]  ${output}

*** Test Cases ***
Display HTML
    Skip Execution If Certs Not Available
    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl  ${EMPTY}
    Should contain  ${output}  <title>VIC: %{VCH-NAME}</title>

Get Portlayer Log
    Skip Execution If Certs Not Available
    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl  /logs/port-layer.log
    Should contain  ${output}  Launching portlayer server

Get VCH-Init Log
    Skip Execution If Certs Not Available
    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl  /logs/init.log
    Should contain  ${output}  reaping child processes

Get Docker Personality Log
    Skip Execution If Certs Not Available
    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl  /logs/docker-personality.log
    Should contain  ${output}  docker personality

Get VICAdmin Log
    Skip Execution If Certs Not Available
    ${output}=  Wait Until Keyword Succeeds  10x  10s  Curl  /logs/vicadmin.log
    Log  ${output}
    Should contain  ${output}  Launching vicadmin pprof server

Fail to Get VICAdmin Log without cert
    Skip Execution If Certs Not Available
    ${output}=  Run  curl -sk %{VIC-ADMIN}/logs/vicadmin.log
    Log  ${output}
    Should Not contain  ${output}  Launching vicadmin pprof server

Fail to Display HTML without cert
    Skip Execution If Certs Not Available
    ${output}=  Run  curl -sk %{VIC-ADMIN}
    Log  ${output}
    Should Not contain  ${output}  <title>VCH %{VCH-NAME}</title>

Fail to get Portlayer Log without cert
    Skip Execution If Certs Not Available
    ${output}=  Run  curl -sk %{VIC-ADMIN}/logs/port-layer.log
    Log  ${output}
    Should Not contain  ${output}  Launching portlayer server

Fail to get Docker Personality Log without cert
    Skip Execution If Certs Not Available
    ${output}=  Run  curl -sk %{VIC-ADMIN}/logs/docker-personality.log
    Log  ${output}
    Should Not contain  ${output}  docker personality

Fail to get VCH init logs without cert
    Skip Execution If Certs Not Available
    ${output}=  Run  curl -sk %{VIC-ADMIN}/logs/init.log
    Log  ${output}
    Should Not contain  ${output}  reaping child processes

Check that VIC logs do not contain sensitive data
    Scrape Logs For The Password
