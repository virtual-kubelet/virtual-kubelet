#!/bin/bash
# Copyright 2016 VMware, Inc. All Rights Reserved.
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

red="\033[1;31m"
color_end="\033[0m"
GLOBIGNORE='vendor'

# will check staged items for whitespace
if [[ $# -gt 0 && $1 =~ [[:upper:]pre] ]]; then
    if [[ $(git diff --cached --check) ]]; then
        echo -e ${red}whitespace check failed${color_end}
        git diff --cached --check
        exit 1
    fi
else
    # check staged and changed
    if [[ $(git diff --cached --check) || $(git diff --check) ]]; then
        echo -e ${red}whitespace check failed${color_end}
        git diff --cached --check
        git diff --check
        exit 1
    fi
fi
