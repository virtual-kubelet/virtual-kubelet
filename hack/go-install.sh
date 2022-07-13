#!/usr/bin/env bash
# https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/master/scripts/go_install.sh

set -o errexit
set -o nounset
set -o pipefail

if [[ -z "${1}" ]]; then
  echo "must provide module as first parameter"
  exit 1
fi

if [[ -z "${2}" ]]; then
  echo "must provide binary name as second parameter"
  exit 1
fi

if [[ -z "${3}" ]]; then
  echo "must provide version as third parameter"
  exit 1
fi

if [[ -z "${GOBIN}" ]]; then
  echo "GOBIN is not set. Must set GOBIN to install the bin in a specified directory."
  exit 1
fi

tmp_dir=$(mktemp -d -t goinstall_XXXXXXXXXX)
function clean {
  rm -rf "${tmp_dir}"
}
trap clean EXIT

rm "${GOBIN}/${2}"* || true

cd "${tmp_dir}"

# create a new module in the tmp directory
go mod init fake/mod

# install the golang module specified as the first argument
go install "${1}@${3}"
mv "${GOBIN}/${2}" "${GOBIN}/${2}-${3}"
ln -sf "${GOBIN}/${2}-${3}" "${GOBIN}/${2}"
