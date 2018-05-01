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
Documentation  Test 22-12 - centos
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Latest centos container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run centos yum list
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Loaded plugins: fastestmirror, ovl
    Should Contain  ${output}  yum-plugin-versionlock.noarch

Centos:6 container
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run centos:6 yum list
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Loaded plugins: fastestmirror, ovl
    Should Contain  ${output}  yum-plugin-versionlock.noarch
