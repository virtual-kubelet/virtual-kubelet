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
Documentation  Test 5-15 - NFS Datastore
Resource  ../../resources/Util.robot
Suite Setup  Wait Until Keyword Succeeds  10x  10m  NFS Datastore Setup
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Keywords ***
NFS Datastore Setup
    [Timeout]    110 minutes
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}  ${false}
    ${esx1}  ${esx2}  ${esx3}  ${vc}  ${esx1-ip}  ${esx2-ip}  ${esx3-ip}  ${vc-ip}=  Create a Simple VC Cluster  datacenter1  cls1
    Set Suite Variable  @{list}  ${esx1}  ${esx2}  ${esx3}  %{NIMBUS_USER}-${vc}

    ${name}  ${ip}=  Deploy Nimbus NFS Datastore  %{NIMBUS_USER}  %{NIMBUS_PASSWORD}
    Append To List  ${list}  ${name}

    ${out}=  Run  govc datastore.create -mode readWrite -type nfs -name nfsDatastore -remote-host ${ip} -remote-path /store /datacenter1/host/cls1
    Should Be Empty  ${out}

    Set Environment Variable  TEST_DATASTORE  nfsDatastore

*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Install VIC Appliance To Test Server

    Run Regression Tests

    Cleanup VIC Appliance On Test Server
