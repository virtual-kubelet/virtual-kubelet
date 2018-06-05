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
Documentation  Test 11-03 - Upgrade-InsecureRegistry
Resource  ../../resources/Util.robot
#Test Teardown  Cleanup Test Environment

*** Variables ***
${test_vic_version}  1.2.1
${vic_success}  Installer completed successfully
${docker_bridge_network}  bridge
${docker_daemon_default_port}  2375
${http_harbor_name}  integration-test-harbor-http
${https_harbor_name}  integration-test-harbor-https
${default_local_docker_endpoint}  unix:///var/run/docker-local.sock

*** Keywords ***
Setup Test Environment
    [arguments]  ${insecure_registry}
    ${handle}  ${docker_daemon_pid}=  Start Docker Daemon Locally  --insecure-registry ${insecure_registry}
    Setup VCH And Registry  ${insecure_registry}
    [Return]  ${handle}  ${docker_daemon_pid}

Run Secret Curl Command
    [Tags]  secret
    [Arguments]  ${registry_ip}  ${protocol}  ${curl_option}  ${user}=admin  ${password}=%{TEST_PASSWORD}
    ${rc}  ${output}=  Run And Return Rc And Output  curl ${curl_option} -u ${user}:${password} -H "Content-Type: application/json" -X POST -d '{"project_name": "test","public": 1}' ${protocol}://${registry_ip}/api/projects
    [Return]  ${rc}  ${output}

Add Project On Registry
    [Arguments]  ${registry_ip}  ${protocol}
    # Harbor API: https://github.com/vmware/harbor/blob/master/docs/swagger.yaml
    Run Keyword If  '${protocol}' == 'https'  Set Test Variable  ${curl_option}  --insecure
    Run Keyword If  '${protocol}' == 'http'  Set Test Variable  ${curl_option}  ${EMPTY}
    :FOR  ${i}  IN RANGE  12
    \   ${rc}  ${output}=  Run Secret Curl Command  ${registry_ip}  ${protocol}  ${curl_option}
    \   Log  ${output}
    \   Return From Keyword If  '${rc}' == '0'
    \   Sleep  10s
    Fail  Failed to add project on registry!

Run Secret Docker Login
    [Tags]  secret
    [Arguments]    ${registry_ip}  ${registry_user}=admin  ${registry_password}=%{TEST_PASSWORD}  ${docker}=DOCKER_API_VERSION=1.23 docker
    ${rc}  ${output}=  Run And Return Rc And Output  ${docker} -H ${default_local_docker_endpoint} login --username ${registry_user} --password ${registry_password} ${registry_ip}
    [Return]  ${rc}  ${output}

Setup VCH And Registry
    [Arguments]  ${registry_ip}  ${docker}=DOCKER_API_VERSION=1.23 docker
    ${rc}  ${output}=  Run And Return Rc And Output  echo "From busybox" | ${docker} -H ${default_local_docker_endpoint} build -t ${registry_ip}/test/busybox -
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Successfully built
    Log To Console  \nbusybox built successfully
    ${rc}  ${output}=  Run Secret Docker Login  ${registry_ip}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Login Succeeded
    Log To Console  \nLogin successfully
    ${rc}=  Run And Return Rc  ${docker} -H ${default_local_docker_endpoint} push ${registry_ip}/test/busybox
    Should Be Equal As Integers  ${rc}  0
    Log To Console  \nbusybox pushed successfully

Test VCH And Registry
    [Arguments]  ${vch_endpoint}  ${registry_ip}  ${docker}=DOCKER_API_VERSION=1.23 docker
    ${rc}  ${output}=  Run And Return Rc And Output  ${docker} -H ${vch_endpoint} pull ${registry_ip}/test/busybox
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Digest:
    Should Contain  ${output}  Status:
    Should Not Contain  ${output}  Error response from daemon

Cleanup Test Environment
    [Arguments]  ${docker}=DOCKER_API_VERSION=1.23 docker
    Clean up VIC Appliance And Local Binary
    ${out}=  Cleanup Harbor  ${harbor_name}
    Log  ${out}
    ${rc}=  Run And Return Rc  ${docker} -H ${default_local_docker_endpoint} rmi ${harbor_ip}/test/busybox
    Should Be Equal As Integers  ${rc}  0
    Kill Local Docker Daemon  ${handle}  ${docker_daemon_pid}

*** Test Cases *** 
Upgrade VCH with Harbor On HTTP
    Pass Execution  Need to use a different insecure registry, because Harbor does not support VIC as insecure and 0.5.0 is too old
    Set Test Environment Variables
    ${out}=  Cleanup Harbor  ${http_harbor_name}
    Log  ${out}
    ${out}=  Cleanup Harbor  ${https_harbor_name}
    Log  ${out}
    Set Test Variable  ${harbor_name}  ${http_harbor_name}
    ${ip}=  Install Harbor To Test Server  ${harbor_name}
    Set Test Variable  ${harbor_ip}  ${ip}
    Add Project On Registry  ${harbor_ip}  http
    ${hdl}  ${pid}=  Setup Test Environment  ${harbor_ip}
    Set Test Variable  ${handle}  ${hdl}
    Set Test Variable  ${docker_daemon_pid}  ${pid}

    Install VIC with version to Test Server  ${test_vic_version}  --insecure-registry ${harbor_ip} --no-tls

    Test VCH And Registry  %{VCH-IP}:%{VCH-PORT}  ${harbor_ip}

    Upgrade
    Check Upgraded Version
    Test VCH And Registry  %{VCH-IP}:%{VCH-PORT}  ${harbor_ip}

Upgrade VCH with Harbor On HTTPS
    Pass Execution  Need to use a different insecure registry, because Harbor does not support VIC as insecure and 0.5.0 is too old
    ${out}=  Cleanup Harbor  ${http_harbor_name}
    Log  ${out}
    ${out}=  Cleanup Harbor  ${https_harbor_name}
    Log  ${out}
    Set Test Variable  ${harbor_name}  ${https_harbor_name}
    ${ip}=  Install Harbor To Test Server  ${harbor_name}  https
    Set Test Variable  ${harbor_ip}  ${ip}
    Add Project On Registry  ${harbor_ip}  https
    ${hdl}  ${pid}=  Setup Test Environment  ${harbor_ip}
    Set Test Variable  ${handle}  ${hdl}
    Set Test Variable  ${docker_daemon_pid}  ${pid}

    ${harbor_cert}=  Fetch Harbor Self Signed Cert  ${harbor_ip}
    Install VIC with version to Test Server  ${test_vic_version}  --insecure-registry ${harbor_ip} --no-tls --registry-ca ${harbor_cert}

    Test VCH And Registry  %{VCH-IP}:%{VCH-PORT}  ${harbor_ip}

    Upgrade
    Check Upgraded Version
    Test VCH And Registry  %{VCH-IP}:%{VCH-PORT}  ${harbor_ip}
