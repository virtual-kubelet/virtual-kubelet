# Copyright 2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 5-26 - Static IP Address
Resource  ../../resources/Util.robot
Suite Setup  Setup VC With Static IP
Suite Teardown  Run Keyword And Ignore Error  Nimbus Cleanup  ${list}


*** Keywords ***
Setup VC With Static IP
    ${name}=  Evaluate  'vic-5-26-' + str(random.randint(1000,9999))  modules=random
    Wait Until Keyword Succeeds  10x  10m  Create Simple VC Cluster With Static IP  ${name}
    
*** Test Cases ***
Test
    Log To Console  \nStarting test...
    Custom Testbed Keepalive  /dbc/pa-dbc1111/mhagen

    Install VIC Appliance To Test Server  additional-args=--public-network-ip &{static}[ip]/&{static}[netmask] --public-network-gateway &{static}[gateway] --dns-server 10.170.16.48
    Run Regression Tests