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
Documentation  Test 6-06 - Verify vic-machine create image store, volume store and container store functions
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If Test Failed  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Image Store Delete - Image store not found
    Log To Console  \nRunning vic-machine create - custom image store path
    Set Test Environment Variables
    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    Log To Console  \nInstalling VCH to test server...
    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --user=%{TEST_USERNAME} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --image-store=%{TEST_DATASTORE}/images --password=%{TEST_PASSWORD} --force --kv
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}...

    Log To Console  \nDeleting image stores...
    ${out}=  Run  govc datastore.rm -ds=%{TEST_DATASTORE} images

    Log To Console  \nRunning vic-machine delete
    Cleanup VIC Appliance On Test Server
