#!/usr/bin/env bash

exit_code=0

echo "==> Running go mod verify check <=="

GO111MODULE=on go mod verify || exit_code=1

if [ $exit_code -ne 0 ]; then
  echo '`go mod verify` was not run. Make sure dependency changes are committed to the repository.'
  echo 'You may also need to check that all deps are pinned to a commit or tag instead of a branch or HEAD.'
  echo 'Check go.mod and go.sum'
else
  echo "go mod verify passed."
fi

exit $exit_code
