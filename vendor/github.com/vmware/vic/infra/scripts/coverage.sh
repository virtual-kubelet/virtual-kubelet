#!/bin/bash -e
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
#
# Works around the fact that `go test -coverprofile` does not work
# with multiple packages, see https://code.google.com/p/go/issues/detail?id=6909
#
# Usage: script/coverage [--html]
#
#     --html        Create HTML report and open it in browser
#

workdir=`git rev-parse --show-toplevel`/.cover
profile="$workdir/cover.out"
dir=$(dirname $0)
mode=count

# list any files (or patterns) to explicitly exclude from coverage
# you should have a pretty good reason before putting items here
exclude_files=(

)

join() { local IFS="$1"; shift; echo "$*"; }

excludes=$(join "|" ${exclude_files[@]} | sed -e 's/\./\\./g')

generate_pkg_cover_data() {
    echo "$@"
    mkdir -p "$workdir"

    for pkg in "$@"; do
        f="$workdir/$(echo $pkg | tr / -).cover"
        go test -i -covermode="$mode" -coverprofile="$f" "$pkg"
        go test -covermode="$mode" -coverprofile="$f" "$pkg"
    done

    echo "mode: $mode" >"$profile"
    if [ -n "$excludes" ]; then
        grep -h -v "^mode:" "$workdir"/*.cover | egrep -v "$excludes" >>"$profile"
    else
        grep -h -v "^mode:" "$workdir"/*.cover >>"$profile"
    fi
}

# translate dirs to packages and strip args
dir_to_pkg() {
    for dir in $@; do
        if test "$dir" == "--html"; then
            export html="true"
        else
            pkgs="$pkgs $(go list $dir/... | grep -v /vendor/)"
        fi
    done
    echo $pkgs
}

show_cover_report() {
    go tool cover -${1}="$profile"
}

generate_pkg_cover_data $(dir_to_pkg "$@")

show_cover_report func
if test "$html" == "true"; then
    show_cover_report html
fi
