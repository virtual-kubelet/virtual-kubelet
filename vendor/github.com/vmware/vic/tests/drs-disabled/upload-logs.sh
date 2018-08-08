#!/bin/bash
# Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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
#

echo "Upload drs-disabled test logs"

set -x
gsutil version -l
set +x

outfile="vic_drs_disabled_logs_"$BUILD_TIMESTAMP".zip"
echo $outfile

/usr/bin/zip -9 -r $outfile drs-disabled *.zip *.log *.debug *.tgz

# GC credentials
keyfile=~/vic-drs-disabled-logs.key
botofile=~/.boto
if [ ! -f $keyfile ]; then
    echo -en $GS_PRIVATE_KEY > $keyfile
    chmod 400 $keyfile
fi
if [ ! -f $botofile ]; then
    echo "[Credentials]" >> $botofile
    echo "gs_service_key_file = $keyfile" >> $botofile
    echo "gs_service_client_id = $GS_CLIENT_EMAIL" >> $botofile
    echo "[GSUtil]" >> $botofile
    echo "content_language = en" >> $botofile
    echo "default_project_id = $GS_PROJECT_ID" >> $botofile
fi

if [ -f "$outfile" ]; then
  gsutil cp $outfile gs://vic-drs-disabled-logs
  echo "----------------------------------------------"
  echo "Download test logs here:"
  echo "https://console.cloud.google.com/m/cloudstorage/b/vic-drs-disabled-logs/o/$outfile?authuser=1"
  echo "----------------------------------------------"
else
  echo "No log output file to upload"
fi

if [ -f "$keyfile" ]; then
  rm -f $keyfile
fi
