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
Documentation  Test 11-1-VIC-Install-Stress
Resource  ../../resources/Util.robot
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
VIC Install Stress
    :FOR  ${idx}  IN RANGE  0  100
    \   Log To Console  \nLoop ${idx+1}
    \   Install VIC Appliance To Test Server  vol=default %{STATIC_VCH_OPTIONS}
    \   Cleanup VIC Appliance On Test Server
    
    Install VIC Appliance To Test Server  vol=default %{STATIC_VCH_OPTIONS}
    Run Regression Tests