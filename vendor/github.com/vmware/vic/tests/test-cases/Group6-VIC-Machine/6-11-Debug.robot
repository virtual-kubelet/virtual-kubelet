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
Documentation  Test 6-11 - Verify enable of ssh in the appliance
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Enable SSH and verify
    # generate a key to use for the Test
    ${rc}=  Run And Return Rc  ssh-keygen -t rsa -N "" -f %{VCH-NAME}.key
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  chmod 600 %{VCH-NAME}.key
    Should Be Equal As Integers  ${rc}  0

    ${rc}=  Run And Return Rc  bin/vic-machine-linux debug --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --enable-ssh --authorized-key=%{VCH-NAME}.key.pub
    Should Be Equal As Integers  ${rc}  0

    # check the ssh
    ${rc}=  Run And Return Rc  ssh -vv -o StrictHostKeyChecking=no -i %{VCH-NAME}.key root@%{VCH-IP} /bin/true
    Should Be Equal As Integers  ${rc}  0

    # delete the keys
    Remove Files  %{VCH-NAME}.key  %{VCH-NAME}.key.pub


Check Password Change When Expired
    # generate a key to use for the Test
    ${rc}=  Run And Return Rc  ssh-keygen -t rsa -N "" -f %{VCH-NAME}.key
    Should Be Equal As Integers  ${rc}  0
    ${rc}=  Run And Return Rc  chmod 600 %{VCH-NAME}.key
    Should Be Equal As Integers  ${rc}  0

    ${rc}=  Run And Return Rc  bin/vic-machine-linux debug --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --enable-ssh --authorized-key=%{VCH-NAME}.key.pub
    Should Be Equal As Integers  ${rc}  0

    # push the date forward, past the suport duration
    ${rc}  ${output}=  Run And Return Rc And Output  ssh -o StrictHostKeyChecking=no -i %{VCH-NAME}.key root@%{VCH-IP} 'date -s " +6 year"'
    Should Be Equal As Integers  ${rc}  0

    # command should fail with expired password
    ${rc}=  Run And Return Rc  ssh -vv -o StrictHostKeyChecking=no -i %{VCH-NAME}.key root@%{VCH-IP} /bin/true
    Should Not Be Equal As Integers  ${rc}  0

    # Set the password to a dictionary word - this should not be rejected via this path
    ${rc}=  Run And Return Rc  bin/vic-machine-linux debug --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME} --enable-ssh --rootpw=dictionary
    Should Be Equal As Integers  ${rc}  0

    # check we can now log in cleanly - log in via password
    ${rc}=  Run And Return Rc  sshpass -p dictionary ssh -o StrictHostKeyChecking=no root@%{VCH-IP} /bin/true
    Should Be Equal As Integers  ${rc}  0

    # delete the keys
    Remove Files  %{VCH-NAME}.key  %{VCH-NAME}.key.pub

Check Error From Incorrect ID
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux debug --target %{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --id=wrong
    Should Be Equal As Integers  ${rc}  1
    Should Contain  ${output}  Failed to get Virtual Container Host 
    Should Contain  ${output}  id \\"wrong\\" could not be found
    
