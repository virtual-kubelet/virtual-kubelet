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
Documentation  Test 5343
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server



*** Test Cases ***
Check vsphere event stream
    # basic confirmation of function
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${name}=  Generate Random String  15
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${name} ${busybox} sleep 600
    Should Be Equal As Integers  ${rc}  0
    ${shortid}=  Get container shortID  ${id}

    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} start ${name}
    Should Be Equal As Integers  ${rc}  0

    # ensure that portlayer log contains the powered on event - this string comes in the e.Message portion of the vSphere event
    # and may be prone to Localization which would cause this test to fail.
    # for efficiency We assume that if we saw powered on then "${id} Created" would also have matched
    Portlayer Log Should Match Regexp  ${name}-${shortid} on \\s\\S* in \\S* is powered on

    # delete the session to suppress reception of events
    ${rc}  ${out}=  Run And Return Rc And Output  govc session.ls
    Log  ${out}
    ${matches}=  Get Lines Matching Regexp  ${out}  %{VCH-IP}\\s* vic-engine  partial_match=True
    @{sessions}=  Split to lines  ${matches}
    :FOR  ${session}  IN  @{sessions}
    \  ${key}=  Fetch From Left  ${session}  ${SPACE}
    \  ${rc}=  Run And Return Rc  govc session.rm ${key}
    \  Should Be Equal As Integers  ${rc}  0

    # power off the VM directly
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.power --off ${name}-${shortid}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Confirm container reported as stopped by VCH
    Wait Until Container Stops  ${id}

    # Assert that the power off event is present
    # Would prefer to do this as a tail on the live log but no idea how to do stream processing in robot
    Wait Until Keyword Succeeds  1m  10s  Portlayer Log Should Match Regexp  ${name}-${shortid} on \\s*\\S* in \\S* is powered off
