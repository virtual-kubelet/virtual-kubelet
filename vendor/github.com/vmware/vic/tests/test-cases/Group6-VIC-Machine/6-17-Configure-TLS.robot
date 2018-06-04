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
Documentation  Test 6-17 - Verify vic-machine configure TLS options
Resource  ../../resources/Util.robot
Suite Teardown  Run Keyword  Cleanup VIC Appliance On Test Server
Suite Setup  Run Keyword  Setup Test Environment
Test Teardown  Run Keyword  Test Cleanup
Test Timeout  20 minutes

*** Keywords ***
Setup Test Environment
    Set Test Environment Variables
    Run Keyword And Ignore Error  Test Cleanup
    Run Keyword And Ignore Error  Cleanup Dangling VMs On Test Server
    Run Keyword And Ignore Error  Cleanup Datastore On Test Server

    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  '${domain}' == ''  Pass Execution  Skipping test - domain not set, won't generate keys

    ${output}=  Run  bin/vic-machine-linux create ${vicmachinetls} --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --image-store=%{TEST_DATASTORE} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --tls-cert-path=${EXECDIR}/foo-bar-certs/
    Should Contain  ${output}  --tlscacert=\\"${EXECDIR}/foo-bar-certs/ca.pem\\" --tlscert=\\"${EXECDIR}/foo-bar-certs/cert.pem\\" --tlskey=\\"${EXECDIR}/foo-bar-certs/key.pem\\"
    Should Contain  ${output}  Generating CA certificate/key pair - private key in ${EXECDIR}/foo-bar-certs/ca-key.pem
    Should Contain  ${output}  Generating server certificate/key pair - private key in ${EXECDIR}/foo-bar-certs/server-key.pem
    Should Contain  ${output}  Generating client certificate/key pair - private key in ${EXECDIR}/foo-bar-certs/key.pem
    Should Contain  ${output}  Generated browser friendly PFX client certificate - certificate in ${EXECDIR}/foo-bar-certs/cert.pfx

    Should Contain  ${output}  Installer completed successfully
    Get Docker Params  ${output}  ${true}

    ${save_env}=  Run  cat ${EXECDIR}/foo-bar-certs/%{VCH-NAME}.env
    Should Contain  ${save_env}  DOCKER_CERT_PATH=${EXECDIR}/foo-bar-certs
    Log To Console  Installer completed successfully: %{VCH-NAME}

Test Cleanup
    Run  rm -rf bundle cert-bundle.tgz out-bundle /root/ca ${EXECDIR}/foo-bar-certs

*** Test Cases ***
Configure VCH - Server cert with untrusted CA
    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  '${domain}' == ''  Pass Execution  Skipping test - domain not set, won't generate keys
    # Generate CA and wildcard cert for *.<DOMAIN>
    Generate Certificate Authority
    Generate Wildcard Server Certificate

    ${out}=  Run  cp /root/ca/cert-bundle.tgz .; tar xvf cert-bundle.tgz
    Log  ${out}

    # Run vic-machine configure, supply server cert and key
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --tls-server-key "bundle/*.${domain}.key.pem" --tls-server-cert "bundle/*.${domain}.cert.pem" ${vicmachinetls} --tls-cert-path "out-bundle" --debug 1
    Log  ${output}
    Should Contain  ${output}  Completed successfully

    # Verify that the supplied certificate is presented on web interface
    ${output}=  Run  openssl s_client -showcerts -connect %{VCH-IP}:2378
    Log  ${output}
    Should Contain  ${output}  issuer=/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Certificate Authority/CN=Stark Enterprises Global CA

Configure VCH - Server cert with trusted CA
    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  '${domain}' == ''  Pass Execution  Skipping test - domain not set, won't generate keys

    # Generate CA and wildcard cert for *.<DOMAIN>, install CA into root store
    Generate Certificate Authority
    Generate Wildcard Server Certificate
    Trust Certificate Authority

    ${out}=  Run  cp /root/ca/cert-bundle.tgz .; tar xvf cert-bundle.tgz
    Log  ${out}

    # Run vic-machine install, supply server cert and key
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --tls-server-key "bundle/*.%{DOMAIN}.key.pem" --tls-server-cert "bundle/*.%{DOMAIN}.cert.pem" ${vicmachinetls} --debug 1
    Log  ${output}
    Should Contain  ${output}  Loaded server certificate bundle
    Should Contain  ${output}  Unable to locate existing CA in cert path
    Should Contain  ${output}  Completed successfully


    ${output}=  Run  openssl s_client -showcerts -connect %{VCH-IP}:2378
    Log  ${output}
    Should Contain  ${output}  issuer=/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Certificate Authority/CN=Stark Enterprises Global CA

    Reload Default Certificate Authorities

Configure VCH - Run Configure Without Cert Options & Ensure Certs Are Unchanged
    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  '${domain}' == ''  Pass Execution  Skipping test - domain not set, won't generate keys

    # Generate CA and wildcard cert for *.<DOMAIN>, install CA into root store
    Generate Certificate Authority
    Generate Wildcard Server Certificate
    Trust Certificate Authority

    ${out}=  Run  cp /root/ca/cert-bundle.tgz .; tar xvf cert-bundle.tgz
    Log  ${out}

    Run  rm -rf foo-bar-certs
    # Run vic-machine configure, supply server cert and key
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} --tls-server-key "bundle/*.%{DOMAIN}.key.pem" --tls-server-cert "bundle/*.%{DOMAIN}.cert.pem" ${vicmachinetls} --tls-cert-path=foo-bar-certs --debug 1
    Log  ${output}
    Should Contain  ${output}  Loaded server certificate bundle
    Should Contain  ${output}  Unable to locate existing CA in cert path
    Should Contain  ${output}  Completed successfully


    ${output}=  Run  openssl s_client -showcerts -connect %{VCH-IP}:2378
    Log  ${output}
    Should Contain  ${output}  issuer=/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Certificate Authority/CN=Stark Enterprises Global CA

    Run  rm -rf bundle
    Run  rm -f cert-bundle.tgz
    Run  rm -rf /root/ca

    Reload Default Certificate Authorities

    # Run vic-machine configure, supply server cert and key
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} ${vicmachinetls} --debug 1

    Log  ${output}
    Should Contain  ${output}  No certificate regeneration requested. No new certificates provided. Certificates left unchanged

    ${output}=  Run  openssl s_client -showcerts -connect %{VCH-IP}:2378
    Log  ${output}
    Should Contain  ${output}  issuer=/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Certificate Authority/CN=Stark Enterprises Global CA

    Reload Default Certificate Authorities

Configure VCH - Replace certificates with self-signed certificate using --no-tlsverify

    ${domain}=  Get Environment Variable  DOMAIN  ''
    Run Keyword If  '${domain}' == ''  Pass Execution  Skipping test - domain not set, won't generate keys

    Run  rm -rf foo-bar-certs
    ${output}=  Run  bin/vic-machine-linux configure --name=%{VCH-NAME} --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --thumbprint=%{TEST_THUMBPRINT} ${vicmachinetls} --tls-cert-path "foo-bar-certs" --debug 1 --no-tlsverify

    Should Contain  ${output}  Generating self-signed certificate/key pair - private key in foo-bar-certs/server-key.pem

    Should Contain  ${output}  Completed successfully

    ${output}=  Run  openssl s_client -showcerts -connect %{VCH-IP}:2378
    Log  ${output}

    Should Contain  ${output}  Verify return code: 21 (unable to verify the first certificate)
    Should Contain  ${output}  verify error:num=20:unable to get local issuer certificate
    Should Not Contain  ${output}  issuer=/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Certificate Authority/CN=Stark Enterprises Global CA
