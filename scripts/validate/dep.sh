#!/usr/bin/env bash

exit_code=0

echo "==> Running dep check <=="

dep check || exit_code=1

if [ $exit_code -ne 0 ]; then
  echo '`dep ensure` was not run. Make sure dependency changes are committed to the repository.'
  echo 'You may also need to check that all deps are pinned to a commit or tag instead of a branch or HEAD.'
  echo 'Check Gopkg.toml and Gopkg.lock'
else
  echo "dep check passed."
fi

exit $exit_code