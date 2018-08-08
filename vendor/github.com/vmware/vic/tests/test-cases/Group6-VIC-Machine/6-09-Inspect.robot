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
# limitations under the License.

*** Settings ***
Documentation  Test 6-09 - Verify vic-machine inspect functions
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If Test Failed  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Cleanup Container Network Test Networks
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.remove published-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.remove peers-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Remove VC Distributed Portgroup  published-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Remove VC Distributed Portgroup  peers-net

Cleanup Container Network Test
    Cleanup VIC Appliance On Test Server
    Cleanup Container Network Test Networks

*** Test Cases ***
Inspect VCH Basic
    Install VIC Appliance To Test Server

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE}
    Should Be Equal As Integers  0  ${rc}
    Should Contain  ${output}  VCH ID
    Should Contain  ${output}  Installer version
    Should Contain  ${output}  VCH version
    Should Contain  ${output}  VCH upgrade status
    Should Contain  ${output}  Installer has same version as VCH
    Should Contain  ${output}  No upgrade available with this installer version
    Should Contain  ${output}  VCH Admin Portal
    Should Contain  ${output}  Published ports can be reached at
    Should Contain  ${output}  Connect to docker

Inspect VCH Configuration
    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect config --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  --debug=1
    Should Contain  ${output}  --name=%{VCH-NAME}
    Should Contain  ${output}  --target=https://%{TEST_URL}
    Should Contain  ${output}  --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --image-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --bridge-network=%{BRIDGE_NETWORK}
    Should Not Contain  ${output}  --cpu
    Should Not Contain  ${output}  --cpu-shares
    Should Not Contain  ${output}  --memory
    Should Not Contain  ${output}  --memory-shares
    Should Not Contain  ${output}  --base-image-size
    Should Not Contain  ${output}  --bridge-network-range
    Should Be Equal As Integers  0  ${rc}

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect config --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE} --format raw
    Should Contain  ${output}  --debug=1
    Should Contain  ${output}  --name=%{VCH-NAME}
    Should Contain  ${output}  --target=https://%{TEST_URL}
    Should Contain  ${output}  --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --image-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --bridge-network=%{BRIDGE_NETWORK}
    Should Not Contain  ${output}  --cpu
    Should Not Contain  ${output}  --cpu-shares
    Should Not Contain  ${output}  --memory
    Should Not Contain  ${output}  --memory-shares
    Should Not Contain  ${output}  --base-image-size
    Should Not Contain  ${output}  --bridge-network-range
    Should Not Contain  ${output}  INFO
    Should Not Contain  ${output}  WARN
    Should Be Equal As Integers  0  ${rc}

    Cleanup VIC Appliance On Test Server

Inspect VCH Configuration with Resource Limitation
    Install VIC Appliance To Test Server  additional-args=--memory 8000 --memory-reservation 512 --memory-shares 6000 --cpu 10000 --cpu-reservation 512 --cpu-shares high --endpoint-cpu 2 --endpoint-memory 4096

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect config --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  --debug=1
    Should Contain  ${output}  --name=%{VCH-NAME}
    Should Contain  ${output}  --target=https://%{TEST_URL}
    Should Contain  ${output}  --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --image-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --bridge-network=%{BRIDGE_NETWORK}
    Should Contain  ${output}  --memory-shares=6000
    Should Contain  ${output}  --memory-reservation=512
    Should Contain  ${output}  --memory=8000
    Should Contain  ${output}  --cpu=10000
    Should Contain  ${output}  --cpu-reservation=512
    Should Contain  ${output}  --cpu-shares=high
    Should Contain  ${output}  --endpoint-memory=4096
    Should Contain  ${output}  --endpoint-cpu=2
    Should Be Equal As Integers  0  ${rc}

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect config --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE} --format raw

    Should Contain  ${output}  --debug=1
    Should Contain  ${output}  --name=%{VCH-NAME}
    Should Contain  ${output}  --target=https://%{TEST_URL}
    Should Contain  ${output}  --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  --image-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  --volume-store=ds://%{TEST_DATASTORE}
    Should Contain  ${output}  --bridge-network=%{BRIDGE_NETWORK}
    Should Contain  ${output}  --memory-shares=6000
    Should Contain  ${output}  --memory-reservation=512
    Should Contain  ${output}  --memory=8000
    Should Contain  ${output}  --cpu=10000
    Should Contain  ${output}  --cpu-reservation=512
    Should Contain  ${output}  --cpu-shares=high
    Should Contain  ${output}  --endpoint-memory=4096
    Should Contain  ${output}  --endpoint-cpu=2
    Should Not Contain  ${output}  INFO
    Should Not Contain  ${output}  WARN
    Should Be Equal As Integers  0  ${rc}

    Cleanup VIC Appliance On Test Server

Inspect VCH Configuration with Container Networks
    # Set the only teardown for this test to cleanup both portgroups and VCH, regardless of test outcome.
    [Teardown]  Cleanup Container Network Test

    Cleanup Container Network Test Networks

    ${vswitch}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.vswitch.info -json | jq -r ".Vswitch[0].Name"
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.add -vswitch ${vswitch} published-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  govc host.portgroup.add -vswitch ${vswitch} peers-net
    ${dvs}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  govc find -type DistributedVirtualSwitch | head -n1
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Add VC Distributed Portgroup  ${dvs}  published-net
    ${out}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Add VC Distributed Portgroup  ${dvs}  peers-net

    Install VIC Appliance To Test Server  additional-args=-container-network published-net -container-network peers-net -cnf peers-net:peers --container-network-ip-range peers-net:10.10.10.0/24 -cng peers-net:10.10.10.1/24

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux inspect config --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user %{TEST_USERNAME} --password=%{TEST_PASSWORD} --name=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE} --format raw

    Should Contain  ${output}  --container-network=published-net:published-net
    Should Not Contain  ${output}  --container-network-firewall=published-net:published
    Should Contain  ${output}  --container-network=peers-net:peers-net
    Should Contain  ${output}  --container-network-gateway=peers-net:10.10.10.1/24
    Should Contain  ${output}  --container-network-ip-range=peers-net:10.10.10.0/24
    Should Contain  ${output}  --container-network-firewall=peers-net:peers
    Should Be Equal As Integers  0  ${rc}

Verify inspect output for a full tls VCH
    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  '${domain}' == ''  Pass Execution  Skipping test - domain not set, won't generate keys

    Install VIC Appliance To Test Server

    ${output}=  Run  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  DOCKER_CERT_PATH=${EXECDIR}/%{VCH-NAME}

    ${output}=  Run  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --tls-cert-path=%{VCH-NAME} --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  DOCKER_CERT_PATH=${EXECDIR}/%{VCH-NAME}

    ${output}=  Run  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --tls-cert-path=fakeDir --compute-resource=%{TEST_RESOURCE}
    Should Not Contain  ${output}  DOCKER_CERT_PATH=${EXECDIR}/%{VCH-NAME}
    Should Contain  ${output}  Unable to find valid client certs
    Should Contain  ${output}  DOCKER_CERT_PATH must be provided in environment or certificates specified individually via CLI arguments

    Cleanup VIC Appliance On Test Server

Verify inspect output for a --no-tls VCH
    Set Test Environment Variables
    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --no-tls
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}

    ${output}=  Run  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --compute-resource=%{TEST_RESOURCE}
    Should Not Contain  ${output}  DOCKER_CERT_PATH=${EXECDIR}/%{VCH-NAME}
    Should Not Contain  ${output}  Unable to find valid client certs
    Should Not Contain  ${output}  DOCKER_CERT_PATH must be provided in environment or certificates specified individually via CLI arguments

    Cleanup VIC Appliance On Test Server

Verify inspect output for a --no-tlsverify VCH
    Set Test Environment Variables
    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --no-tlsverify
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}

    ${output}=  Run  bin/vic-machine-linux inspect --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --compute-resource=%{TEST_RESOURCE}
    Should Not Contain  ${output}  DOCKER_CERT_PATH=${EXECDIR}/%{VCH-NAME}
    Should Not Contain  ${output}  Unable to find valid client certs
    Should Not Contain  ${output}  DOCKER_CERT_PATH must be provided in environment or certificates specified individually via CLI arguments

    Cleanup VIC Appliance On Test Server
