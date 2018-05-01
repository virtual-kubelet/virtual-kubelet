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
Documentation  Test 15-03 - Appliance and Network basic settings
Resource  ../../resources/Util.robot
Suite Setup  Setup Suite Environment
Suite Teardown  Cleanup Suite Environment

*** Variables ***
${dhcp}  True
${not_collect_log}  False
${esx_number}  1
${network}  False
${maximum_retry}  10

*** Keywords ***
Setup Suite Environment
    @{vms}=  Create A Simple VC Cluster  datacenter  cls  ${esx_number}  ${network}
    ${vm_num}=  Evaluate  ${esx_number}+1
    ${vm-names}=  Get Slice From List  ${vms}  0  ${vm_num}
    ${vm-ips}=  Get Slice From List  ${vms}  ${vm_num}
    Set Suite Variable  ${vc-ip}  @{vm-ips}[${esx_number}]
    Set Suite Variable  ${vm-names}  ${vm-names}
    Set Suite Variable  ${vm-ips}  ${vm-ips}
    ${size}=  Evaluate  50*1024*1024
    Set Suite Variable  ${ova_default_disk_size_inKB}  ${size}
    ${size}=  Evaluate  80*1024*1024
    Set Suite Variable  ${ova_adjusted_disk_size_inKB}  ${size}
    GOVC Use VC Environment 

Cleanup Suite Environment
    Nimbus Cleanup  ${vm-names}  ${not_collect_log}

Check VCH VM Disk Size 
    [Arguments]  ${target_size}
    ${size}=  Run  govc device.info -json -vm ${ova_target_vm_name} disk-1000-1 | jq .Devices[].CapacityInKB
    
    Should Be Equal As Integers  ${size}  ${target_size}

Resize VCH VM Disk
    [Arguments]  ${target_size}
    Run  govc vm.disk.change -vm=${ova_target_vm_name} -disk.name "disk-1000-1" -size=${target_size}KB

GOVC Use VC Environment
    Set Environment Variable  GOVC_URL  ${vc-ip}
    Set Environment Variable  GOVC_USERNAME  Administrator@vsphere.local
    Set Environment Variable  GOVC_PASSWORD  Admin\!23

GOVC Use ESXi Environment
    Set Environment Variable  GOVC_URL  @{vm-ips}[0]
    Set Environment Variable  GOVC_USERNAME  root
    Set Environment Variable  GOVC_PASSWORD  ${NIMBUS_ESX_PASSWORD}

*** Test Cases ***
Check VCH VM Network Settings
    [Setup]  Deploy VIC-OVA To Test Server
    ${ip}=  Run  govc vm.ip -n ethernet-0 -v4 ${ova_target_vm_name}
    Should Be Equal  ${ip}  ${ova_network_ip0}
    ${netmasks}=  Run  govc vm.info -json ${ova_target_vm_name} | jq -r '.VirtualMachines[].Guest.Net[] | select(.Network == "VM Network").IpConfig.IpAddress[].PrefixLength'
    @{netmasks}=  Split To Lines  ${netmasks}
    ${netmask}=  Convert To Integer  @{netmasks}[0]
    Should Be Equal As Integers  ${netmask}  24
    ${dhcp}=  Run  govc vm.info -json ${ova_target_vm_name} | jq -r '.VirtualMachines[].Guest.IpStack[].DnsConfig.Dhcp'
    Should Be Equal  ${dhcp}  false
    ${dns}=  Run  govc vm.info -json ${ova_target_vm_name} | jq -r '.VirtualMachines[].Guest.IpStack[].DnsConfig.IpAddress[]'
    Should Be Equal  ${dns}  ${ova_network_dns}
    ${search_domains}=  Run  govc vm.info -json ${ova_target_vm_name} | jq -r '.VirtualMachines[].Guest.IpStack[].DnsConfig.SearchDomain[]'
    @{search_domains}=  Split String  ${search_domains}  ,
    @{search_domains_setting}=  Split String  ${ova_network_searchpath}  ,
    ${length}=  Get Length  ${search_domains}
    Length Should Be  ${search_domains_setting}  ${length}
    :For  ${index}  IN RANGE  ${length}
    \   Should Be Equal  @{search_domains}[${index}]  @{search_domains_setting}[${index}]

Check VCH VM Disk Repartition
    Check VCH VM Disk Size  ${ova_default_disk_size_inKB}
    Resize VCH VM Disk  ${ova_adjusted_disk_size_inKB}
    Check VCH VM Disk Size  ${ova_adjusted_disk_size_inKB}
    [Teardown]  Cleanup VIC-OVA On Test Server

# If the first ipaddress for VM Network is an IPv4 ipaddress, then dhcp is assumed to be successful
Check VCH VM Network DHCP Settings
    [Setup]  Deploy VIC-OVA To Test Server  ${dhcp}
    ${ips}=  Run  govc vm.info -json ${ova_target_vm_name} | jq -r '.VirtualMachines[].Guest.Net[] | select(.Network == "VM Network").IpConfig.IpAddress[].IpAddress' 
    @{ips}=  Split To Lines  ${ips}
    ${ip_dot_number}=  Run  echo @{ips}[0] | awk '{print gsub(/\\\./, "")}'
    ${ip_dot_number}=  Convert To Integer  ${ip_dot_number}
    Should Be Equal As Integers  ${ip_dot_number}  3
    [Teardown]  Cleanup VIC-OVA On Test Server
