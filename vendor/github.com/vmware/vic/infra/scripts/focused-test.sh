#!/bin/bash
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
#
# Runs go test on any packages that differ from vmware/master that have
# Go test files.
#
# Usage: infra/scripts/focused-test.sh
#

if [ -z "$1" ]; then
    export REMOTE=$(git remote -v | grep "github.com.vmware/vic.git (fetch)" | awk '{print$1;exit}')/master
    echo "Using ${REMOTE} as default remote"
else
    echo "Using ${REMOTE} as specified remote"
    export REMOTE="$1"
fi

echo "Finding modified packages with test files"
PKG=$(git diff --stat ${REMOTE} --name-only | xargs dirname | uniq | while read i; do
    for f in ./$i/*_test.go; do
        if [ -e $f ]; then
            echo -n "./$i ";
        fi;
        break;
    done;
done)

echo "Testing packages: $PKG"
if [ -n "$PKG" ]; then
    go test $PKG;
fi
