# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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
Documentation  This resource contains all keywords related to creating, deleting, maintaining VCHs

*** Keywords ***
Set Test Environment Variables
    # Finish setting up environment variables
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  DRONE_BUILD_NUMBER
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  DRONE_BUILD_NUMBER  0
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  PUBLIC_NETWORK
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  PUBLIC_NETWORK  'VM Network'
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  TEST_DATACENTER
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  TEST_DATACENTER  ${SPACE}
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  DRONE_MACHINE
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  DRONE_MACHINE  'local'
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  OVERFLOW_TEST_TIMEOUT
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  OVERFLOW_TEST_TIMEOUT  3m

    @{URLs}=  Split String  %{TEST_URL_ARRAY}
    ${len}=  Get Length  ${URLs}
    ${IDX}=  Evaluate  %{DRONE_BUILD_NUMBER} \% ${len}

    Set Environment Variable  TEST_URL  @{URLs}[${IDX}]
    Set Environment Variable  GOVC_URL  %{TEST_URL}
    Set Environment Variable  GOVC_USERNAME  %{TEST_USERNAME}
    Set Environment Variable  GOVC_PASSWORD  %{TEST_PASSWORD}

    ${IN_HAAS}=  Run Keyword And Return Status  Should Contain  %{HAAS_URL_ARRAY}  %{TEST_URL}
    ${IN_OVERFLOW}=  Run Keyword And Return Status  Should Contain  %{OVERFLOW_URL_ARRAY}  %{TEST_URL}
    Run Keyword If  ${IN_HAAS}  Log To Console  Test Server is in HaaS
    Run Keyword If  ${IN_OVERFLOW}  Log To Console  Test Server is in Nimbus
    Run Keyword If  ${IN_OVERFLOW}  Set Environment Variable  TEST_TIMEOUT  %{OVERFLOW_TEST_TIMEOUT}

    ${rc}  ${thumbprint}=  Run And Return Rc And Output  govc about.cert -k -json | jq -r .ThumbprintSHA1
    Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  TEST_THUMBPRINT  ${thumbprint}
    Log To Console  \nTEST_URL=%{TEST_URL}
    Log To Console  \nDRONE_MACHINE=%{DRONE_MACHINE}
    ${worker_date}=  Run  date
    Log To Console  \nWorker_Date=${worker_date}

    ${rc}  ${host}=  Run And Return Rc And Output  govc ls host
    Should Be Equal As Integers  ${rc}  0
    ${out}=  Run  govc ls -t HostSystem ${host} | xargs -I% -n1 govc host.date.info -host\=% | grep 'date and time'
    Log To Console  \nTest_Server_Dates=\n${out}\n

    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  TEST_RESOURCE
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  TEST_RESOURCE  ${host}/Resources
    Set Environment Variable  GOVC_RESOURCE_POOL  %{TEST_RESOURCE}
    ${noQuotes}=  Strip String  %{TEST_DATASTORE}  characters="
    #"
    Set Environment Variable  GOVC_DATASTORE  ${noQuotes}

    ${about}=  Run  govc about
    ${status}=  Run Keyword And Return Status  Should Contain  ${about}  VMware ESXi
    Run Keyword If  ${status}  Set Environment Variable  HOST_TYPE  ESXi
    Run Keyword Unless  ${status}  Set Environment Variable  HOST_TYPE  VC

    ${about}=  Run  govc datastore.info %{TEST_DATASTORE} | grep 'Type'
    ${status}=  Run Keyword And Return Status  Should Contain  ${about}  vsan
    Run Keyword If  ${status}  Set Environment Variable  DATASTORE_TYPE  VSAN
    Run Keyword Unless  ${status}  Set Environment Variable  DATASTORE_TYPE  Non_VSAN

    # set the TLS config options suitable for vic-machine in this env
    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  $domain == ''  Set Suite Variable  ${vicmachinetls}  --no-tlsverify
    Run Keyword If  $domain != ''  Set Suite Variable  ${vicmachinetls}  --tls-cname=*.${domain}

    Set Test VCH Name
    # cleanup any potential old certs directories
    Remove Directory  %{VCH-NAME}  recursive=${true}

    # create a new, unique bridge if there isn't one already
    ${status}=  Run Keyword And Return Status  Environment Variable Should Not Be Set  BRIDGE_NETWORK
    Run Keyword If  ${status}  Wait Until Keyword Succeeds  5x  1s  Create Unique Bridge Network

Create Unique Bridge Network
    # Ensure unique bridges are non-overlapping in a shared build environment (our CI)
    @{URLs}=  Split String  %{TEST_URL_ARRAY}
    ${idx}=  Get Index From List  ${URLs}  %{TEST_URL}
    ${lowerVLAN}=  Evaluate  (${idx}+2) * 100
    ${upperVLAN}=  Evaluate  ${lowerVLAN}+100

    # Set a unique bridge network for each VCH that has a random VLAN ID
    ${vlan}=  Evaluate  str(random.randint(${lowerVLAN}, ${upperVLAN}))  modules=random
    ${vswitch}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.vswitch.info -json | jq -r ".Vswitch[0].Name"
    ${rc}  ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run And Return Rc And Output  govc host.portgroup.add -vlan=${vlan} -vswitch ${vswitch} VCH-%{DRONE_BUILD_NUMBER}-${vlan}
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Be Equal As Integers  ${rc}  0

    ${dvs}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  govc find -type DistributedVirtualSwitch | head -n1
    ${rc}  ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run And Return Rc And Output  govc dvs.portgroup.add -vlan=${vlan} -dvs ${dvs} VCH-%{DRONE_BUILD_NUMBER}-${vlan}
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Be Equal As Integers  ${rc}  0
    Set Environment Variable  BRIDGE_NETWORK  VCH-%{DRONE_BUILD_NUMBER}-${vlan}

Set Test VCH Name
    ${name}=  Evaluate  'VCH-%{DRONE_BUILD_NUMBER}-' + str(random.randint(1000,9999))  modules=random
    Set Environment Variable  VCH-NAME  ${name}
    Log  Set VCH-NAME as ${name}

Set List Of Env Variables
    [Arguments]  ${vars}
    @{vars}=  Split String  ${vars}
    :FOR  ${var}  IN  @{vars}
    \   ${varname}  ${varval}=  Split String  ${var}  =
    \   Set Environment Variable  ${varname}  ${varval}

Parse Environment Variables
    [Arguments]  ${line}
    # If using the default logrus format
    ${status}=  Run Keyword And Return Status  Should Match Regexp  ${line}  msg\="([^"]*)"
    ${match}  ${vars}=  Run Keyword If  ${status}  Should Match Regexp  ${line}  msg\="([^"]*)"
    Run Keyword If  ${status}  Set List Of Env Variables  ${vars}
    Return From Keyword If  ${status}

    #  If using the old logging format
    ${status}=  Run Keyword And Return Status  Should Contain  ${line}  mINFO
    ${logdeco}  ${vars}=  Run Keyword If  ${status}  Split String  ${line}  ${SPACE}  1
    Run Keyword If  ${status}  Set List Of Env Variables  ${vars}
    Return From Keyword If  ${status}

    # Split the log log into pieces, discarding the initial log decoration, and assign to env vars
    ${logmon}  ${logday}  ${logyear}  ${logtime}  ${loglevel}  ${vars}=  Split String  ${line}  max_split=5
    Set List Of Env Variables  ${vars}

Get Docker Params
    # Get VCH docker params e.g. "-H 192.168.218.181:2376 --tls"
    [Arguments]  ${output}  ${certs}
    @{output}=  Split To Lines  ${output}
    :FOR  ${item}  IN  @{output}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  DOCKER_HOST=
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${line}  ${item}

    # Ensure we start from a clean slate with docker env vars
    Remove Environment Variable  DOCKER_HOST  DOCKER_TLS_VERIFY  DOCKER_CERT_PATH  CURL_CA_BUNDLE  COMPOSE_PARAMS  COMPOSE_TLS_VERSION

    Parse Environment Variables  ${line}

    ${dockerHost}=  Get Environment Variable  DOCKER_HOST

    @{hostParts}=  Split String  ${dockerHost}  :
    ${ip}=  Strip String  @{hostParts}[0]
    ${port}=  Strip String  @{hostParts}[1]
    Set Environment Variable  VCH-IP  ${ip}
    Log  Set VCH-IP as ${ip}
    Set Environment Variable  VCH-PORT  ${port}
    Log  Set VCH-PORT as ${port}

    :FOR  ${index}  ${item}  IN ENUMERATE  @{output}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  http
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${line}  ${item}
    \   ${status}  ${message}=  Run Keyword And Ignore Error  Should Contain  ${item}  Published ports can be reached at
    \   ${idx} =  Evaluate  ${index} + 1
    \   Run Keyword If  '${status}' == 'PASS'  Set Suite Variable  ${ext-ip}  @{output}[${idx}]


    ${status}=             Run Keyword And Return Status  Should Match Regexp  ${ext-ip}  msg\=([^"]*)
    ${ignore}  ${ext-ip}=  Run Keyword If      ${status}  Should Match Regexp  ${ext-ip}  msg\=([^"]*)
                           ...  ELSE                      Split String From Right  ${ext-ip}  ${SPACE}  1
    ${ext-ip}=  Strip String  ${ext-ip}
    Set Environment Variable  EXT-IP  ${ext-ip}
    Log  Set EXT-IP as ${ext-ip}


    ${status}=                Run Keyword And Return Status  Should Match Regexp  ${line}  msg\="([^"]*)"
    ${ignore}  ${vic-admin}=  Run Keyword If      ${status}  Should Match Regexp  ${line}  msg\="([^"]*)"
                              ...  ELSE                      Split String From Right  ${line}  ${SPACE}  1
    Set Environment Variable  VIC-ADMIN  ${vic-admin}

    Run Keyword If  ${port} == 2376  Set Environment Variable  VCH-PARAMS  -H ${dockerHost} --tls
    Run Keyword If  ${port} == 2375  Set Environment Variable  VCH-PARAMS  -H ${dockerHost}

    ### Add environment variables for Compose and TLS

    # Check if tls is enable from vic-machine's output and not trust ${certs} which some tests bypasses
    ${tls_enabled}=  Get Environment Variable  DOCKER_TLS_VERIFY  ${false}

    ### Compose case for no-tlsverify

    # Set environment variables if certs not used to create the VCH.  This is NOT the recommended
    # approach to running compose.  There will be security warnings in the logs and some compose
    # operations may not work properly because certs == false currently means we install with
    # --no-tlsverify. Add CURL_CA_BUNDLE for a workaround in compose tests.  If we change
    # certs == false to install with --no-tls, then we need to change this again.
    Run Keyword If  ${tls_enabled} == ${false}  Set Environment Variable  CURL_CA_BUNDLE  ${EMPTY}

    # Get around quirk in compose if no-tlsverify, then CURL_CA_BUNDLE must exist and compose called with --tls
    Run Keyword If  ${tls_enabled} == ${false}  Set Environment Variable  COMPOSE-PARAMS  -H ${dockerHost} --tls

    ### Compose case for tlsverify (assumes DOCKER_TLS_VERIFY also set)

    Run Keyword If  ${tls_enabled} == ${true}  Set Environment Variable  COMPOSE_TLS_VERSION  TLSv1_2
    Run Keyword If  ${tls_enabled} == ${true}  Set Environment Variable  COMPOSE-PARAMS  -H ${dockerHost}

Convert List to String
    [Arguments]  @{list}
    Should Not Be Empty  ${list}
    ${list-string}=  Set Variable  ${EMPTY}
    :FOR  ${item}  IN  @{list}
    \   ${list-was-empty}=  Set Variable If  '${list-string}' == '${EMPTY}'  ${True}  ${false}
    \   ${list-string}=  Run Keyword If  ${list-was-empty}  Set Variable  ${item}
    \   ...  ELSE  Catenate  SEPARATOR=|  ${list-string}  ${item}

    [Return]  ${list-string}

Add VCH to Removal Exception List
    [Arguments]  ${vch}=${EMPTY}
    ${exceptions-string}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    @{exceptions-list}=  Run Keyword If  '${exceptions-string}' == '${EMPTY}'  Create List
    @{exceptions-list}=  Run Keyword Unless  '${exceptions-string}' == '${EMPTY}'  Split String  ${exceptions-string}  separator=|

    ${set}=  Create Dictionary
    Add List To Dictionary  ${set}  ${exceptions-list}
    Set To Dictionary  ${set}  ${vch}  1

    ${exceptions-list}=  Set Variable  ${set.keys()}

    # Append To List  ${exceptions-list}  ${vch}
    ${list-string}=  Convert List To String  @{exceptions-list}
    Set Environment Variable  VM_EXCEPTIONS  ${list-string}
    Log To Console  Saved '${list-string}' to removal exceptions

Remove VCH from Removal Exception List
    [Arguments]  ${vch}=${EMPTY}
    ${exceptions-string}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    Return From Keyword If  '${exceptions-string}' == '${EMPTY}'  No Exceptions Found
    @{exceptions-list}=  Run Keyword Unless  '${exceptions-string}' == '${EMPTY}'  Split String  ${exceptions-string}  separator=|
    ${idx}=  Get Index From List  ${exceptions-list}  ${vch}
    Remove From List  ${exceptions-list}  ${idx}
    ${len}=  Get Length  ${exceptions-list}
    ${list-string}=  Run Keyword If  ${len} != 0  Convert List To String  @{exceptions-list}
    ...  ELSE  Set Variable  ${EMPTY}
    Set Environment Variable  VM_EXCEPTIONS  ${list-string}

Check If VCH Is In Exception
    [Arguments]  ${vch}=${EMPTY}  ${exceptions}=${EMPTY}
    ${exceptions}=  Run Keyword If  '${exceptions}' == '${EMPTY}'  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ...  ELSE  Set Variable  ${exceptions}
    Return From Keyword If  '${exceptions}' == '${EMPTY}'  ${false}
    ${excluded}=  Set Variable  ${false}
    ${exceptions-list}=  Split String  ${exceptions}  separator=|
    : FOR  ${vm-exclude}  IN  @{exceptions-list}
    \    Continue For Loop If  '${vm-exclude}' != '${vch}'
    \    ${excluded}=  Set Variable  ${true}
    \    Exit For Loop

    [Return]  ${excluded}

Dump Docker Debug Data From VCH
    Log To Console  **********
    List Existing Images On VCH
    List Running Containers On VCH
    Log To Console  **********

Use Target VIC Appliance
    # Use a VIC appliance created outside of CI
    [Arguments]  ${target-vch}=${EMPTY}
    Return From Keyword If  '${target-vch}' == '${EMPTY}'  ${False}

    ${debug-vch}=  Get Environment Variable  DEBUG_VCH  ${EMPTY}
    Set Test Environment Variables
    Set Environment Variable  VCH-NAME  ${target-vch}
    Log To Console  Reusing existing vch: ${target-vch}
    Run VIC Machine Inspect Command
    Add VCH to Removal Exception List  vch=${target-vch}
    Run Keyword If  '${debug-vch}' != '${EMPTY}'  Dump Docker Debug Data From VCH

    [Return]  ${True}

Conditional Install VIC Appliance To Test Server
    [Arguments]  ${certs}=${true}  ${init}=${False}
    ${target-vch}=  Get Environment Variable  TARGET_VCH  ${EMPTY}
    ${multi-vch}=  Get Environment Variable  MULTI_VCH  ${EMPTY}

    # If TARGET_VCH was defined, use that VCH for tests and exit
    Run Keyword If  '${target-vch}' != '${EMPTY}'  Use Target VIC Appliance  target-vch=${target-vch}
    Return From Keyword If  '${target-vch}' != '${EMPTY}'  ${True}

    Install VIC Appliance To Test Server  certs=${certs}

    # If MULT_VCH set to 1, then we are in multi VCH mode, otherwise, we are in single VCH mode
    ${single-vch-mode}=  Run Keyword If  '${multi-vch}' == '1'  Set Variable  ${False}
    ...  ELSE  Set Variable  ${True}

    # In single vch mode, save VCH name to TARGET_VCH and add VCH to exception removal list
    Run Keyword If  ${init}  Set Environment Variable  TARGET_VCH  %{VCH-NAME}

Install VIC Appliance To Test Server
    [Arguments]  ${vic-machine}=bin/vic-machine-linux  ${appliance-iso}=bin/appliance.iso  ${bootstrap-iso}=bin/bootstrap.iso  ${certs}=${true}  ${vol}=default  ${cleanup}=${true}  ${debug}=1  ${additional-args}=${EMPTY}
    Set Test Environment Variables
    ${opsuser-args}=  Get Ops User Args
    ${output}=  Install VIC Appliance To Test Server With Current Environment Variables  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${cleanup}  ${debug}  ${opsuser-args}  ${additional-args}
    Log  ${output}
    [Return]  ${output}

Install VIC Appliance To Test Server With Current Environment Variables
    [Arguments]  ${vic-machine}=bin/vic-machine-linux  ${appliance-iso}=bin/appliance.iso  ${bootstrap-iso}=bin/bootstrap.iso  ${certs}=${true}  ${vol}=default  ${cleanup}=${true}  ${debug}=1  ${opsuser-args}=${EMPTY}  ${additional-args}=${EMPTY}
    # disable firewall
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.esxcli network firewall set -e false
    # Attempt to cleanup old/canceled tests
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Networks On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling vSwitches On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Containers On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Folders On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling Resource Pools On Test Server
    Run Keyword If  ${cleanup}  Run Keyword And Ignore Error  Cleanup Dangling VM Groups On Test Server

    # Install the VCH now
    Log To Console  \nInstalling VCH to test server...
    ${output}=  Run VIC Machine Command  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${debug}  ${opsuser-args}  ${additional-args}
    Log  ${output}

    # Check for failure to obtain IP address
    # Example:
    # State of all interfaces:
    #   "public" IP: "waiting for IP"
    #   "client" IP: "waiting for IP"
    #   "management" IP: "waiting for IP"
    #   "bridge" IP: "waiting for IP"
    ${noIP}=  Set Variable If  '"waiting for IP"' in '''${output}'''  True  False
    Run Keyword If  ${noIP}  Log             Possible DHCP failure
    Run Keyword If  ${noIP}  Log To Console  Possible DHCP failure

    Should Contain  ${output}  Installer completed successfully

    Get Docker Params  ${output}  ${certs}
    Log To Console  Installer completed successfully: %{VCH-NAME}...

    [Return]  ${output}

Run VIC Machine Command
    [Tags]  secret
    [Arguments]  ${vic-machine}  ${appliance-iso}  ${bootstrap-iso}  ${certs}  ${vol}  ${debug}  ${opsuser-args}  ${additional-args}
    ${output}=  Run Keyword If  ${certs}  Run  ${vic-machine} create --debug ${debug} --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=${appliance-iso} --bootstrap-iso=${bootstrap-iso} --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --insecure-registry wdc-harbor-ci.eng.vmware.com --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:${vol} --container-network=%{PUBLIC_NETWORK}:public ${vicmachinetls} ${opsuser-args} ${additional-args}
    Run Keyword If  ${certs}  Should Contain  ${output}  Installer completed successfully
    Return From Keyword If  ${certs}  ${output}

    ${output}=  Run Keyword Unless  ${certs}  Run  ${vic-machine} create --debug ${debug} --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=${appliance-iso} --bootstrap-iso=${bootstrap-iso} --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --insecure-registry wdc-harbor-ci.eng.vmware.com --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:${vol} --container-network=%{PUBLIC_NETWORK}:public --no-tlsverify ${opsuser-args} ${additional-args}
    Run Keyword Unless  ${certs}  Should Contain  ${output}  Installer completed successfully
    [Return]  ${output}

Run Secret VIC Machine Delete Command
    [Tags]  secret
    [Arguments]  ${vch-name}
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux delete --name=${vch-name} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
    [Return]  ${rc}  ${output}

Run Secret VIC Machine Inspect Command
    [Tags]  secret
    [Arguments]  ${name}
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect --name=${name} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --thumbprint=%{TEST_THUMBPRINT} --compute-resource=%{TEST_RESOURCE}

    [Return]  ${rc}  ${output}

Run VIC Machine Delete Command
    ${rc}  ${output}=  Run Secret VIC Machine Delete Command  %{VCH-NAME}
    Log  ${output}
    ${status}=  Run Keyword And Return Status  Wait Until Keyword Succeeds  6x  5s  Check Delete Success  %{VCH-NAME}
    Assert  ${status}  vic-machine delete failed to remove %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Completed successfully

    # Check that the VCH folder (if created) is removed
    ${ret}=  Run  govc find %{TEST_DATACENTER}/vm -type f
    Should Not Contain  ${ret}  %{VCH-NAME}

    ${output}=  Run  rm -rf %{VCH-NAME}
    [Return]  ${output}

Run VIC Machine Inspect Command
    [Arguments]  ${name}=%{VCH-NAME}
    ${rc}  ${output}=  Run Secret VIC Machine Inspect Command  ${name}
    Log  ${output}
    Get Docker Params  ${output}  ${true}

Inspect VCH
    [Arguments]  ${expected}
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${expected}

Wait For VCH Initialization
    [Arguments]  ${attempts}=12x  ${interval}=10 seconds  ${name}=%{VCH-NAME}
    Wait Until Keyword Succeeds  ${attempts}  ${interval}  VCH Docker Info  ${name}

VCH Docker Info
    [Arguments]  ${name}=%{VCH-NAME}
    Run VIC Machine Inspect Command  ${name}
    ${rc}=  Run And Return Rc  docker %{VCH-PARAMS} info
    Should Be Equal As Integers  ${rc}  0

Check UpdateInProgress
    [Arguments]  ${expected}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep UpdateInProgress
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  ${expected}

# This keyword is used to match two patterns on the same line occurring in any order
Portlayer Log Should Match Regexp
    [Arguments]  ${pattern1}  ${pattern2}
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}=  Run And Return Rc  curl -sk %{VIC-ADMIN}/logs/port-layer.log -b ${cookies} | grep -ie \'${pattern1}\' | grep -iqe \'${pattern2}\'
    Should Be Equal As Integers  ${rc}  0

Gather Logs From Test Server
    [Arguments]  ${name-suffix}=${EMPTY}
    Run Keyword And Continue On Failure  Run  zip %{VCH-NAME}-certs -r %{VCH-NAME}
    Curl Container Logs  ${name-suffix}
    ${host}=  Wait Until Keyword Succeeds  12x  10s  Get VM Host Name  %{VCH-NAME}
    Log  ${host}
    ${out}=  Run  govc datastore.download -host ${host} %{VCH-NAME}/vmware.log %{VCH-NAME}-vmware${name-suffix}.log
    Log  ${out}
    Should Contain  ${out}  OK
    ${out}=  Run  govc datastore.download -host ${host} %{VCH-NAME}/tether.debug %{VCH-NAME}-tether${name-suffix}.debug
    Log  ${out}
    Should Contain  ${out}  OK
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc logs -log=vmkernel -n=10000 > vmkernel${name-suffix}.log

Curl Container Logs
    [Arguments]  ${name-suffix}=${EMPTY}
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${out}=  Run  curl -k -b ${cookies} %{VIC-ADMIN}/container-logs.zip -o ${SUITE NAME}-%{VCH-NAME}-container-logs${name-suffix}.zip
    Log  ${out}
    ${out}=  Run  curl -k -b ${cookies} %{VIC-ADMIN}/logs/port-layer.log
    Log  ${out}
    Should Not Contain  ${out}  SIGSEGV: segmentation violation

Curl VCH Admin Cookies
    [Arguments]  ${name-prefix}=${EMPTY}
    [Tags]  secret
    ${rc}  ${out}=  Run And Return Rc and Output  curl -k -D ${name-prefix}vic-admin-cookies -Fusername=%{TEST_USERNAME} -Fpassword=%{TEST_PASSWORD} %{VIC-ADMIN}/authentication
    [Return]  ${rc}  ${out}  ${name-prefix}vic-admin-cookies

Login To VCH Admin And Save Cookies
    ${rc}  ${tmp}=  Run And Return Rc And Output  mktemp -d -p /tmp
    Should Be Equal As Integers  ${rc}  0
    
    ${rc}  ${out}  ${cookies}=  Curl VCH Admin Cookies  ${tmp}/
    Log  ${out}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${cookies}

Check For The Proper Log Files
    [Arguments]  ${container}
    ${rc}  ${tmp}=  Run And Return Rc And Output  mktemp -d -p /tmp
    Should Be Equal As Integers  ${rc}  0

    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc and Output  curl -sk %{VIC-ADMIN}/container-logs.tar.gz -b ${cookies} -o ${tmp}/container-logs.tar.gz
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc and Output  tar tvzf ${tmp}/container-logs.tar.gz
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    @{words}=  Split String  ${container}  -
    Should Contain Any  ${output}  @{words}[0]/output.log  @{words}[1]/output.log
    Should Contain Any  ${output}  @{words}[0]/vmware.log  @{words}[1]/vmware.log
    Should Contain Any  ${output}  @{words}[0]/tether.debug  @{words}[1]/tether.debug

Scrape Logs For the Password
    [Tags]  secret
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}=  Run And Return Rc  curl -sk %{VIC-ADMIN}/logs/port-layer.log -b ${cookies} | grep -q "%{TEST_PASSWORD}"
    Should Be Equal As Integers  ${rc}  1
    ${rc}=  Run And Return Rc  curl -sk %{VIC-ADMIN}/logs/init.log -b ${cookies} | grep -q "%{TEST_PASSWORD}"
    Should Be Equal As Integers  ${rc}  1
    ${rc}=  Run And Return Rc  curl -sk %{VIC-ADMIN}/logs/docker-personality.log -b ${cookies} | grep -q "%{TEST_PASSWORD}"
    Should Be Equal As Integers  ${rc}  1
    ${rc}=  Run And Return Rc  curl -sk %{VIC-ADMIN}/logs/vicadmin.log -b ${cookies} | grep -q "%{TEST_PASSWORD}"
    Should Be Equal As Integers  ${rc}  1

Cleanup VIC Appliance On Test Server
    ${sessions}=  Run Keyword And Ignore Error  Get Session List
    Log  ${sessions}
    ${memory}=  Run Keyword And Ignore Error  Get Hostd Memory Consumption
    Log  ${memory}
    Log To Console  Gathering logs from the test server %{VCH-NAME}
    Run Keyword And Continue On Failure  Gather Logs From Test Server
    # Exit from Cleanup if VCH-NAME is currently in exception list
    ${exclude}=  Check If VCH Is In Exception  vch=%{VCH-NAME}
    Run Keyword If  ${exclude}  Wait Until Keyword Succeeds  3x  5 seconds  Remove All Containers
    Return From Keyword If  ${exclude}
    Log To Console  Deleting the VCH appliance %{VCH-NAME}
    ${output}=  Run VIC Machine Delete Command
    Log  ${output}
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network
    Run Keyword And Ignore Error  Run  govc datastore.rm %{VCH-NAME}-VOL
    [Return]  ${output}

Cleanup VCH Bridge Network
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.remove %{BRIDGE_NETWORK}
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.info
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Not Contain  ${out}  %{BRIDGE_NETWORK}

    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Remove VC Distributed Portgroup  %{BRIDGE_NETWORK}
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  govc ls network
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Not Contain  ${out}  %{BRIDGE_NETWORK}

    Remove Environment Variable  BRIDGE_NETWORK

Add VC Distributed Portgroup
    [Arguments]  ${dvs}  ${pg}
    ${out}=  Run  govc dvs.portgroup.add -nports 12 -dc=%{TEST_DATACENTER} -dvs=${dvs} ${pg}
    Log  ${out}

Remove VC Distributed Portgroup
    [Arguments]  ${pg}
    ${out}=  Run  govc object.destroy network/${pg}
    Log  ${out}

Cleanup Datastore On Test Server
    ${out}=  Run  govc datastore.ls
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${items}=  Split To Lines  ${out}
    :FOR  ${item}  IN  @{items}
    \   ${build}=  Split String  ${item}  -
    \   # Skip any item that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   # Skip any item in the exception list
    \   @{name}=  Split String  ${item}  -VOL
    \   ${skip}=  Check If VCH Is In Exception  vch=@{name}[0]  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any item that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   Log To Console  Removing the following item from datastore: ${item}
    \   ${rc}  ${out}=    Run And Return Rc And Output  govc datastore.rm ${item}
    \   Continue For Loop If  ${rc} != 0
    \   Wait Until Keyword Succeeds  6x  5s  Check Delete Success  ${item}

Cleanup Dangling VMs On Test Server
    ${out}=  Run  govc ls vm
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${vms}=  Split To Lines  ${out}
    :FOR  ${vm}  IN  @{vms}
    \   ${vm}=  Fetch From Right  ${vm}  /
    \   ${build}=  Split String  ${vm}  -
    \   # Skip any VM that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   ${skip}=  Check If VCH Is In Exception  vch=${vm}  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any VM that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   ${uuid}=  Run  govc vm.info -json\=true ${vm} | jq -r '.VirtualMachines[0].Config.Uuid'
    \   Log To Console  Destroying dangling VCH: ${vm}
    \   ${rc}  ${output}=  Run Secret VIC Machine Delete Command  ${vm}
    \   Run Keyword And Continue On Failure  Wait Until Keyword Succeeds  6x  5s  Check Delete Success  ${vm}

Cleanup Dangling Folders On Test Server
    ${out}=  Run  govc ls vm
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${vms}=  Split To Lines  ${out}
    :FOR  ${vm}  IN  @{vms}
    \   ${vm}=  Fetch From Right  ${vm}  /
    \   ${build}=  Split String  ${vm}  -
    \   # Skip any VM that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   ${skip}=  Check If VCH Is In Exception  vch=${vm}  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any VM that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   Log To Console  Destroying dangling VCH Folder: vm/${vm}
    \   ${rc}=  Run and Return Rc  govc object.destroy vm/${vm}
    \   Run KeyWord If  '$rc' != '0'  Log  failed to clean up vch folder path vm/${vm}
    \   ${rc}  ${output}=  Run Secret VIC Machine Delete Command  ${vm}
    \   Run Keyword And Continue On Failure  Wait Until Keyword Succeeds  6x  5s  Check Delete Success  ${vm}

Cleanup Dangling Resource Pools On Test Server
    ${out}=  Run  govc ls host/*/Resources/*
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${pools}=  Split To Lines  ${out}
    :FOR  ${pool}  IN  @{pools}
    \   ${shortPool}=  Fetch From Right  ${pool}  /
    \   ${build}=  Split String  ${shortPool}  -
    \   # Skip any pool that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   # Skip Resource Pools belonging to VCHs in the exception list
    \   ${skip}=  Check If VCH Is In Exception  vch=${shortPool}  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any pool that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   Log To Console  Destroying dangling resource pool: ${pool}
    \   ${output}=  Run  govc pool.destroy ${pool}
    \   Log  ${output}

Cleanup Dangling Networks On Test Server
    ${out}=  Run  govc ls network
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${nets}=  Split To Lines  ${out}
    :FOR  ${net}  IN  @{nets}
    \   ${net}=  Fetch From Right  ${net}  /
    \   ${build}=  Split String  ${net}  -
    \   # Skip any Network that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   # Don't blow away our unique bridge network when running locally
    \   Continue For Loop If  '@{build}[1]' == '0'
    \   # Skip any Network that is attached to a VCH in the exception list
    \   ${skip}=  Check If VCH Is In Exception  vch=@{build}[0]-@{build}[1]-@{build}[2]  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any Network that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   ${uuid}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.remove ${net}
    \   ${uuid}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Remove VC Distributed Portgroup  ${net}

Cleanup Dangling VM Groups On Test Server
    ${out}=  Run  govc cluster.group.ls
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${groups}=  Split To Lines  ${out}
    :FOR  ${group}  IN  @{groups}
    \   ${build}=  Split String  ${group}  -
    \   # Skip any group that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   # Skip any Group that is attached to a VCH in the exception list
    \   ${skip}=  Check If VCH Is In Exception  vch=${group}  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any Group that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   Log To Console  Destroying dangling DRS VM Group: ${group}
    \   ${output}=  Run  govc cluster.group.remove -name ${group}

Cleanup Dangling vSwitches On Test Server
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.vswitch.info | grep VCH
    Log  ${out}
    ${exceptions}=  Get Environment Variable  VM_EXCEPTIONS  ${EMPTY}
    ${nets}=  Split To Lines  ${out}
    :FOR  ${net}  IN  @{nets}
    \   ${net}=  Fetch From Right  ${net}  ${SPACE}
    \   ${build}=  Split String  ${net}  -
    \   # Skip any vSwitch that is not associated with integration tests
    \   Continue For Loop If  '@{build}[0]' != 'VCH'
    \   # Skip any switch that is attached to a VCH in the exception list
    \   ${skip}=  Check If VCH Is In Exception  vch=@{build}[0]-@{build}[1]-@{build}[2]  exceptions=${exceptions}
    \   Continue For Loop If  ${skip}
    \   # Skip any vSwitch that is still running
    \   ${state}=  Get State Of Drone Build  @{build}[1]
    \   Continue For Loop If  '${state}' == 'running'
    \   ${uuid}=  Run  govc host.vswitch.remove ${net}

Get Scratch Disk From VM Info
    [Arguments]  ${vm}
    ${disks}=  Run  govc vm.info -json ${vm} | jq -r '.VirtualMachines[].Layout.Disk[].DiskFile[]'
    ${disks}=  Split To Lines  ${disks}
    :FOR  ${disk}  IN  @{disks}
    \   ${disk}=  Fetch From Right  ${disk}  ${SPACE}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${disk}  scratch.vmdk
    \   Return From Keyword If  ${status}  ${disk}

Cleanup Dangling Containers On Test Server
    ${vms}=  Run  govc ls vm
    ${vms}=  Split To Lines  ${vms}
    :FOR  ${vm}  IN  @{vms}
    \   # Ignore VCH's, we only care about containers at this point
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${vm}  VCH
    \   Continue For Loop If  ${status}
    \   ${disk}=  Get Scratch Disk From VM Info  ${vm}
    \   ${vch}=  Fetch From Left  ${disk}  /
    \   ${vch}=  Split String  ${vch}  -
    \   # Skip any VM that is not associated with integration tests
    \   Continue For Loop If  '@{vch}[0]' != 'VCH'
    \   ${state}=  Get State Of Drone Build  @{vch}[1]
    \   # Skip any VM that is still running
    \   Continue For Loop If  '${state}' == 'running'
    \   # Destroy the VM and remove it from datastore because it is a dangling container
    \   Log To Console  Cleaning up dangling container: ${vm}
    \   ${out}=  Run  govc vm.destroy ${vm}
    \   ${name}=  Fetch From Right  ${vm}  /
    \   ${out}=  Run  govc datastore.rm ${name}
    \   Wait Until Keyword Succeeds  6x  5s  Check Delete Success  ${name}

Get VCH ID
    [Arguments]  ${vch-name}
    ${ret}=  Run  bin/vic-machine-linux ls --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD}
    Should Not Contain  ${ret}  Error
    @{lines}=  Split To Lines  ${ret}
    :FOR  ${line}  IN  @{lines}
    \   # Get line with name ${vch-name}
    \   @{vch}=  Split String  ${line}
    \   ${len}=  Get Length  ${vch}
    \   Continue For Loop If  ${len} < 5
    \   ${name}=  Strip String  @{vch}[2]
    \   Continue For Loop If  '${name}' != '${vch-name}'
    \   ${vch-id}=  Strip String  @{vch}[0]
    \   Log To Console  \nVCH ID: ${vch-id}
    \   Log  VCH ID ${vch-id}
    \   Return From Keyword  ${vch-id}

# VCH upgrade helpers
Install VIC with version to Test Server
    [Arguments]  ${version}  ${insecureregistry}=  ${cleanup}=${true}  ${additional-args}=
    Log To Console  \nDownloading vic ${version} from gcp...
    ${time}=  Evaluate  time.clock()  modules=time
    ${rc}  ${output}=  Run And Return Rc And Output  wget https://storage.googleapis.com/vic-engine-releases/vic_${version}.tar.gz -O vic-${time}.tar.gz
    Create Directory  vic-${time}
    ${rc}  ${output}=  Run And Return Rc And Output  tar zxvf vic-${time}.tar.gz -C vic-${time}
    Install VIC Appliance To Test Server  vic-machine=./vic-${time}/vic/vic-machine-linux  appliance-iso=./vic-${time}/vic/appliance.iso  bootstrap-iso=./vic-${time}/vic/bootstrap.iso  certs=${false}  cleanup=${cleanup}  additional-args=${additional-args}  vol=default ${insecureregistry}

    Set Environment Variable  VIC-ADMIN  %{VCH-IP}:2378
    Set Environment Variable  INITIAL-VERSION  ${version}
    Run  rm -rf vic-${time}.tar.gz vic-${time}

Clean up VIC Appliance And Local Binary
    Cleanup VIC Appliance On Test Server
    Run  rm -rf vic.tar.gz vic

Upgrade
    Log To Console  \nUpgrading VCH...
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
    Log  ${output}
    Should Contain  ${output}  Completed successfully
    Should Not Contain  ${output}  Rolling back upgrade
    Should Be Equal As Integers  ${rc}  0

Upgrade with ID
    Log To Console  \nUpgrading VCH using vch ID...
    ${vch-id}=  Get VCH ID  %{VCH-NAME}
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --debug 1 --id=${vch-id} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT}
    Log  ${output}
    Should Contain  ${output}  Completed successfully
    Should Not Contain  ${output}  Rolling back upgrade
    Should Be Equal As Integers  ${rc}  0

Check Upgraded Version
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux version
    @{vers}=  Split String  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE}
    Log  ${output}
    Should Contain  ${output}  Completed successfully
    Should Contain  ${output}  @{vers}[2]
    Should Not Contain  ${output}  %{INITIAL-VERSION}
    Should Be Equal As Integers  ${rc}  0
    Log  ${output}
    Get Docker Params  ${output}  ${true}

Check Original Version
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux version
    @{vers}=  Split String  ${output}
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE}
    Log  ${output}
    Should Contain  ${output}  Completed successfully
    Should Contain  ${output}  %{INITIAL-VERSION}
    Should Be Equal As Integers  ${rc}  0
    Log  ${output}
    Get Docker Params  ${output}  ${true}

Rollback
     Log To Console  \nTesting rollback...
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux upgrade --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --force=true --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --rollback
    Log  ${output}
    Should Contain  ${output}  Completed successfully
    Should Be Equal As Integers  ${rc}  0

Enable VCH SSH
    [Arguments]  ${vic-machine}=bin/vic-machine-linux  ${rootpw}=%{TEST_PASSWORD}  ${target}=%{TEST_URL}%{TEST_DATACENTER}  ${password}=%{TEST_PASSWORD}  ${thumbprint}=%{TEST_THUMBPRINT}  ${name}=%{VCH-NAME}  ${user}=%{TEST_USERNAME}  ${resource}=%{TEST_RESOURCE}
    Log To Console  \nEnable SSH on vch...
    ${rc}  ${output}=  Run And Return Rc And Output  ${vic-machine} debug --rootpw ${rootpw} --target ${target} --password ${password} --thumbprint ${thumbprint} --name ${name} --user ${user} --compute-resource ${resource} --enable-ssh
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Completed successfully

Assert
    [Arguments]  ${status}  ${msg}
    ${envExists}=  Run Keyword And Return Status  Environment Variable Should Be Set  FAST_FAILURE
    Run Keyword If  ${envExists}  Run Keyword If  %{FAST_FAILURE}  Run Keyword Unless  ${status}  Fatal Error  ${msg}
    Run Keyword Unless  ${status}  Fail  ${msg}

Manually Cleanup VCH
    [Arguments]  ${vch-name}
    ${out}=  Run Keyword And Ignore Error  Run  govc vm.destroy ${vch-name}
    ${out}=  Run Keyword And Ignore Error  Run  govc pool.destroy host/*/Resources/${vch-name}
    ${out}=  Run Keyword And Ignore Error  Run  govc datastore.rm ${vch-name}
    ${out}=  Run Keyword And Ignore Error  Run  govc host.portgroup.remove ${vch-name}-bridge
    ${out}=  Run Keyword And Ignore Error  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  govc object.destroy vm/${vch-name}

Get Ops User Args
    ${run-as-ops-user}=  Get Environment Variable  RUN_AS_OPS_USER  0
    Return From Keyword If  '${run-as-ops-user}' == '0'  ${EMPTY}
    Log To Console  Running tests as ops user...
    Create Ops User Account On Test Server
    ${opsuser-args}=  Set Variable  --ops-user='%{VCH_OPS_USERNAME}' --ops-password='%{VCH_OPS_PASSWORD}' --ops-grant-perms
    [Return]  ${opsuser-args}

Create Ops User Account On Test Server
    Log To Console  \nChecking ops user account...
    ${tokens}=  Split String  %{VCH_OPS_USERNAME}  @
    ${ops-username}=  Get From List  ${tokens}  0
    ${ops-domain}=  Get From List  ${tokens}  1
    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -p %{SSH_TEST_PASSWORD} ssh -o StrictHostKeyChecking=no %{SSH_TEST_USERNAME}@%{TEST_URL} /usr/lib/vmware-vmafd/bin/dir-cli user find-by-name --account ${ops-username} --login %{TEST_USERNAME} --password %{TEST_PASSWORD}

    ${status}=  Run Keyword And Return Status  Should Not Contain  ${output}  ERROR_NO_SUCH_USER
    # If output does not contain ERROR_NO_SUCH_USER, the user already exists and we bail
    Run Keyword If  ${status}  Log To Console  \nOps user already exists...\n
    Run Keyword If  ${status}  Return From Keyword
    # Otherwise, create the user
    Log To Console  \nCreating ops user...\n
    ${rc}  ${output}=  Run And Return Rc And Output  sshpass -p %{SSH_TEST_PASSWORD} ssh -o StrictHostKeyChecking=no %{SSH_TEST_USERNAME}@%{TEST_URL} /usr/lib/vmware-vmafd/bin/dir-cli user create --account ${ops-username} --user-password %{VCH_OPS_PASSWORD} --first-name %{VCH_OPS_USERNAME} --last-name ${ops-domain} --login %{TEST_USERNAME} --password %{TEST_PASSWORD}
    Should Contain  ${output}  created successfully
