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

source=${1:?A source file must be specified}
# The target destination in cloud storage. If the destination ends with a / it will be treated as a directory.
# If not it _may_ be treated as a directory or a file depending on current remote objects. see gsutil cp doc.
dest=${2:?A bucket must be specified, eg vic-ci-logs, or vic-ci-logs/user/branch/}

if [ ! -r "${source}" ]; then
  echo "Specified source file does not exist or cannot be read: ${source}"
  exit 1
fi

if [ ${dest:0:1} == "/" ]; then
  echo "Destination must start with a bucket name and no leading /"
  exit 1
fi

# extract first path element as bucket name
bucket=${dest%%/*}
# drop bucket portion from path
path=${dest#*/}
# drop trailing / if any
path=${path%/}

# GC credentials
keyfile=~/${bucket}.key
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

echo "----------------------------------------------"
echo "Uploading logs to ${dest}"
echo "----------------------------------------------"
if gsutil cp "${source}" "gs://${dest}"; then
  url="https://console.cloud.google.com/m/cloudstorage/b/${bucket}/o/${path}/${source}?authuser=1"
  echo "$url" > log-download.url
  echo "Download test logs here:"
  echo "$url"
else
  echo "Log upload faled. Dumping gsutil version logic"
  set -x
  gsutil version -l
  set +x
fi
echo "----------------------------------------------"

rm -f $keyfile


