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
Documentation  Test 8-02 OOB VM Register
Resource  ../../resources/Util.robot
Suite Teardown  Extra Cleanup

*** Keywords ***
Extra Cleanup
    ${out}=  Run Keyword And Ignore Error  Run  govc vm.destroy ${old-vm}
    ${out}=  Run Keyword And Ignore Error  Run  govc pool.destroy host/*/Resources/${old-vm}
    ${out}=  Run Keyword And Ignore Error  Run  govc datastore.rm ${old-vm}
    ${out}=  Run Keyword And Ignore Error  Run  govc host.portgroup.remove ${old-vm}-bridge
    Cleanup VIC Appliance On Test Server

*** Test Cases ***
Verify VIC Still Works When Different VM Is Registered
    Install VIC Appliance To Test Server
    Set Suite Variable  ${old-vm}  %{VCH-NAME}
    Install VIC Appliance To Test Server

    ${out}=  Run  govc vm.power -off ${old-vm}
    Should Contain  ${out}  OK
    ${out}=  Run  govc vm.unregister ${old-vm}
    Should Be Empty  ${out}
    ${out}=  Run  govc vm.register ${old-vm}/${old-vm}.vmx
    Should Be Empty  ${out}

    ${out}=  Run  docker %{VCH-PARAMS} ps -a
    Log  ${out}
    Should Contain  ${out}  CONTAINER ID
    Should Contain  ${out}  IMAGE
    Should Contain  ${out}  COMMAND

    Run Regression Tests

    ${out}=  Run  govc vm.destroy ${old-vm}
