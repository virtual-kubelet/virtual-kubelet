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
Documentation  This resource provides any keywords related to Unified OVA

*** Variables ***
${ova_root_pwd}  ova-test-root-pwd
${ova_appliance_options}  --prop:appliance.root_pwd=${ova_root_pwd} --prop:appliance.permit_root_login=True

${ova_target_vm_name}  vic-unified-ova-integration-test
${ovftool_options}  --noSSLVerify --acceptAllEulas --name=${ova_target_vm_name} --diskMode=thin --powerOn --X:waitForIp --X:injectOvfEnv --X:enableHiddenProperties

${ova_network_ip0}  10.17.109.207
${ova_network_netmask0}  255.255.255.0
${ova_network_gateway}  10.17.109.253
${ova_network_dns}  10.118.81.1
${ova_network_searchpath}  eng.vmware.com
${ova_network_domain}  mrburns
${ova_network_options}  --prop:network.ip0=${ova_network_ip0} --prop:network.netmask0=${ova_network_netmask0} --prop:network.gateway=${ova_network_gateway} --prop:network.DNS=${ova_network_dns} --prop:network.searchpath=${ova_network_searchpath} --prop:network.domain=${ova_network_domain}

${ova_harbor_admin_password}  harbor-admin-passwd
${ova_harbor_db_password}  harbor-db-passwd
${ova_service_options}  --prop:registry.admin_password=${ova_harbor_admin_password} --prop:registry.db_password=${ova_harbor_db_password}

${ova_options}  ${ovftool_options} ${ova_appliance_options} ${ova_service_options}
${ova_options_with_network}  ${ova_options} ${ova_network_options} 

${tls_not_disabled}  False

*** Keywords ***
# Requires vc credential for govc
Deploy VIC-OVA To Test Server
    [Arguments]  ${dhcp}=True  ${protocol}=http  ${build}=False  ${user}=%{TEST_USERNAME}  ${password}=%{TEST_PASSWORD}  ${host}=%{TEST_URL_ARRAY}  ${datastore}=%{TEST_DATASTORE}  ${cluster}=%{TEST_RESOURCE}  ${datacenter}=%{TEST_DATACENTER}   
    Set Global Variable  ${ova_path}  bin/vic-1.1.1-${rev}.ova
    Run Keyword if  ${build}  Build Unified OVA
    ${rev}=  Run  git rev-parse --short HEAD

    Log To Console  \nCleanup environment...
    Run Keyword And Ignore Error  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc vm.destroy ${ova_target_vm_name}
    Run Keyword And Ignore Error  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc object.destroy /${datacenter}/vm/${ova_target_vm_name}

    Log To Console  \nStarting to deploy unified-ova to test server...
    Run Keyword If  ${dhcp}  Log To Console  ovftool --datastore=${datastore} ${ova_options} ${ova_path} 'vi://${user}:${password}@${host}/${cluster}'
    ...  ELSE  Log To Console  ovftool --datastore=${datastore} ${ova_options_with_network} ${ova_path} 'vi://${user}:${password}@${host}/${cluster}'
    ${out}=  Run Keyword If  ${dhcp}  Run  ovftool --datastore=${datastore} ${ova_options} --net:"Network"="vm-network" ${ova_path} 'vi://${user}:${password}@${host}/${cluster}'
    ...  ELSE  Run  ovftool --datastore=${datastore} ${ova_options_with_network} ${ova_path} 'vi://${user}:${password}@${host}/${cluster}'
    
    Should Contain  ${out}  Received IP address:
    Should Not Contain  ${out}  None

    ${out}=  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc ls /ha-datacenter/host/cls/
    ${out}=  Split To Lines  ${out}
    ${idx}=  Set Variable  1
    :FOR  ${line}  IN  @{out}
    \   Continue For Loop If  '${line}' == '/ha-datacenter/host/cls/Resources'
    \   ${ip}=  Fetch From Right  ${line}  /
    \   Set Suite Variable  ${esx${idx}-ip}  ${ip}
    \   ${idx}=  Evaluate  ${idx}+1

    Run Keyword And Ignore Error  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc host.esxcli -host.ip=${esx1-ip} system settings advanced set -o /Net/GuestIPHack -i 1
    Run Keyword And Ignore Error  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc host.esxcli -host.ip=${esx2-ip} system settings advanced set -o /Net/GuestIPHack -i 1
    Run Keyword And Ignore Error  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc host.esxcli -host.ip=${esx3-ip} system settings advanced set -o /Net/GuestIPHack -i 1

    ${ip}=  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc vm.ip -esxcli vic-unified-ova-integration-test

    Set Environment Variable  HARBOR_IP  ${ip}
    
    Log To Console  \nHarbor IP: %{HARBOR_IP}
    
    Log To Console  Waiting for Harbor to Come Up...
    :FOR  ${i}  IN RANGE  20
    \  ${out}=  Run  curl -k ${protocol}://%{HARBOR_IP}
    \  Log  ${out}
    \  ${status}=  Run Keyword And Return Status  Should Not Contain  ${out}  502 Bad Gateway
    \  ${status}=  Run Keyword If  ${status}  Run Keyword And Return Status  Should Not Contain  ${out}  Connection refused
    \  ${status}=  Run Keyword If  ${status}  Run Keyword And Return Status  Should Contain  ${out}  <title>Harbor</title>
    \  Return From Keyword If  ${status}  %{HARBOR_IP}
    \  Sleep  30s
    Fail  Harbor failed to come up properly!
    [Return]  %{HARBOR_IP}

# Requires vc credential for govc
Cleanup VIC-OVA On Test Server  
    [Arguments]  ${url}=%{GOVC_URL}  ${username}=%{GOVC_USERNAME}  ${password}=%{GOVC_PASSWORD}
    ${rc}  ${output}=  Run And Return Rc And Output  GOVC_URL=${url} GOVC_USERNAME=${username} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc vm.destroy ${ova_target_vm_name}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Run Keyword And Ignore Error  Run  GOVC_URL=${url} GOVC_USERNAME=${username} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc object.destroy /%{TEST_DATACENTER}/vm/${ova_target_vm_name}
    Log To Console  \nUnified-OVA deployment is cleaned up on test server

Build Unified OVA
    [Arguments]  ${user}=%{TEST_USERNAME}  ${password}=%{TEST_PASSWORD}  ${host}=%{TEST_URL}
    Log To Console  \nStarting to build Unified OVA... 
    Log To Console  \nRemove stale local OVA artifacts
    Run  Remove OVA Artifacts Locally
    ${out}=  Run   PACKER_ESX_HOST=${host} PACKER_USER=${user} PACKER_PASSWORD=${password} make ova-release 
    Log  ${out}
    @{out}=  Split To Lines  ${out}
    Should Not Contain  @{out}[-1]  Error
    Log To Console  \nUnified OVA is built successfully

Remove OVA Artifacts Locally
    ${rev}=  Run  git rev-parse --short HEAD
    Remove Files  bin/vic-1.1.1-${rev}.ova bin/vic-1.1.1-${rev}.ovf bin/vic-1.1.1-${rev}.mk bin/vic-1.1.1-${rev}-disk*.vmdk
