##  header-check

Simple header check for CI jobs, currently checks ".go" files only.

This will be called by the CI system (with no args) to perform checking and
fail the job if headers are not correctly set. It can also be called with the
'fix' argument to automatically add headers to the missing files.

Check if headers are fine:
```
  $ ./infra/scripts/header-check.sh
```
Check and fix headers:
```
  $ ./infra/scripts/header-check.sh fix
```