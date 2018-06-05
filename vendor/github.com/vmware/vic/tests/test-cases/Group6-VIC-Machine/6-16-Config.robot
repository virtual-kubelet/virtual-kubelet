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
Documentation  Test 6-16 - Verify vic-machine configure
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Test Cases ***
Configure VCH debug state
    ${output}=  Run  bin/vic-machine-linux configure --help
    Should Contain  ${output}  --debug
    ${output}=  Check VM Guestinfo  %{VCH-NAME}  guestinfo.vice./init/diagnostics/debug
    Should Contain  ${output}  1
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${id1}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${vm1}=  Get VM display name  ${id1}
    ${output}=  Check VM Guestinfo  ${vm1}  guestinfo.vice./diagnostics/debug
    Should Contain  ${output}  1
    ${output}=  Run  bin/vic-machine-linux configure --debug 0 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT}
    Should Contain  ${output}  Completed successfully
    ${output}=  Check VM Guestinfo  %{VCH-NAME}  guestinfo.vice./init/diagnostics/debug
    Should Contain  ${output}  0
    ${output}=  Check VM Guestinfo  ${vm1}  guestinfo.vice./diagnostics/debug
    Should Contain  ${output}  1
    ${rc}  ${id2}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -itd ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${vm2}=  Get VM display name  ${id2}
    ${output}=  Check VM Guestinfo  ${vm2}  guestinfo.vice./diagnostics/debug
    Should Contain  ${output}  0
    ${output}=  Run  bin/vic-machine-linux configure --debug 1 --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT}
    Should Contain  ${output}  Completed successfully
    ${rc}  ${output}=  Run And Return Rc And Output  govc snapshot.tree -vm %{VCH-NAME} | grep reconfigure
    Should Be Equal As Integers  ${rc}  0
    ${output}=  Split To Lines  ${output}
    Length Should Be  ${output}  1
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT}
    Should Be Equal As Integers  0  ${rc}
    Should Contain  ${output}  --debug=1

Configure VCH Container Networks
    ${out}=  Run  govc host.portgroup.remove vm-network
    ${out}=  Run  govc host.portgroup.add -vswitch vSwitchLAN vm-network

    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network vm-network:vmnet
    Should Contain  ${output}  Completed successfully

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  vmnet

    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --container-network=vm-network:vmnet

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --net=vmnet ${busybox}
    Should Be Equal As Integers  ${rc}  0

    # Test that configure fails if an existing container-network is not specified
    ${out}=  Run  govc host.portgroup.remove management
    ${out}=  Run  govc host.portgroup.add -vswitch vSwitchLAN management
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network management:mgmt --container-network-ip-range=management:10.10.10.0/24 --container-network-gateway=management:10.10.10.1/24
    Should Contain  ${output}  all existing container networks must also be specified
    Should Not Contain  ${output}  Completed successfully

    # Add another container network while specifying the existing one
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network vm-network:vmnet --container-network management:mgmt --container-network-ip-range=management:10.10.10.0/24 --container-network-gateway=management:10.10.10.1/24 --container-network-firewall=management:open
    Should Contain  ${output}  Completed successfully

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} network ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  vmnet
    Should Contain  ${output}  mgmt

    ${stripped}=  Remove String  %{PUBLIC_NETWORK}  '
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --container-network=${stripped}:public
    Should Contain  ${output}  --container-network=vm-network:vmnet
    Should Contain  ${output}  --container-network=management:mgmt
    Should Contain  ${output}  --container-network-ip-range=management:10.10.10.0/24
    Should Contain  ${output}  --container-network-gateway=management:10.10.10.1/24
    Should Contain  ${output}  --container-network-firewall=management:open

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run -dit --net=mgmt ${busybox}
    Should Be Equal As Integers  ${rc}  0

    # Test that changes to existing networks are not supported
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network vm-network:vmnet --container-network management:mgmt --container-network-ip-range=management:10.10.10.0/24 --container-network-gateway=management:10.10.10.2/24
    Should Contain  ${output}  changes to existing container networks are not supported
    Should Not Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network vm-network:vmnet --container-network management:mgmt --container-network-ip-range=management:10.10.10.0/16 --container-network-gateway=management:10.10.10.1/24
    Should Contain  ${output}  changes to existing container networks are not supported
    Should Not Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network vm-network:vmnet --container-network management:mgmt
    Should Contain  ${output}  changes to existing container networks are not supported
    Should Not Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --container-network=%{PUBLIC_NETWORK}:public --container-network vm-network:vmnet --container-network management:mgmt --container-network-firewall=management:closed
    Should Contain  ${output}  changes to existing container networks are not supported
    Should Not Contain  ${output}  Completed successfully


    # Clean up portgroups
    ${out}=  Run  govc host.portgroup.remove vm-network
    ${out}=  Run  govc host.portgroup.remove management

Configure VCH https-proxy
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --http-proxy http://proxy.vmware.com:3128
    Should Contain  ${output}  Completed successfully
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep HTTP_PROXY
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  http://proxy.vmware.com:3128
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep HTTPS_PROXY
    Should Be Equal As Integers  ${rc}  1
    Should Not Contain  ${output}  proxy.vmware.com:3128
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --http-proxy=http://proxy.vmware.com:3128
    Should Not Contain  ${output}  --https-proxy

    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --https-proxy https://proxy.vmware.com:3128
    Should Contain  ${output}  Completed successfully
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep HTTPS_PROXY
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  https://proxy.vmware.com:3128
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep HTTP_PROXY
    Should Be Equal As Integers  ${rc}  1
    Should Not Contain  ${output}  proxy.vmware.com:3128
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --https-proxy=https://proxy.vmware.com:3128
    Should Not Contain  ${output}  --http-proxy

Configure VCH ops user credentials and thumbprint
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --ops-user=%{TEST_USERNAME} --ops-password=%{TEST_PASSWORD}
    Should Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --ops-user=%{TEST_USERNAME}
    Should Contain  ${output}  --thumbprint=%{TEST_THUMBPRINT}

Configure VCH https-proxy through vch id
    ${vch-id}=  Get VCH ID  %{VCH-NAME}
    ${output}=  Run  bin/vic-machine-linux configure --id=${vch-id} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --https-proxy ""
    Should Contain  ${output}  Completed successfully
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep HTTPS_PROXY
    Should Be Equal As Integers  ${rc}  1
    Should Not Contain  ${output}  proxy.vmware.com:3128

Configure VCH DNS server
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT}
    Should Not Contain  ${output}  --dns-server
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --dns-server 10.118.81.1 --dns-server 10.118.81.2
    Should Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT}
    Should Contain  ${output}  --dns-server=10.118.81.1
    Should Contain  ${output}  --dns-server=10.118.81.2
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep dns
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  network/dns
    Should Not Contain  ${output}  assigned.dns
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --dns-server ""
    Should Contain  ${output}  Completed successfully
    Should Not Contain  ${output}  --dns-server
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.info -e %{VCH-NAME} | grep dns
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  assigned.dns
    Should Not Contain  ${output}  network/dns

Configure VCH resources
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --cpu 5129 --cpu-reservation 10 --cpu-shares 8000 --memory 4096 --memory-reservation 10 --memory-shares 163840
    Should Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --cpu=5129
    Should Contain  ${output}  --cpu-reservation=10
    Should Contain  ${output}  --cpu-shares=8000
    Should Contain  ${output}  --memory=4096
    Should Contain  ${output}  --memory-reservation=10
    Should Contain  ${output}  --memory-shares=163840

    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL}%{TEST_DATACENTER} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --cpu 1 --cpu-shares 1000 --memory 1 --memory-shares 1000
    Should Not Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --cpu=5129
    Should Contain  ${output}  --cpu-reservation=10
    Should Contain  ${output}  --cpu-shares=8000
    Should Contain  ${output}  --memory=4096
    Should Contain  ${output}  --memory-reservation=10
    Should Contain  ${output}  --memory-shares=163840

Configure VCH volume stores
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-conf:configure
    Should Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}/%{VCH-NAME}-conf:configure

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} info
    Should Be Equal As Integers  ${rc}  0
    ${volstores}=  Get Lines Containing String  ${output}  VolumeStores:
    Should Contain  ${volstores}  default
    Should Contain  ${volstores}  configure
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create defaultVol
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume create confVol --opt VolumeStore=configure
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} volume ls
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  defaultVol
    Should Contain  ${output}  confVol

    # Test that configure fails if an existing volume store is not specified
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-conf:configure
    Should Contain  ${output}  all existing volume stores must also be specified
    Should Not Contain  ${output}  Completed successfully

    # Test that changes to existing volume stores are not supported
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-badpath:configure
    Should Contain  ${output}  changes to existing volume stores are not supported
    Should Not Contain  ${output}  Completed successfully

    # Add a new volume store while specifying the URL scheme
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-conf:configure --volume-store=ds://%{TEST_DATASTORE}/%{VCH-NAME}-scheme:scheme
    Should Contain  ${output}  Completed successfully
    ${output}=  Run  bin/vic-machine-linux inspect config --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}/%{VCH-NAME}-conf:configure
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}/%{VCH-NAME}-scheme:scheme

Configure Present in vic-machine
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux
    Should Contain  ${output}  configure
