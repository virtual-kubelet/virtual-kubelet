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
Documentation  Test 1-30 - Docker Deploy
Resource  ../../resources/Util.robot
Suite Setup  Conditional Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Docker deploy
    ${rc}  ${output}=  Run And Return Rc And Output  docker1.13 %{VCH-PARAMS} deploy %{GOPATH}/src/github.com/vmware/vic/demos/compose/voting-app/votingapp.dab
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  only supported with experimental daemon

