# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 5-28 - VICAdmin Isolated
Resource  ../../resources/Util.robot
Suite Setup  Deploy Testbed With Static IP
Suite Teardown  Teardown VCH With No WAN
Default Tags

*** Keywords ***
Deploy Testbed With Static IP
    Setup VC With No WAN
    Deploy VCH With No WAN

Setup VC With No WAN
    ${name}=  Evaluate  'vic-5-28-' + str(random.randint(1000,9999))  modules=random
    Nimbus Suite Setup  Create Simple VC Cluster With Static IP  ${name}
    Set Test Environment Variables
    
    Log To Console  Create a vch with a public network on a no-wan portgroup.

    ${vlan}=  Evaluate  str(random.randint(1, 195))  modules=random

    ${dvs}=  Run  govc find -type DistributedVirtualSwitch | head -n1
    ${rc}  ${output}=  Run And Return Rc And Output  govc dvs.portgroup.add -vlan=${vlan} -dvs ${dvs} dpg-no-wan
    Should Be Equal As Integers  ${rc}  0
    
Deploy VCH With No WAN
    ${out}=  Deploy VCH With No WAN Secret
    Log  ${out}
    Get Docker Params  ${out}  ${false}

Deploy VCH With No WAN Secret
    [Tags]  secret
    ${output}=  Run  bin/vic-machine-linux create --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --no-tlsverify --bridge-network=%{BRIDGE_NETWORK} --management-network=%{PUBLIC_NETWORK} --client-network=%{PUBLIC_NETWORK} --client-network-ip &{static}[ip]/&{static}[netmask] --client-network-gateway 10.0.0.0/8:&{static}[gateway] --public-network dpg-no-wan --public-network-ip 192.168.100.2/24 --public-network-gateway 192.168.100.1 --dns-server 10.170.16.48 --insecure-registry wdc-harbor-ci.eng.vmware.com
    [Return]  ${output}

Teardown VCH With No WAN
    Run Keyword And Ignore Error  Nimbus Cleanup  ${list}

*** Test Cases ***
Display HTML
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN} -b ${cookies}
    Log  ${output}
    Should contain  ${output}  <title>VIC: %{VCH-NAME}</title>

WAN Status Should Fail
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN} -b ${cookies}
    Log  ${output}
    Should contain  ${output}  <div class="sixty">Registry and Internet Connectivity<span class="error-message">

Fail To Pull Docker Image
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc and Output  docker %{VCH-PARAMS} pull ${busybox}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  1
    Should contain  ${output}  no route to host

Get Portlayer Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/port-layer.log -b ${cookies}
    Log  ${output}
    Should contain  ${output}  Launching portlayer server

Get VCH-Init Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/init.log -b ${cookies}
    Log  ${output}
    Should contain  ${output}  reaping child processes

Get Docker Personality Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/docker-personality.log -b ${cookies}
    Log  ${output}
    Should contain  ${output}  docker personality

Get VICAdmin Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/vicadmin.log -b ${cookies}
    Log  ${output}
    Should contain  ${output}  Launching vicadmin pprof server
