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
Documentation     Test 23-03 - VCH Create
Resource          ../../resources/Util.robot
Resource          ../../resources/Group23-VIC-Machine-Service-Util.robot
Suite Setup       Setup
Suite Teardown    Teardown
Test Setup        Cleanup Test Server
Default Tags


*** Keywords ***
Setup
    Start VIC Machine Server
    Set Test Environment Variables

    ${PUBLIC_NETWORK}=  Remove String  %{PUBLIC_NETWORK}  '
    Set Suite Variable    ${PUBLIC_NETWORK}


Teardown
    Stop VIC Machine Server
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network
    Cleanup Test Server


Cleanup Test Server
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server


Create VCH
    [Arguments]    ${data}
    Post Path Under Target    vch    ${data}


Create VCH Within Datacenter
    [Arguments]    ${data}
    ${dcID}=    Get Datacenter ID
    Post Path Under Target    datacenter/${dcID}/vch    ${data}


Inspect VCH Config ${name}
    ${RC}    ${OUTPUT}=    Run And Return Rc And Output    bin/vic-machine-linux inspect config --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --compute-resource=%{TEST_RESOURCE} --name=${name} --format raw
    Should Be Equal As Integers    ${RC}    0
    Set Test Variable    ${OUTPUT}


Get VCH ${name}
    Get Path Under Target    vch
    ${id}=    Run    echo '${OUTPUT}' | jq -r '.vchs[] | select(.name=="${name}").id'
    Set Test Variable    ${id}

    Get Path Under Target    vch/${id}


Pull Busy Box
    [Arguments]    ${docker_params}    ${expected_rc}    ${expected_output}

    ${rc}  ${output}=  Run And Return Rc And Output  docker -H ${docker_params} pull busybox

    Should Be Equal As Integers    ${rc}    ${expected_rc}
    Should Contain    ${output}    ${expected_output}


Get Docker Host Params
    [Arguments]    ${vch_name}
    Run Keyword And Continue On Failure    Wait Until Keyword Succeeds    30x    10s    Get Docker Params API    ${vch_name}
    Run Keyword If    "${docker_host}" == "null"    Run VIC Machine Inspect Command   ${vch_name}
    ${vch_params}=    Get Environment Variable    VCH-PARAMS    ${EMPTY}
    Run Keyword if    "${vch_params}" != "${EMPTY}"    Extract Docker IP And Port    ${vch_params}


Extract Docker IP And Port
    [Arguments]    ${vch_params}
    Log To Console    Get Docker Params API did not work, use Run VIC Machine Inspect Command instead for docker_host info
    @{hostParts}=    Split String    ${vch_params}
    ${docker_host}=    Strip String    @{hostParts}[1]
    Set Test Variable    ${docker_host}


*** Test Cases ***
Create minimal VCH
    Create VCH    '{"name":"%{VCH-NAME}-api-test-minimal","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Created


    Inspect VCH Config %{VCH-NAME}-api-test-minimal

    Output Should Contain    --image-store=ds://%{TEST_DATASTORE}
    Output Should Contain    --bridge-network=%{BRIDGE_NETWORK}


    Get VCH %{VCH-NAME}-api-test-minimal

    Property Should Be Equal        .name                                %{VCH-NAME}-api-test-minimal

    Property Should Not Be Equal    .compute.resource.id                 null

    Property Should Contain         .storage.image_stores[0]             %{TEST_DATASTORE}
    Property Should Be Equal        .storage.base_image_size.value       8000000
    Property Should Be Equal        .storage.base_image_size.units       KB

    Property Should Contain         .auth.server.certificate.pem         -----BEGIN CERTIFICATE-----
    Property Should Be Equal        .auth.server.private_key.pem         null

    Property Should Contain         .network.bridge.ip_range             172.16.0.0/12

    Property Should Contain         .runtime.power_state                 poweredOn
    Property Should Contain         .runtime.upgrade_status              Up to date

    Get Docker Host Params    %{VCH-NAME}-api-test-minimal

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}-api-test-minimal


Create minimal VCH within datacenter
    Create VCH Within Datacenter    '{"name":"%{VCH-NAME}-api-test-dc","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Created


    Inspect VCH Config %{VCH-NAME}-api-test-dc

    Output Should Contain    --image-store=ds://%{TEST_DATASTORE}
    Output Should Contain    --bridge-network=%{BRIDGE_NETWORK}


    Get VCH %{VCH-NAME}-api-test-dc

    Property Should Be Equal        .name                                %{VCH-NAME}-api-test-dc

    Property Should Not Be Equal    .compute.resource.id                 null

    Property Should Contain         .storage.image_stores[0]             %{TEST_DATASTORE}
    Property Should Be Equal        .storage.base_image_size.value       8000000
    Property Should Be Equal        .storage.base_image_size.units       KB

    Property Should Contain         .auth.server.certificate.pem         -----BEGIN CERTIFICATE-----
    Property Should Be Equal        .auth.server.private_key.pem         null

    Property Should Contain         .network.bridge.ip_range             172.16.0.0/12

    Property Should Contain         .runtime.power_state                 poweredOn
    Property Should Contain         .runtime.upgrade_status              Up to date

    Get Docker Host Params    %{VCH-NAME}-api-test-dc


    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}-api-test-dc


Create complex VCH
    Create VCH    '{"name":"%{VCH-NAME}-api-test-complex","debug":3,"compute":{"cpu":{"limit":{"units":"MHz","value":2345},"reservation":{"units":"GHz","value":2},"shares":{"level":"high"}},"memory":{"limit":{"units":"MiB","value":1200},"reservation":{"units":"MiB","value":501},"shares":{"number":81910}},"resource":{"name":"%{TEST_RESOURCE}"},"affinity":{"use_vm_group":true}},"endpoint":{"cpu":{"sockets":2},"memory":{"units":"MiB","value":3072}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"],"volume_stores":[{"datastore":"ds://%{TEST_DATASTORE}/test-volumes/foo","label":"foo"}],"base_image_size":{"units":"B","value":16000000}},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"container":[{"alias":"vic-containers","firewall":"outbound","nameservers":["8.8.8.8","8.8.4.4"],"port_group":{"name":"${PUBLIC_NETWORK}"},"gateway":{"address":"203.0.113.1","routing_destinations":["203.0.113.1/24"]},"ip_ranges":["203.0.113.8/31"]}],"public":{"port_group":{"name":"${PUBLIC_NETWORK}"},"static":"192.168.100.22/24","gateway":{"address":"192.168.100.1"},"nameservers":["192.168.110.10","192.168.1.1"]}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}},"syslog_addr":"tcp://syslog.example.com:4444", "container": {"name_convention": "container-{id}"}}'

    Verify Return Code
    Verify Status Created


    Inspect VCH Config %{VCH-NAME}-api-test-complex

    Output Should Contain    --debug=3

    Output Should Contain    --cpu=2345
    Output Should Contain    --cpu-reservation=2000
    Output Should Contain    --cpu-shares=high
    Output Should Contain    --memory=1200
    Output Should Contain    --memory-reservation=501
    Output Should Contain    --memory-shares=81910

    Output Should Contain    --affinity-vm-group=true

    Output Should Contain    --endpoint-cpu=2
    Output Should Contain    --endpoint-memory=3072

    Output Should Contain    --image-store=ds://%{TEST_DATASTORE}
    Output Should Contain    --volume-store=ds://%{TEST_DATASTORE}/test-volumes/foo:foo
    Output Should Contain    --base-image-size=16MB

    Output Should Contain    --bridge-network=%{BRIDGE_NETWORK}
    Output Should Contain    --container-network=${PUBLIC_NETWORK}:vic-containers
    Output Should Contain    --container-network-gateway=${PUBLIC_NETWORK}:203.0.113.1/24
    Output Should Contain    --container-network-ip-range=${PUBLIC_NETWORK}:203.0.113.8/31
    Output Should Contain    --container-network-dns=${PUBLIC_NETWORK}:8.8.8.8
    Output Should Contain    --container-network-dns=${PUBLIC_NETWORK}:8.8.4.4
    Output Should Contain    --container-network-firewall=${PUBLIC_NETWORK}:outbound

    Output Should Contain    --public-network-gateway=192.168.100.1
    Output Should Contain    --public-network-ip=192.168.100.22/24
    Output Should Contain    --dns-server=192.168.110.10
    Output Should Contain    --dns-server=192.168.1.1

    Output Should Contain    --insecure-registry=https://insecure.example.com
    Output Should Contain    --whitelist-registry=10.0.0.0/8
    Output Should Contain    --whitelist-registry=https://insecure.example.com


    Get VCH %{VCH-NAME}-api-test-complex

    Property Should Be Equal        .name                                %{VCH-NAME}-api-test-complex
    Property Should Be Equal        .debug                               3
    Property Should Be Equal        .syslog_addr                         tcp://syslog.example.com:4444

    Property Should Not Be Equal    .compute.resource.id                 null
    Property Should Be Equal        .compute.cpu.limit.value             2345
    Property Should Be Equal        .compute.cpu.limit.units             MHz
    Property Should Be Equal        .compute.cpu.reservation.value       2000
    Property Should Be Equal        .compute.cpu.reservation.units       MHz
    Property Should Be Equal        .compute.cpu.shares.level            high
    Property Should Be Equal        .compute.memory.limit.value          1200
    Property Should Be Equal        .compute.memory.limit.units          MiB
    Property Should Be Equal        .compute.memory.reservation.value    501
    Property Should Be Equal        .compute.memory.reservation.units    MiB
    Property Should Be Equal        .compute.memory.shares.number        81910
    Property Should Be Equal        .compute.affinity.use_vm_group       true

    Property Should Be Equal        .endpoint.cpu.sockets                2
    Property Should Be Equal        .endpoint.memory.value               3072
    Property Should Be Equal        .endpoint.memory.units               MiB

    Property Should Contain         .storage.image_stores[0]             %{TEST_DATASTORE}
    Property Should Contain         .storage.volume_stores[0].datastore  %{TEST_DATASTORE}/test-volumes/foo
    Property Should Contain         .storage.volume_stores[0].label      foo
    Property Should Be Equal        .storage.base_image_size.value       16000
    Property Should Be Equal        .storage.base_image_size.units       KB

    Property Should Be Equal        .registry.image_fetch_proxy.http     http://example.com
    Property Should Be Equal        .registry.image_fetch_proxy.https    https://example.com
    Property Should Contain         .registry.insecure | join(" ")       https://insecure.example.com
    Property Should Contain         .registry.whitelist | join(" ")      https://insecure.example.com
    Property Should Contain         .registry.whitelist | join(" ")      10.0.0.0/8

    Property Should Contain         .auth.server.certificate.pem         -----BEGIN CERTIFICATE-----
    Property Should Be Equal        .auth.server.private_key.pem         null

    Property Should Be Equal        .network.bridge.ip_range             172.16.0.0/12
    Property Should Be Equal        .network.container[0].alias          vic-containers
    Property Should Be Equal        .network.container[0].firewall       outbound
    Property Should Be Equal        .network.container[0].ip_ranges[0]   203.0.113.8/31

    Property Should Be Equal        .network.container[0].nameservers[0]                   8.8.8.8
    Property Should Be Equal        .network.container[0].nameservers[1]                   8.8.4.4
    Property Should Be Equal        .network.container[0].gateway.address                  203.0.113.1
    Property Should Be Equal        .network.container[0].gateway.routing_destinations[0]  203.0.113.1/24

    Property Should Be Equal        .network.public.gateway.address      192.168.100.1
    Property Should Be Equal        .network.public.nameservers[0]       192.168.110.10
    Property Should Be Equal        .network.public.nameservers[1]       192.168.1.1

    Property Should Be Equal        .runtime.power_state                 poweredOn
    Property Should Be Equal        .runtime.upgrade_status              Up to date

    Property Should Be Equal        .container.name_convention           container-{id}


    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}-api-test-complex


Fail to create VCH with invalid operations credentials
    Create VCH    '{"name":"%{VCH-NAME}-api-bad-ops","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"endpoint":{"operations_credentials":{"user":"invalid","password":"invalid"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    operations credentials


Fail to create VCH with invalid datastore
    Create VCH    '{"name":"%{VCH-NAME}-api-bad-storage","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}-invalid"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    datastore


Fail to create VCH with invalid compute
    Create VCH    '{"name":"%{VCH-NAME}-api-bad-compute","compute":{"resource":{"name":"%{TEST_RESOURCE}-invalid"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    compute resource


Fail to create VCH without network
    Create VCH    '{"name":"%{VCH-NAME}-api-bad-network","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    network


Fail to create VCH with gateway without static address
    Create VCH    '{"name":"%{VCH-NAME}-api-bad-gateway","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"},"gateway":{"address":"127.0.0.1","routing_destinations":[]}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    static

Fail to create VCH with a name containing invalid characters
    Create VCH    '{"name":"%{VCH-NAME}_%x_invalid-character-in-name","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain  unsupported character

Fail to create VCH with a very long name (over 31 characters)
    Create VCH    '{"name":"%{VCH-NAME}-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain  length exceeds 31 characters

Fail to create VCH with a name that is already in use
    ${there_can_only_be_one}=    Set Variable    %{VCH-NAME}-highlander

    Create VCH    '{"name":"${there_can_only_be_one}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Created

    Create VCH    '{"name":"${there_can_only_be_one}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code

    ${status}=    Get State Of Github Issue    7749
    Run Keyword If  '${status}' == 'closed'  Fail  Test 23-03-VCH-Create.robot "Fail to create VCH with a name that is already in use" needs to be updated now that Issue #7749 has been resolved
    # Issue 7749 should provide the correct return error
    Verify Status Internal Server Error

    Output Should Contain    already exists

    [Teardown]    Run Secret VIC Machine Delete Command    ${there_can_only_be_one}


Fail to create a VCH with an invalid container name name convention
    ${invalid_name_convention}=    Set Variable    192.168.1.1-mycontainer

    Create VCH    '{"name":"%{VCH-NAME}-api-test-minimal","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}, "container":{"name_convention": "${invalid_name_convention}"}}'

    Verify Return Code
    Verify Status Unprocessable Entity

    Output Should Contain    container.name_convention in body should match


Fail to create a VCH specifying an ID
    ${status}=    Get State Of Github Issue    7696
    Run Keyword If  '${status}' == 'closed'  Fail  Test 23-03-VCH-Create.robot "Fail to create a VCH specifying an ID" needs to be updated now that Issue #7696 has been resolved
    # Issue 7696 should provide a better error message for the error or invalidate this test

#    ${new_vch}=    Set Variable    %{VCH-NAME}-produce_id
#    ${id_vch}=    Set Variable    %{VCH-NAME}-never_created
#
#    Create VCH    '{"name":"${new_vch}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'
#
#    Verify Return Code
#    Verify Status Created
#
#    Get VCH ${new_vch}
#
#    Post Path Under Target    vch    '{"id":"${id}", "name":"${id_vch}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'
#    Verify Return Code
#    Verify Status Created
#
#    Output Should Contain    **Need updated error message here**
#    Delete Path Under Target    vch/${id}
#    Get VCH ${id_vch}
#    Delete Path Under Target    vch/${id}


Fail to create VCH where http != https (on http key/pair) in image_fetch_proxy - registry settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"https://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    error processing proxies: Could not parse HTTP proxy


Fail to create VCH where https != http (on https key/pair) in image_fetch_proxy - registry settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"http://example.com"},"insecure":["https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    error processing proxies: Could not parse HTTPS proxy


Fail to create VCH where whitelist contains an int and not string - registry settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":[100008]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    cannot unmarshal number into Go struct field VCHRegistry.whitelist of type string


Fail to create VCH where whitelist contains invalid character - registry settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":[10.0.0./8]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    failed, because invalid character


Create VCH where whitelist registry contains valid registry wildcard domain and validate
    ${wildcard_registry}=    Set Variable    "*.docker.com"

    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"whitelist":[${wildcard_registry}]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Created

    Get Docker Host Params    %{VCH-NAME}

    Wait Until Keyword Succeeds    10x    2s    Pull Busy Box    ${docker_host} --tls    0    Digest

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}


Fail to validate created VCH where whitelist registry contains unauthorized registry wildcard domain - whitelist settings
    ${invalid_wildcard_registry}=    Set Variable    "*.docker.gov"

    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"whitelist":[${invalid_wildcard_registry}]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Created

    Get Docker Host Params    %{VCH-NAME}

    Wait Until Keyword Succeeds    10x    2s    Pull Busy Box    ${docker_host} --tls    1    Access denied to unauthorized registry

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}


Fail to create VCH where insecure property contains invalid char - registry settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":[https://insecure.example.com],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    failed, because invalid character


Fail to create VCH where insecure property contains an int in one of the string arrays - registry settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com", 101010101010, "https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    failed, because json: cannot unmarshal number into Go struct field


Fail to create VCH where compute resource property contains invalid data - resource settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_registry","compute":{"resource":{"name":"TEST_RESOURCE"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    not found
    Output Should Contain    TEST_RESOURCE


Fail to create VCH where compute cpu limit property is very high - resource settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_resource","compute":{"cpu":{"limit":{"units":"GHz","value":6969696969696969},"reservation":{"units":"GHz","value":2969696969696969},"shares":{"level":"high"}},"memory":{"limit":{"units":"MiB","value":1200},"reservation":{"units":"MiB","value":501},"shares":{"number":81910}},"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code

    ${status}=    Get State Of Github Issue    7750
    Run Keyword If  '${status}' == 'closed'  Fail  Test 23-03-VCH-Create.robot "Fail to create VCH where compute cpu limit property is very high - resource settings" needs to be updated now that Issue #7750 has been resolved
    # Issue 7750 should provide the correct return error
    Verify Status Internal Server Error

    Output Should Contain    The amount of CPU resource available in the parent resource pool is insufficient for the operation.


Fail to create VCH where an invalid target path is specified - resource settings
    Create VCH    '{"name":"%{VCH-NAME}-invalid_resource","compute":{"resource":{"name":"%{TEST_RESOURCE}/GO/GO/GADGET"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    failed to validate VCH


Fail to create VCH where an invalid cname field is specified - security settings
    Create VCH    '{"name":"%{VCH-NAME}-security_settings","debug":3,"compute":{"cpu":{"limit":{"units":"MHz","value":2345},"reservation":{"units":"GHz","value":2},"shares":{"level":"high"}},"memory":{"limit":{"units":"MiB","value":1200},"reservation":{"units":"MiB","value":501},"shares":{"number":81910}},"resource":{"name":"%{TEST_RESOURCE}"}},"endpoint":{"cpu":{"sockets":2},"memory":{"units":"MiB","value":3072}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"],"volume_stores":[{"datastore":"ds://%{TEST_DATASTORE}/test-volumes/foo","label":"foo"}],"base_image_size":{"units":"B","value":16000000}},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"container":[{"alias":"vic-containers","firewall":"outbound","nameservers":["8.8.8.8","8.8.4.4"],"port_group":{"name":"${PUBLIC_NETWORK}"},"gateway":{"address":"203.0.113.1","routing_destinations":["203.0.113.1/24"]},"ip_ranges":["203.0.113.8/31"]}],"public":{"port_group":{"name":"${PUBLIC_NETWORK}"},"static":"192.168.100.22/24","gateway":{"address":"192.168.100.1"},"nameservers":["192.168.110.10","192.168.1.1"]}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":10,"organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}},"syslog_addr":"tcp://syslog.example.com:4444", "container": {"name_convention": "container-{id}"}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    failed, because json: cannot unmarshal number into Go struct field


Fail to create VCH where an invalid organization field is specified - security settings
    Create VCH    '{"name":"%{VCH-NAME}-security_settings","debug":3,"compute":{"cpu":{"limit":{"units":"MHz","value":2345},"reservation":{"units":"GHz","value":2},"shares":{"level":"high"}},"memory":{"limit":{"units":"MiB","value":1200},"reservation":{"units":"MiB","value":501},"shares":{"number":81910}},"resource":{"name":"%{TEST_RESOURCE}"}},"endpoint":{"cpu":{"sockets":2},"memory":{"units":"MiB","value":3072}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"],"volume_stores":[{"datastore":"ds://%{TEST_DATASTORE}/test-volumes/foo","label":"foo"}],"base_image_size":{"units":"B","value":16000000}},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"container":[{"alias":"vic-containers","firewall":"outbound","nameservers":["8.8.8.8","8.8.4.4"],"port_group":{"name":"${PUBLIC_NETWORK}"},"gateway":{"address":"203.0.113.1","routing_destinations":["203.0.113.1/24"]},"ip_ranges":["203.0.113.8/31"]}],"public":{"port_group":{"name":"${PUBLIC_NETWORK}"},"static":"192.168.100.22/24","gateway":{"address":"192.168.100.1"},"nameservers":["192.168.110.10","192.168.1.1"]}},"registry":{"image_fetch_proxy":{"http":"http://example.com","https":"https://example.com"},"insecure":["https://insecure.example.com"],"whitelist":["10.0.0.0/8"]},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":[10],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": true}},"syslog_addr":"tcp://syslog.example.com:4444", "container": {"name_convention": "container-{id}"}}'

    Verify Return Code
    Verify Status Bad Request

    Output Should Contain    failed, because json: cannot unmarshal number into Go struct field


# TODO: Review/Remove this test once Github ticket 7694 is resolved
Create VCH setting tls to false and validate - security settings
    ${status}=    Get State Of Github Issue    7694
    Run Keyword If  '${status}' == 'closed'  Fail  Test 23-03-VCH-Create.robot "Create VCH setting tls to false and validate - security settings" needs to be updated/removed now that Issue #7694 has been resolved
    # Issue 7694 should validate whether this test should be updated or removed

    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"no_tls":false}}'

    Verify Return Code
    Verify Status Created

    Get Docker Host Params    %{VCH-NAME}

    Wait Until Keyword Succeeds    10x    2s    Pull Busy Box    ${docker_host}    0    Digest

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}


# TODO: Review/Remove this test once Github ticket 7694 is resolved
Create VCH setting tls to true and validate - security settings
    ${status}=    Get State Of Github Issue    7694
    Run Keyword If  '${status}' == 'closed'  Fail  Test 23-03-VCH-Create.robot "Create VCH setting tls to true and validate - security settings" needs to be updated/removed now that Issue #7694 has been resolved
    # Issue 7694 should validate whether this test should be updated or removed

    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"no_tls":true}}'

    Verify Return Code
    Verify Status Created

    Get Docker Host Params    %{VCH-NAME}

    Wait Until Keyword Succeeds    20x    5s    Pull Busy Box    ${docker_host}    0    Digest

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}


Create VCH with no auth - security settings
    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}}}'

    Verify Return Code
    Verify Status Created

    Get Docker Host Params    %{VCH-NAME}

    Wait Until Keyword Succeeds    20x    5s    Pull Busy Box    ${docker_host}    0    Digest

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}


Create VCH without specifying certs on client params - security settings
    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"generate":{"cname":"vch.example.com","organization":["VMware, Inc."],"size":{"value":2048,"units":"bits"}}},"client":{"no_tls_verify": false}}}'

    Verify Return Code
    Verify Status Created

    Get Docker Host Params    %{VCH-NAME}

    Wait Until Keyword Succeeds    20x    5s    Pull Busy Box    ${docker_host} --tlsverify    1    could not read CA certificate

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}



Create VCH with invalid cert info on server params - security settings
    Create VCH    '{"name":"%{VCH-NAME}","compute":{"resource":{"name":"%{TEST_RESOURCE}"}},"storage":{"image_stores":["ds://%{TEST_DATASTORE}"]},"network":{"bridge":{"ip_range":"172.16.0.0/12","port_group":{"name":"%{BRIDGE_NETWORK}"}},"public":{"port_group":{"name":"${PUBLIC_NETWORK}"}}},"auth":{"server":{"certificate":{"pem":"-----BEGIN FAKE-----jfjfj39382vA==-----END FAKE-----"},"private_key":{"pem":"-----BEGIN FAKE-----jfFjfj39382vA==-----END FAKE-----"}}}}'

    Verify Return Code
    Verify Status Unprocessable Entity

    Output Should Contain    pem in body should match

    [Teardown]    Run Secret VIC Machine Delete Command    %{VCH-NAME}
