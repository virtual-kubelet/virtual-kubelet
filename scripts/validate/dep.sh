#!/bin/bash

validate_dep() {
	dep ensure || return $?
	# only check the vendor dir since different dep versions can cause changes to Gopkg files
	nChanges=$(git status --porcelain -u ./vendor | wc -l)
	[ $nChanges -eq 0 ]
}


validate_dep || {
	ret=$?
	echo '`dep ensure` was not run. Make sure dependency changes are committed to the repository.'
	echo 'You may also need to check that all deps are pinned to a commit or tag instead of a branch or HEAD.'
	echo 'Check Gopkg.toml and Gopkg.lock'
	git status --porcelain -u
	dep version
	exit $ret
}