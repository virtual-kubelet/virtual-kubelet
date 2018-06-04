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
Documentation    Test 5-21 - Datastore-Path
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  Setup Suite ESX
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Variables ***
${testDatastoreSpace}=  "datastore (1)"
${dsScheme}=  ds://

*** Keywords ***
Setup Suite ESX
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${esx1}  ${esx1-ip}=  Deploy Nimbus ESXi Server  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Set Suite Variable  ${ESX1}  ${esx1}
    Set Suite Variable  @{list}  ${esx1}

    Log To Console  Deploy VIC Appliance To ESX
    Set Environment Variable  TEST_URL_ARRAY  ${esx1-ip}
    Set Environment Variable  TEST_URL  ${esx1-ip}
    Set Environment Variable  TEST_USERNAME  root
    Set Environment Variable  TEST_PASSWORD  ${NIMBUS_ESX_PASSWORD}
    Set Environment Variable  TEST_DATASTORE  datastore1
    Set Environment Variable  TEST_TIMEOUT  30m
    Set Environment Variable  HOST_TYPE  ESXi
    Remove Environment Variable  TEST_DATACENTER
    Remove Environment Variable  TEST_RESOURCE
    Remove Environment Variable  BRIDGE_NETWORK
    Remove Environment Variable  PUBLIC_NETWORK

    ${output}=  Install VIC Appliance To Test Server  certs=${false}
    Should Contain  ${output}  Installer completed successfully


*** Test Cases ***
Datastore - DS Scheme Specified in Volume Store
    Log To Console  \nStarting DS Scheme in Volume Store Test...

    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    Log To Console  \nRunning custom vic-machine create - with DS Scheme in Volume Store
    # Need to run custom vic-machine create to specify volume store with DS scheme
    ${output}=  Run  bin/vic-machine-linux create --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL_ARRAY} --user=%{TEST_USERNAME} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --image-store=%{TEST_DATASTORE} --volume-store=${dsScheme}%{TEST_DATASTORE}/images:default --password=%{TEST_PASSWORD} --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --insecure-registry harbor.ci.drone.local --force --kv
    Should Contain  ${output}  Installer completed successfully


Datastore - Space in Path
    Log To Console  \nStarting Space in Path Test...

    # Rename original TEST_DATASTORE to a datastore with a space in the path
    ${out}=  Run  govc object.rename /ha-datacenter/datastore/%{TEST_DATASTORE} ${testDatastoreSpace}
    Should Contain  ${out}  OK

    Set Environment Variable  TEST_DATASTORE  ${testDatastoreSpace}

    ${output}=  Install VIC Appliance To Test Server  certs=${false}  vol=default
    Should Contain  ${output}  Installer completed successfully


Datastore - Space in Path with Scheme
    Log To Console  \nStarting Space in Path with Scheme Test...

    # Rename original TEST_DATASTORE to a datastore with a space in the path: this is a double check
    ${out}=  Run  govc object.rename /ha-datacenter/datastore/%{TEST_DATASTORE} ${testDatastoreSpace}
    Should Contain  ${out}  OK

    Set Environment Variable  TEST_DATASTORE  ${testDatastoreSpace}

    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    Log To Console  \nRunning custom vic-machine create - with DS Scheme in Volume Store with space in path
    # Need to run custom vic-machine create to specify volume store with DS scheme
    ${output}=  Run  bin/vic-machine-linux create --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL_ARRAY} --user=%{TEST_USERNAME} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --image-store=%{TEST_DATASTORE} --volume-store=${dsScheme}%{TEST_DATASTORE}/images:default --password=%{TEST_PASSWORD} --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --insecure-registry harbor.ci.drone.local --force --kv
    Should Contain  ${output}  Installer completed successfully
