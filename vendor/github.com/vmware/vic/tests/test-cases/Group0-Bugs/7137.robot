# Copyright 2018 VMware, Inc. All Rights Reserved.
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
Documentation  Test 7137
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server


*** Test Cases ***
Check for die events when forcing update via state refresh
    # basic confirmation of function
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${since}=  Run And Return Rc And Output  docker %{VCH-PARAMS} info --format '{{json .SystemTime}}'
    Should Be Equal As Integers  ${rc}  0

    ${name}=  Generate Random String  15
    ${rc}  ${id}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${name} -d ${busybox} sleep 3
    Should Be Equal As Integers  ${rc}  0

    # tight loop on inspect - this will force an inline state refresh
    Run  end=$(($(date +%s) + 24));while [ $(date +%s) -lt $end ]; do docker %{VCH-PARAMS} inspect ${id} >/dev/null; done

    ${rc}  ${until}=  Run And Return Rc And Output  docker %{VCH-PARAMS} info --format '{{json .SystemTime}}'
    Should Be Equal As Integers  ${rc}  0

    # check for die event - need to supply --until=current-server-timestamp so command returns immediately
    ${rc}  ${events}=  Run And Return Rc And Output  docker %{VCH-PARAMS} events --since=${since} --filter container=${name} --format='Status={{.Status}}' --until=${until}
    Should Be Equal As Integers  ${rc}  0

    Log  ${events}
    Should Contain  ${events}  start
    Should Contain  ${events}  die
    Should Not Contain  ${events}  stop

    # network endpoints should have been unbound regardless of which state update was hit (inline triggered by inspect or event based from vsphere)
    ${rc}  ${bridge}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network inspect bridge
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${bridge}  ${id}


