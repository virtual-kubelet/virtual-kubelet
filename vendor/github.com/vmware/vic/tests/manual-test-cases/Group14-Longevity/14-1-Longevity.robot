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
Documentation  Test 14-1 - Longevity
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If Test Failed  Longevity cleanup

*** Keywords ***
Longevity cleanup
    Run Keyword And Continue On Failure  Post Message To Slack Channel  mwilliamson-staff  Longevity has failed on %{GOVC_URL}
    Run Keyword And Continue On Failure  Gather Logs From Test Server
    Run Keyword And Continue On Failure  Run  govc logs.download

*** Test Cases ***
Longevity
    # Each loop should take between 1 and 2 hours
    :FOR  ${idx}  IN RANGE  0  48
    \   ${rand}=  Evaluate  random.randint(10, 50)  modules=random
    \   Log To Console  \nLoop: ${idx}
    \   Install VIC Appliance To Test Server  debug=%{DEBUG_VCH_LEVEL}  additional-args=%{STATIC_VCH_OPTIONS} %{SYSLOG_VCH_OPTION}
    \   Repeat Keyword  ${rand} times  Run Regression Tests
    \   Cleanup VIC Appliance On Test Server
    \   ${rand}=  Evaluate  random.randint(10, 50)  modules=random
    \   Install VIC Appliance To Test Server  debug=%{DEBUG_VCH_LEVEL}  certs=${true}  additional-args=%{STATIC_VCH_OPTIONS} %{SYSLOG_VCH_OPTION}
    \   Repeat Keyword  ${rand} times  Run Regression Tests
    \   Cleanup VIC Appliance On Test Server

    Post Message To Slack Channel  mwilliamson-staff  Longevity has passed on %{GOVC_URL}
