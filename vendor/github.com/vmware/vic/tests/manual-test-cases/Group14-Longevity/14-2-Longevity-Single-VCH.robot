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
Documentation  Test 14-2 - Longevity - Single - VCH
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If Test Failed  Longevity cleanup

*** Keywords ***
Longevity cleanup
    Run Keyword And Continue On Failure  Post Message To Slack Channel  general  Longevity has failed on %{GOVC_URL}
    Run Keyword And Continue On Failure  Run  govc logs.download

*** Test Cases ***
Longevity - Single - VCH
    # Just install with certs, as it is our most common expected install path
    Install VIC Appliance To Test Server  debug=0  certs=${true}  additional-args=%{STATIC_VCH_OPTIONS}
    # Each regression test takes about 1-2 minutes, so round down and call it a minute
    # 2880 is the number of minutes in 2 days
    :FOR  ${idx}  IN RANGE  0  2880
    \   Log To Console  \nLoop: ${idx}
    \   Run Regression Tests
    
    Cleanup VIC Appliance On Test Server
    
    Install VIC Appliance To Test Server  debug=0  certs=${true}  additional-args=%{STATIC_VCH_OPTIONS}
    Run Regression Tests
    Cleanup VIC Appliance On Test Server
