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
Documentation     Suite 25-01 - Basic
Resource          ../../resources/Util.robot
Resource          ../../resources/Group25-Host-Affinity-Util.robot
Test Setup        Set Test Environment Variables
Test Teardown     Cleanup
Default Tags


*** Test Cases ***
Creating a VCH creates a VM group and container VMs get added to it
    Verify Group Not Found       %{VCH-NAME}

    Install VIC Appliance To Test Server With Current Environment Variables    additional-args=--affinity-vm-group

    Verify Group Contains VMs    %{VCH-NAME}    1

    Create Three Containers

    Verify Group Contains VMs    %{VCH-NAME}    4


Deleting a VCH deletes its VM group
    [Teardown]    Run Keyword If Test Failed    Cleanup VIC Appliance On Test Server

    Verify Group Not Found       %{VCH-NAME}

    Install VIC Appliance To Test Server With Current Environment Variables    additional-args=--affinity-vm-group

    Verify Group Contains VMs    %{VCH-NAME}    1

    Run VIC Machine Delete Command

    Verify Group Not Found       %{VCH-NAME}


Deleting a container cleans up its VM group
    Verify Group Not Found       %{VCH-NAME}

    Install VIC Appliance To Test Server With Current Environment Variables    additional-args=--affinity-vm-group

    Create Three Containers

    Verify Group Contains VMs    %{VCH-NAME}    4

    Delete Containers

    Verify Group Contains VMs    %{VCH-NAME}    1


Create a VCH without a VM group
    Verify Group Not Found       %{VCH-NAME}

    Create Group                 %{VCH-NAME}

    Verify Group Empty           %{VCH-NAME}

    Install VIC Appliance To Test Server With Current Environment Variables    cleanup=${false}

    Verify Group Empty           %{VCH-NAME}

    Create Three Containers

    Verify Group Empty           %{VCH-NAME}


Attempt to create a VCH when a VM group with the same name already exists
    [Teardown]    Remove Group   %{VCH-NAME}

    Verify Group Not Found       %{VCH-NAME}

    Create Group                 %{VCH-NAME}

    Verify Group Empty           %{VCH-NAME}

    Run Keyword and Expect Error    *    Install VIC Appliance To Test Server With Current Environment Variables    additional-args=--affinity-vm-group    cleanup=${false}

    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network

    Verify Group Empty           %{VCH-NAME}


Deleting a VCH gracefully handles missing VM group
    [Teardown]    Run Keyword If Test Failed    Cleanup VIC Appliance On Test Server

    Verify Group Not Found       %{VCH-NAME}

    Install VIC Appliance To Test Server With Current Environment Variables    additional-args=--affinity-vm-group

    Verify Group Contains VMs    %{VCH-NAME}    1

    Remove Group                 %{VCH-NAME}

    Verify Group Not Found       %{VCH-NAME}

    Run VIC Machine Delete Command

    Run Keyword If  %{DRONE_BUILD_NUMBER} != 0  Run Keyword And Ignore Error  Cleanup VCH Bridge Network
