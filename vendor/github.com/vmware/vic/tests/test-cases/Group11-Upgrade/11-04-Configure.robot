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
Documentation  Test 11-04 - Configure
Resource  ../../resources/Util.robot
Suite Setup  Install VIC with version to Test Server  v1.3.1
Suite Teardown  Clean up VIC Appliance And Local Binary

*** Test Cases ***
Configure VCH with new vic-machine
    ${ret}=  Run  bin/vic-machine-linux configure --target %{TEST_URL} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --http-proxy http://proxy.vmware.com:3128
    Should Not Contain  ${ret}  Completed successfully
    Should Contain  ${ret}  configure failed
