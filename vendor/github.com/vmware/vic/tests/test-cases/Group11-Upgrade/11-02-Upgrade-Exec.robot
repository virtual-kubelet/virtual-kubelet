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
Documentation  Test 11-02 - Upgrade Exec
Resource  ../../resources/Util.robot
Suite Setup  Install VIC with version to Test Server  8351
Suite Teardown  Clean up VIC Appliance And Local Binary
Default Tags

*** Test Cases ***
Exec Not Allowed On Older Containers
    Launch Container  exec-test  bridge

    Upgrade
    Check Upgraded Version

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -d exec-test ls
    Should Not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  running tasks not supported for this container

    Launch Container  exec-test2  bridge
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} exec -d exec-test2 ls
    Should Be Equal As Integers  ${rc}  0
