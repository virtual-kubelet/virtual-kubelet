#!/usr/bin/env bash
# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Generate a root CA and a server certificate

##### Default configuration options
SERVER_CERT_CN="starkenterprises.io"
#####

while getopts ":s:" opt; do
  case $opt in
    s) SERVER_CERT_CN="$OPTARG"
    ;;
    \?) echo "Invalid option -$OPTARG" >&2
    ;;
  esac
done


### Create root CA
mkdir -p /root/ca
cp openssl.cnf /root/ca
cd /root/ca
mkdir certs crl csr newcerts private
chmod 700 private
touch index.txt
echo 1000 > serial

# Generate root CA key
# Private key is not encrypted - use -aes256 to specify a password
openssl genrsa -out private/ca.key.pem 4096
chmod 400 private/ca.key.pem

# Generate root CA CSR
openssl req -config openssl.cnf \
    -new -sha256 \
    -key private/ca.key.pem \
    -out csr/ca.csr.pem \
    -extensions v3_ca \
    -subj "/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Certificate Authority/CN=Stark Enterprises Global CA"

# Self sign for root CA certificate
openssl x509 -req -extfile openssl.cnf \
    -extensions v3_ca \
    -days 7300 -in csr/ca.csr.pem -signkey private/ca.key.pem -out certs/ca.cert.pem

chmod 444 certs/ca.cert.pem
openssl x509 -noout -text -in certs/ca.cert.pem

# Output CRT format
openssl x509 -in certs/ca.cert.pem -inform PEM -out certs/ca.cert.crt


### Create server certificate
cd /root/ca
# Generate server key
# Private key is not encrypted - use -aes256 to specify a password
openssl genrsa -out private/${SERVER_CERT_CN}.key.pem 4096
chmod 400 private/${SERVER_CERT_CN}.key.pem

# Generate server CSR
openssl req -config openssl.cnf \
    -new -sha256 \
    -key private/${SERVER_CERT_CN}.key.pem \
    -out csr/${SERVER_CERT_CN}.csr.pem \
    -subj "/C=US/ST=California/L=Los Angeles/O=Stark Enterprises/OU=Stark Enterprises Web Services/CN=${SERVER_CERT_CN}"

# Sign CSR with CA key, output server certificate
openssl ca -config openssl.cnf \
    -batch \
    -extensions server_cert \
    -days 365 -notext -md sha256 \
    -in csr/${SERVER_CERT_CN}.csr.pem \
    -out certs/${SERVER_CERT_CN}.cert.pem

chmod 444 certs/${SERVER_CERT_CN}.cert.pem
openssl x509 -noout -text -in certs/${SERVER_CERT_CN}.cert.pem

# Test certificate
openssl verify -CAfile certs/ca.cert.pem certs/${SERVER_CERT_CN}.cert.pem


### Bundle output
mkdir bundle
cp certs/ca.cert.crt bundle
cp private/${SERVER_CERT_CN}.key.pem bundle
cp certs/${SERVER_CERT_CN}.cert.pem bundle
tar cvf cert-bundle.tgz bundle
