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
Documentation  Test 5-20 - Restore Starting State
Resource  ../../resources/Util.robot
#Suite Setup  Install VIC Appliance To Test Server
#Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Restore Container Starting State on Restart
    [Timeout]    110 minutes
    Pass Execution  Not sure why this test case is here, but it needs to be re-implemented to work in Nimbus
	# enable firewall
	Run  govc host.esxcli network firewall set -e true
	${out}=  Run  docker %{VCH-PARAMS} pull busybox
	# with enabled firewall this will timeout
	${rc}  ${container}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -i busybox
	#Should Be Equal As Integers  ${rc}  0
	#${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container}
	Should Contain  ${container}  deadline
	${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
	Should Contain  ${out}  Starting
	# disable firewall
	Run  govc host.esxcli network firewall set -e false
	# restart appliance
	Reboot VM  %{VCH-NAME}
	# wait for docker info to succeed
	Wait Until Keyword Succeeds  20x  5 seconds  Run Docker Info  %{VCH-PARAMS}
	# ensure we have a starting container
	${rc}  ${out}=  Run And Return Rc And Output  docker %{VCH-PARAMS} ps -a
	Should Contain  ${out}  Starting
