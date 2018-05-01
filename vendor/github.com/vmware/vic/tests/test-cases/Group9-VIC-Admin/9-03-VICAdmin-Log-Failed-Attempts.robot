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
Documentation  Test 9-03 - VICAdmin Log Failed Attempts
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server
Suite Teardown  Cleanup VIC Appliance On Test Server

*** Test Cases ***
Verify Unable To Verify
    ${out}=  Run  wget --tries=3 --connect-timeout=10 %{VIC-ADMIN}/logs/vicadmin.log -O failure.log
    Should Contain  ${out}  ERROR: cannot verify
    Should Contain  ${out}  certificate, issued by
    Should Contain  ${out}  Unable to locally verify the issuer's authority.
    
Verify Temporary Redirect
    ${out}=  Run  wget --tries=3 --connect-timeout=10 --no-check-certificate %{VIC-ADMIN}/logs/vicadmin.log -O failure.log
    Should Contain  ${out}  HTTP request sent, awaiting response... 303 See Other

Verify Failed Log Attempts
    ${status}=  Run Keyword And Return Status  Environment Variable Should Not Be Set  DOCKER_CERT_PATH
    Pass Execution If  ${status}  This test is only applicable if using TLS with certs

    #Save the first appliance certs and cleanup the first appliance
    #${old-certs}=  Set Variable  %{DOCKER_CERT_PATH}
    Run  cp -r %{DOCKER_CERT_PATH} old-certs
    Cleanup VIC Appliance On Test Server
    
    #Install a second appliance
    Install VIC Appliance To Test Server
    OperatingSystem.File Should Exist  old-certs/cert.pem
    OperatingSystem.File Should Exist  old-certs/key.pem
    ${out}=  Run  wget -v --tries=3 --connect-timeout=10 --certificate=old-certs/cert.pem --private-key=old-certs/key.pem --no-check-certificate %{VIC-ADMIN}/logs/vicadmin.log -O failure.log
    Log  ${out}
    ${out}=  Run  wget -v --tries=3 --connect-timeout=10 --certificate=%{DOCKER_CERT_PATH}/cert.pem --private-key=%{DOCKER_CERT_PATH}/key.pem --no-check-certificate %{VIC-ADMIN}/logs/vicadmin.log -O success.log
    Log  ${out}
    ${out}=  Run  cat success.log
    Log  ${out}
    ${out}=  Run  grep -i fail success.log
    Should Contain  ${out}  tls: failed to verify client's certificate: x509: certificate signed by unknown authority
    Run  rm -r old-certs
