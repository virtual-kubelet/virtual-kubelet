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

# Sign a CSR with specified CA

set -euf -o pipefail

CA_NAME="STARK_ENTERPRISES_ROOT_CA" # used to verify cert signature
OUTDIR="/root/ca"
SERVER_CERT_CN="starkenterprises.io"

while getopts ":c:d:n:" opt; do
  case $opt in
    c) CA_NAME="$OPTARG"
    ;;
    d) OUTDIR="$OPTARG"
    ;;
    n) SERVER_CERT_CN="$OPTARG"
    ;;
    \?) echo "Invalid option -$OPTARG" >&2
    ;;
  esac
done


CONF_DIR=`dirname $0`
cd $OUTDIR
openssl ca -config $CONF_DIR/openssl.cnf \
    -batch \
    -extensions server_cert \
    -days 365 -notext -md sha256 \
    -in csr/${SERVER_CERT_CN}.csr.pem \
    -out certs/${SERVER_CERT_CN}.cert.pem

chmod 444 certs/${SERVER_CERT_CN}.cert.pem
openssl x509 -noout -text -in certs/${SERVER_CERT_CN}.cert.pem

# Test certificate
openssl verify -CAfile certs/$CA_NAME.crt certs/${SERVER_CERT_CN}.cert.pem
