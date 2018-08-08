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
Documentation  Test 19-4 - DRS-Disabled-License-Features
Resource  ../../resources/Util.robot
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
vic-machine create checks DRS setting and RP options
    Set Environment Variable  TEST_TIMEOUT  30m

    ${output}=  Install VIC Appliance To Test Server  additional-args=--memory 8000 --memory-reservation 512 --memory-shares 6000 --cpu 10000 --cpu-reservation 512 --cpu-shares high
    Should Contain  ${output}  DRS is recommended, but is disabled
    Should Contain  ${output}  Memory Limit
    Should Contain  ${output}  Memory Reservation
    Should Contain  ${output}  Memory Shares
    Should Contain  ${output}  CPU Limit
    Should Contain  ${output}  CPU Reservation
    Should Contain  ${output}  CPU Shares
