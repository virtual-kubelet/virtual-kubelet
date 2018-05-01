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
Documentation  Test 11-9-Container-Disk-Stress
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Container Disk Stress
    ${out}=  Run  docker %{VCH-PARAMS} pull busybox

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run ubuntu bash -c "apt-get update; apt-get install bonnie++; bonnie++ -u root;"
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Delete files in random order...done.
    
    Run Regression Tests