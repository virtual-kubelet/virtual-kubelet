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
Documentation  Test 6-01 - Verify Help
Resource  ../../resources/Util.robot
Test Timeout  20 minutes

*** Test Cases ***
Inspect help basic
    ${ret}=  Run  bin/vic-machine-linux inspect -h
    Should Contain  ${ret}  vic-machine-linux inspect - Inspect VCH

Delete help basic
    ${ret}=  Run  bin/vic-machine-linux delete -h
    Should Contain  ${ret}  vic-machine-linux delete - Delete VCH
