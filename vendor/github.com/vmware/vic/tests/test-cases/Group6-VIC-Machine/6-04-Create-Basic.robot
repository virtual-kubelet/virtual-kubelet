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
Documentation  Test 6-04 - Verify vic-machine create basic use cases
Resource  ../../resources/Util.robot
Test Teardown  Run Keyword If Test Failed  Cleanup VIC Appliance On Test Server
Test Timeout  20 minutes

*** Keywords ***
Manually Create VCH Folder On VC
     # Grab the vm folder for the VC
     ${rc}  ${vm-folder-path}=  Run And Return Rc And Output  govc ls | grep vm
     Should Be Equal As Integers  ${rc}  0
     # Create vch named folder for
     ${rc}=  Run And Return Rc  govc folder.create ${vm-folder-path}/%{VCH-NAME}
     Should Be Equal As Integers  ${rc}  0

Create Dummy VM In VCH Folder On VC
    ${vm-folder-path}=  Run  govc ls | grep vm
    # grab path to the cluster
    ${rc}  ${compute-path}=  Run And Return Rc And Output  govc ls host | grep %{TEST_RESOURCE}
    Should Be Equal As Integers  ${rc}  0
    # Create dummy VM at the correct inventory path.
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.create -pool=${compute-path} -net=%{PUBLIC_NETWORK} -folder=${vm-folder-path}/%{VCH-NAME} %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

Create Dummy VM On ESX
    ${rc}  ${compute-path}=  Run And Return Rc And Output  govc ls host | grep %{TEST_RESOURCE}
    Should Be Equal As Integers  ${rc}  0
    # Create dummy VM at the correct inventory path.
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.create -pool=${compute-path} -net=%{PUBLIC_NETWORK} %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

Cleanup Manually Created VCH Folder
    ${rc}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run And Return Rc  govc object.destroy vm/%{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

Cleanup Dummy VM
    [Arguments]  ${vm-name}
    ${rc}  ${output}=  Run And Return Rc And Output  govc vm.destroy %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

Cleanup Dummy VM And VCH Folder
    Run Keyword And Continue On Failure  Cleanup Dummy VM  %{VCH-NAME}
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run Keyword And Continue On Failure  Cleanup Manually Created VCH Folder

*** Test Cases ***
Create VCH - supply DNS server
    Set Test Environment Variables
    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --image-store=%{TEST_DATASTORE} --password=%{TEST_PASSWORD} --no-tls --dns-server=1.1.1.1 --dns-server=2.2.2.2
    Should Contain  ${output}  Installer completed successfully
    ${output}=  Run  bin/vic-machine-linux debug --target=%{TEST_URL} --name=%{VCH-NAME} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} --enable-ssh --pw password --thumbprint=%{TEST_THUMBPRINT}
    Should Contain  ${output}  Completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}
    Open Connection  %{VCH-IP}
    Login  root  password
    ${out}=  Execute Command  cat /etc/resolv.conf
    Log  ${out}
    ${first}=  Get Line  ${out}  0
    Should Be Equal  ${first}  nameserver 1.1.1.1
    ${second}=  Get Line  ${out}  1
    Should Be Equal  ${second}  nameserver 2.2.2.2

    Cleanup VIC Appliance On Test Server

Create VCH - custom base disk
    Set Test Environment Variables
    # Attempt to cleanup old/canceled tests
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    # Deploy vic-machine with debug enabled to attempt to cache #7047
    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --debug=1 --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --image-store=%{TEST_DATASTORE} --password=%{TEST_PASSWORD} --base-image-size=6GB ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    ${output}=  Run  docker %{VCH-PARAMS} logs $(docker %{VCH-PARAMS} start $(docker %{VCH-PARAMS} create --name customDiskContainer ${busybox} /bin/df -h) && sleep 10) | grep /dev/sda | awk '{print $2}'
    # df shows GiB and vic-machine takes in GB so 6GB on cmd line == 5.5GB in df
    Should Be Equal As Strings  ${output}  5.5G
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f customDiskContainer
    Should Be Equal As Integers  ${rc}  0

    Run Regression Tests
    Cleanup VIC Appliance On Test Server

Create VCH - Folder Structure Correctness
    Install VIC Appliance To Test Server
    ${rc}  ${out}=  Run And Return Rc And Output  govc ls vm | grep %{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${out}  vm/%{VCH-NAME}
    Check VM Folder Path  %{VCH-NAME}
    Cleanup VIC Appliance On Test Server

Create VCH - URL without user and password
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls}
    Should Contain  ${output}  vSphere user must be specified

    # Delete the portgroup added by env vars keyword
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Cleanup VCH Bridge Network

Create VCH - target URL
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests
    Cleanup VIC Appliance On Test Server

Create VCH - operations user
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls} --ops-user=%{TEST_USERNAME} --ops-password=%{TEST_PASSWORD} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests
    Cleanup VIC Appliance On Test Server

Create VCH - specified datacenter
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Pass Execution  Requires vCenter environment

    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls} --compute-resource=%{TEST_DATACENTER} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests
    Cleanup VIC Appliance On Test Server

Create VCH - defaults
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --password=%{TEST_PASSWORD} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Contain  ${output}  Installer completed successfully
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Get Docker Params  ${output}  ${true}
    ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --password=%{TEST_PASSWORD} ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Contain  ${output}  Installer completed successfully
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests
    Cleanup VIC Appliance On Test Server

Create VCH - full params
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests
    Cleanup VIC Appliance On Test Server

Create VCH - using environment variables
    Set Test Environment Variables
    Set Environment Variable  VIC_MACHINE_TARGET  %{TEST_URL}
    Set Environment Variable  VIC_MACHINE_USER  %{TEST_USERNAME}
    Set Environment Variable  VIC_MACHINE_PASSWORD  %{TEST_PASSWORD}
    Set Environment Variable  VIC_MACHINE_THUMBPRINT  %{TEST_THUMBPRINT}
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --image-store=%{TEST_DATASTORE} --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} --volume-store=%{TEST_DATASTORE}/%{VCH-NAME}-VOL:default ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    Run Regression Tests
    Cleanup VIC Appliance On Test Server
    Remove Environment Variable  VIC_MACHINE_TARGET
    Remove Environment Variable  VIC_MACHINE_USER
    Remove Environment Variable  VIC_MACHINE_PASSWORD
    Remove Environment Variable  VIC_MACHINE_THUMBPRINT

Create VCH - custom image store directory
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store %{TEST_DATASTORE}/vic-machine-test-images --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com

    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}
    ${output}=  Run  GOVC_DATASTORE=%{TEST_DATASTORE} govc datastore.ls
    Should Contain  ${output}  vic-machine-test-images

    Run Regression Tests
    Cleanup VIC Appliance On Test Server
    ${output}=  Run  GOVC_DATASTORE=%{TEST_DATASTORE} govc datastore.ls
    Should Not Contain  ${output}  vic-machine-test-images

Create VCH - long VCH name
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME}-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls}
    Should Contain  ${output}  exceeds the permitted 31 characters limit

    # Delete the portgroup added by env vars keyword
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Cleanup VCH Bridge Network

Create VCH - Existing VCH Name
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} ${vicmachinetls}
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}
    Log To Console  Installer completed successfully: %{VCH-NAME}

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} ${vicmachinetls}
    ${vm-folder-path}=  Run  govc ls | grep vm
    Should Contain  ${output}  \\"%{VCH-NAME}\\" already exists

    Cleanup VIC Appliance On Test Server

Create VCH - Existing VM Name
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    # setup environment
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Manually Create VCH Folder On VC
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Create Dummy VM In VCH Folder On VC
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Create Dummy VM On ESX

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls}
    Log  ${output}

    # VCH creation should succeed on ESXi
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Get Docker Params  ${output}  ${true}
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Should Contain  ${output}  Installer completed successfully
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Log To Console  Installer completed successfully: %{VCH-NAME}
    Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Run Keyword And Ignore Error  Cleanup VIC Appliance On Test Server

    # VCH creation should fail on VC
    ${vm-folder-path}=  Run  govc ls | grep vm
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Contain  ${output}  already in use
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Should Contain  ${output}  %{VCH-NAME}
    Should Not Be Equal As Integers  ${rc}  0

    [teardown]  Cleanup Dummy VM And VCH Folder

Create VCH - Folder Conflict
    # This case cannot occur on standalone ESXi's
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server
    Pass Execution If  '%{HOST_TYPE}' == 'ESXi'  ESXi does not support folders, skipping test.


    # setup environment
    Manually Create VCH Folder On VC

    ${rc}  ${output}=  Run And Return Rc And Output  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls}
    Log  ${output}
    ${vm-path}=  Run  govc ls | grep vm
    Should Contain  ${output}  already in use
    Should Contain  ${output}  %{VCH-NAME}
    Should Not Be Equal As Integers  ${rc}  0

    [teardown]  Cleanup Manually Created VCH Folder

Create VCH - Existing RP on ESX
    Run Keyword If  '%{HOST_TYPE}' == 'VC'  Pass Execution  Test skipped on VC

    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    # Create dummy RP
    ${rc}  ${output}=  Run And Return Rc And Output  govc pool.create %{TEST_RESOURCE}/%{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} ${vicmachinetls} --compute-resource=%{TEST_RESOURCE}
    Should Contain  ${output}  Installer completed successfully
    Log  Installer completed successfully: %{VCH-NAME}

    Cleanup VIC Appliance On Test Server

    ${rc}  ${output}=  Run And Return Rc And Output  govc pool.destroy %{TEST_RESOURCE}/%{VCH-NAME}
    Should Be Equal As Integers  ${rc}  0

Creation log file uploaded to datastore

    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --image-store=%{TEST_DATASTORE} --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} ${vicmachinetls} --insecure-registry wdc-harbor-ci.eng.vmware.com
    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}

    ${filename}=  Run  GOVC_DATASTORE=%{TEST_DATASTORE} govc datastore.ls %{VCH-NAME} | grep vic-machine_
    Should Not Be Empty  ${filename}
    ${output}=  Run  govc datastore.tail -n 1 "%{VCH-NAME}/${filename}"
    Should Contain  ${output}  Installer completed successfully

    Cleanup VIC Appliance On Test Server

Basic timeout
    Set Test Environment Variables
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${output}=  Run  bin/vic-machine-linux create --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --timeout 1s ${vicmachinetls}
    Should Contain  ${output}  Creating VCH exceeded time limit

    ${ret}=  Run  bin/vic-machine-linux delete --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --compute-resource=%{TEST_RESOURCE} --name %{VCH-NAME}
    Should Contain  ${ret}  Completed successfully
    ${out}=  Run  govc ls vm
    Should Not Contain  ${out}  %{VCH-NAME}
    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network

Basic VCH resource config
    Pass execution  Test not implemented

Invalid VCH resource config
    Pass execution  Test not implemented

CPU reservation shares invalid
    Pass execution  Test not implemented

CPU reservation invalid
    Pass execution  Test not implemented

CPU reservation valid
    Pass execution  Test not implemented

Memory reservation shares invalid
    Pass execution  Test not implemented

Memory reservation invalid 1
    Pass execution  Test not implemented

Memory reservation invalid 2
    Pass execution  Test not implemented

Memory reservation invalid 3
    Pass execution  Test not implemented

Memory reservation valid
    Pass execution  Test not implemented

Extension installation
    Pass execution  Test not implemented

Install existing extension
    Pass execution  Test not implemented
