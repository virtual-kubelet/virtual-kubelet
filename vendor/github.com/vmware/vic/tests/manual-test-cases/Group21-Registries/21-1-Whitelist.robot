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
Documentation  Test 21-01 - Whitelist
Resource  ../../resources/Util.robot
Resource  ../../resources/Harbor-Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Setup Harbor
Suite Teardown  Nimbus Cleanup  ${list}  ${false}
Test Teardown  Run Keyword If Test Failed  Cleanup VIC Appliance On Test Server

*** Keywords ***
Simple ESXi Setup
    [Timeout]    110 minutes
    ${esx}  ${esx-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  @{list}  ${esx}

    Set Environment Variable  TEST_URL_ARRAY  ${esx-ip}
    Set Environment Variable  TEST_URL  ${esx-ip}
    Set Environment Variable  TEST_USERNAME  root
    Set Environment Variable  TEST_PASSWORD  ${NIMBUS_ESX_PASSWORD}
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  HOST_TYPE  ESXi
    Remove Environment Variable  TEST_DATACENTER
    Remove Environment Variable  TEST_RESOURCE
    Remove Environment Variable  BRIDGE_NETWORK
    Remove Environment Variable  PUBLIC_NETWORK

Setup Harbor
    Simple ESXi Setup

    # Install a Harbor server with HTTPS a Harbor server with HTTP
    Install Harbor To Test Server  protocol=https  name=harbor-https
    Set Environment Variable  HTTPS_HARBOR_IP  %{HARBOR-IP}

    Install Harbor To Test Server  protocol=http  name=harbor-http
    Set Environment Variable  HTTP_HARBOR_IP  %{HARBOR-IP}

    Get HTTPS Harbor Certificate

Get HTTPS Harbor Certificate
    [Arguments]  ${HARBOR_IP}=%{HTTPS_HARBOR_IP}
    # Get the certificates from the HTTPS server
    ${out}=  Run  wget --tries=10 --connect-timeout=10 --auth-no-challenge --no-check-certificate --user admin --password %{TEST_PASSWORD} https://${HARBOR_IP}/api/systeminfo/getcert
    Log  ${out}
    Move File  getcert  ./ca.crt

*** Test Cases ***
Basic Whitelisting
    # Install VCH with registry CA for whitelisted registry
    ${output}=  Install VIC Appliance To Test Server  vol=default --whitelist-registry=%{HTTPS_HARBOR_IP} --registry-ca=./ca.crt
    Should Contain  ${output}  Secure registry %{HTTPS_HARBOR_IP} confirmed
    Should Contain  ${output}  Whitelist registries =
    Get Docker Params  ${output}  true

    # Check docker info for whitelist info
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} info
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Registry Whitelist Mode: enabled
    Should Contain  ${output}  Whitelisted Registries:
    Should Contain  ${output}  Registry: registry.hub.docker.com

    # Try to login and pull from the HTTPS whitelisted registry (should succeed)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTPS_HARBOR_IP}
    Should Contain  ${output}  Succeeded
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HTTPS_HARBOR_IP}/library/photon:1.0
    Should Be Equal As Integers  ${rc}  0

    # Try to login and pull from the HTTPS whitelisted registry with :443 tacked on at the end (should succeed)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTPS_HARBOR_IP}:443
    Should Contain  ${output}  Succeeded
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HTTPS_HARBOR_IP}:443/library/photon:1.0
    Should Be Equal As Integers  ${rc}  0

    # Try to login and pull from docker hub (should fail)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login --username=victest --password=%{TEST_PASSWORD}
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Access denied to unauthorized registry
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull victest/busybox
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Access denied to unauthorized registry

    Cleanup VIC Appliance On Test Server

Check Login to Insecure Registry (http)
    # Install VCH w/o specifying insecure registry
    ${output}=  Install VIC Appliance To Test Server  additional-args=--registry-ca=./ca.crt
    Should Not Contain  ${output}  Insecure registry %{HTTP_HARBOR_IP} confirmed
    Get Docker Params  ${output}  true

    # Try to login and pull from the HTTP insecure registry (should fail)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTP_HARBOR_IP}
    Should Not Contain  ${output}  Succeeded
    Should Be Equal As Integers  ${rc}  1
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HTTP_HARBOR_IP}/library/photon:1.0
    Should Be Equal As Integers  ${rc}  1

    Cleanup VIC Appliance On Test Server

    ${output}=  Install VIC Appliance To Test Server  additional-args=--insecure-registry=%{HTTP_HARBOR_IP} --registry-ca=./ca.crt
    Should Contain  ${output}  Insecure registry %{HTTP_HARBOR_IP} confirmed
    Get Docker Params  ${output}  true

    # Try to login and pull from the HTTP insecure registry (should succeed)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTP_HARBOR_IP}
    Should Contain  ${output}  Succeeded
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HTTP_HARBOR_IP}/library/photon:1.0
    Should Be Equal As Integers  ${rc}  0

    Cleanup VIC Appliance On Test Server

Configure Registry CA
    # Install VCH without registry CA
    ${output}=  Install VIC Appliance To Test Server

    Should Not Contain  ${output}  Secure registry %{HTTPS_HARBOR_IP} confirmed

    # Try to login to the HTTPS registry (should fail)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTPS_HARBOR_IP}
    Should Not Contain  ${output}  Succeeded

    # Add the HTTPS registry CA to cert pool with vic-machine configure
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux configure --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --registry-ca=./ca.crt --thumbprint=%{TEST_THUMBPRINT} --debug=1
    Should Be Equal As Integers  ${rc}  0

    # Try to login and pull from the HTTPS registry (should succeed)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTPS_HARBOR_IP}
    Should Contain  ${output}  Succeeded
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HTTPS_HARBOR_IP}/library/photon:1.0
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux configure --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --thumbprint=%{TEST_THUMBPRINT} --debug=1
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Try to login and pull from the HTTPS registry (should succeed)
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u admin -p %{TEST_PASSWORD} %{HTTPS_HARBOR_IP}
    Should Contain  ${output}  Succeeded
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HTTPS_HARBOR_IP}/library/photon:1.0
    Should Be Equal As Integers  ${rc}  0

    Cleanup VIC Appliance On Test Server
