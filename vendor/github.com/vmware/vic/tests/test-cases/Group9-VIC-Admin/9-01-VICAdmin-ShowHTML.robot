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
Documentation  Test 9-01 - VICAdmin ShowHTML
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  certs=${false}
Suite Teardown  Cleanup VIC Appliance On Test Server
Default Tags

*** Test Cases ***
Get Login Page
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/authentication
    Should contain  ${output}  <title>VCH Admin</title>

Check that HTTP Request Redirects to HTTPS
    ${rc}  ${output}=  Run And Return Rc And Output  curl -L -sk -vvv http://%{VCH-IP}:2378
    Should Contain  ${output}  SSL connection using TLS
    Should Contain  ${output}  301 Moved Permanently
    Should not contain  ${output}  Empty reply from server

While Logged Out Fail To Display HTML
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}
    Should not contain  ${output}  <title>VIC: %{VCH-NAME}</title>
    Should Contain  ${output}  <a href="/authentication">See Other</a>.

While Logged Out Fail To Get Portlayer Log
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/port-layer.log
    Should Not Contain  ${output}  Launching portlayer server
    Should Contain  ${output}  <a href="/authentication">See Other</a>.

While Logged Out Fail To Get VCH-Init Log
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/init.log
    Should not contain  ${output}  reaping child processes
    Should Contain  ${output}  <a href="/authentication">See Other</a>.

While Logged Out Fail To Get Docker Personality Log
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/docker-personality.log
    Should not contain  ${output}  docker personality
    Should Contain  ${output}  <a href="/authentication">See Other</a>.

While Logged Out Fail To Get Container Logs
    ${rc}  ${output}=  Run And Return Rc and Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${container}=  Run And Return Rc and Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${container}  Error
    ${rc}  ${output}=  Run And Return Rc and Output  docker %{VCH-PARAMS} start ${container}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${output}=  Run And Return Rc and Output  curl -sk %{VIC-ADMIN}/container-logs.tar.gz | tar tvzf -
    Should not Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  gzip: stdin: not in gzip format
    Log  ${output}
    Should not Contain  ${output}  ${container}/vmware.log
    Should not Contain  ${output}  ${container}/tether.debug

While Logged Out Fail To Get VICAdmin Log
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/vicadmin.log
    Log  ${output}
    Should not contain  ${output}  Launching vicadmin pprof server
    Should Contain  ${output}  <a href="/authentication">See Other</a>.

Display HTML
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN} -b ${cookies}
    Should contain  ${output}  <title>VIC: %{VCH-NAME}</title>

Get Portlayer Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/port-layer.log -b ${cookies}
    Should contain  ${output}  Launching portlayer server

Get VCH-Init Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/init.log -b ${cookies}
    Should contain  ${output}  reaping child processes

Get Docker Personality Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/docker-personality.log -b ${cookies}
    Should contain  ${output}  docker personality

Get Container Logs
    ${rc}  ${output}=  Run And Return Rc and Output  docker %{VCH-PARAMS} pull ${busybox}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${rc}  ${container}=  Run And Return Rc and Output  docker %{VCH-PARAMS} create ${busybox} /bin/top
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${container}  Error
    ${rc}  ${output}=  Run And Return Rc and Output  docker %{VCH-PARAMS} start ${container}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  Error
    ${vmName}=  Get VM Display Name  ${container}

    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc and Output  curl -sk %{VIC-ADMIN}/container-logs.tar.gz -b ${cookies} | (cd /tmp; tar xvzf - ${vmName}/tether.debug ${vmName}/vmware.log)
    Log  ${output}
    ${rc}  ${output}=  Run And Return Rc and Output  ls -l /tmp/${vmName}/vmware.log
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc and Output  ls -l /tmp/${vmName}/tether.debug
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc and Output  grep 'prepping for switch to container filesystem' /tmp/${vmName}/tether.debug
    Should Be Equal As Integers  ${rc}  0
    Run  rm -f /tmp/${vmName}/tether.debug /tmp/${vmName}/vmware.log

Get VICAdmin Log
    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN}/logs/vicadmin.log -b ${cookies}
    Log  ${output}
    Should contain  ${output}  Launching vicadmin pprof server

Check that VIC logs do not contain sensitive data
    Scrape Logs For The Password

Wan Routes Through Proxy
    Cleanup VIC Appliance On Test Server
    Install VIC Appliance To Test Server  certs=${false}  additional-args=--http-proxy=http://0.0.0.0:12345

    ${cookies}=  Login To VCH Admin And Save Cookies
    ${rc}  ${output}=  Run And Return Rc And Output  curl -sk %{VIC-ADMIN} -b ${cookies}
    Should contain  ${output}  <div class="sixty">Registry and Internet Connectivity<span class="error-message">
