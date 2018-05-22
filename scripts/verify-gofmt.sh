#!/bin/sh

set -o errexit

find_files() {
  find ../ -not -wholename '*/vendor/*' -name '*.go'
}

diff=$(find_files | xargs gofmt -d -s 2>&1)
if [[ -n "${diff}" ]]; then
  echo "${diff}" >&2
  echo >&2
  read -ra go_version <<< "$(go version)"
  echo "Please use ${go_version[2]} and run gofmt for above files." >&2
  exit 1
fi
