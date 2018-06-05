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
Documentation  Test 16-1 - Windows VIC Machine
Resource  ../../resources/Util.robot
Test Setup  Set Test Environment Variables

*** Variables ***
${ver}  8351

*** Keywords ***
Cleanup Folders
    ${output}=  Execute Command  rm -Recurse -Force vic*
    ${output}=  Execute Command  rm *.pem

*** Test Cases ***
Install VCH With TLS
    Open Connection  %{WINDOWS_URL}  prompt=>
    Login  %{WINDOWS_USERNAME}  %{WINDOWS_PASSWORD}
    Cleanup Folders
    ${output}=  Execute Command  wget https://storage.googleapis.com/vic-engine-builds/vic_${ver}.tar.gz -OutFile vic.tar.gz
    ${output}=  Execute Command  7z x vic.tar.gz
    ${output}=  Execute Command  7z x vic.tar
    ${output}=  Execute Command  ./vic/vic-machine-windows.exe create --target %{TEST_URL} --user %{TEST_USERNAME} --password %{TEST_PASSWORD}
    Get Docker Params  ${output}  ${true}
    Run Regression Tests
    ${output}=  Execute Command  ./vic/vic-machine-windows.exe delete --target %{TEST_URL} --user %{TEST_USERNAME} --password %{TEST_PASSWORD}    
    Cleanup Folders
    
Install VCH Without TLS
    Open Connection  %{WINDOWS_URL}  prompt=>
    Login  %{WINDOWS_USERNAME}  %{WINDOWS_PASSWORD}
    Cleanup Folders
    ${output}=  Execute Command  wget https://storage.googleapis.com/vic-engine-builds/vic_${ver}.tar.gz -OutFile vic.tar.gz
    ${output}=  Execute Command  7z x vic.tar.gz
    ${output}=  Execute Command  7z x vic.tar
    ${output}=  Execute Command  ./vic/vic-machine-windows.exe create --target %{TEST_URL} --user %{TEST_USERNAME} --password %{TEST_PASSWORD} --no-tls
    Get Docker Params  ${output}  ${false}
    Run Regression Tests
    ${output}=  Execute Command  ./vic/vic-machine-windows.exe delete --target %{TEST_URL} --user %{TEST_USERNAME} --password %{TEST_PASSWORD}    
    Cleanup Folders