#!/usr/bin/env bash
set -e

exit_code=0

echo "==> Running go mod tidy to add missing and remove unused modules <=="

GO111MODULE=on go mod tidy || exit_code=1
if [ $exit_code -ne 0 ]; then
  echo 'Checking go.mod and go.sum files'
else
  echo "go mod tidy passed."
fi

if [ ${exit_code} -eq 0 ]; then
	exit 0
fi

echo "please run \`go mod tidy\` and check in the changes"
exit ${exit_code}
