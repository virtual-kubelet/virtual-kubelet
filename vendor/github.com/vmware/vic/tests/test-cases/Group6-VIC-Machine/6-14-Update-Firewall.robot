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
Documentation  Test 6-14 - Verify vic-machine update firewall function
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Enable and disable VIC firewall rule
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    
    Pass Execution If  '%{HOST_TYPE}' == 'VC'  This test is not applicable to VC

    # Save firewall state
    ${fwSetState}=  Get Host Firewall Enabled

    Enable Host Firewall
    ${fwstatus}=  Get Host Firewall Enabled
    Should Be True  ${fwstatus}

    ${output}=  Run  bin/vic-machine-linux update firewall --target %{TEST_URL} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --thumbprint=%{TEST_THUMBPRINT} --allow
    Should Contain  ${output}  enabled on host
    Should Contain  ${output}  Firewall changes complete


    ${output}=  Run  govc host.esxcli network firewall ruleset list --ruleset-id=vSPC
    Should Contain  ${output}  true

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --no-tls --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests

    ${output}=  Run  bin/vic-machine-linux update firewall --target %{TEST_URL} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --thumbprint=%{TEST_THUMBPRINT} --deny
    Should Contain  ${output}  disabled on host
    Should Contain  ${output}  Firewall changes complete


    ${output}=  Run  govc host.esxcli network firewall ruleset list --ruleset-id=vSPC
    Should Contain  ${output}  false

    # Restore firewall state
    Run Keyword If  ${fwSetState}  Enable Host Firewall
    Run Keyword Unless  ${fwSetState}  Disable Host Firewall
