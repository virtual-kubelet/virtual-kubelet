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
Documentation  Test 22-07 - elasticsearch
Resource  ../../resources/Util.robot
#Suite Setup  Install VIC Appliance To Test Server
#Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Simple background elasticsearch
    ${status}=  Get State Of Github Issue  3624
    Run Keyword If  '${status}' == 'closed'  Fail  Test 22-07-elasticsearch.robot needs to be updated now that Issue #3624 has been resolved
    #${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name es1 -d elasticsearch
    #Log  ${output}
    #Should Be Equal As Integers  ${rc}  0
    #${ip}=  Get IP Address of Container  es1
