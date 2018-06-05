#!/bin/bash -e
# Copyright 2017 VMware, Inc. All Rights Reserved.
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

# Saves test results to reporting server

echo "rpcbind : $REPORTING_SERVER_URL" >>  /etc/hosts.allow
mkdir -p /run/sendsigs.omit.d
service rpcbind restart
mkdir /drone-test-results
mount $REPORTING_SERVER_URL:/export/drone-test-results /drone-test-results

# copy test run files to build folder
mkdir -p /drone-test-results/testruns/$DRONE_BUILD_NUMBER
cp log.html report.html /drone-test-results/testruns/$DRONE_BUILD_NUMBER/
