#!/bin/bash

# set -e -o pipefail

orig_tags=(${@})
new_tags=()
omit_providers=()

let found_provider=0

function debug() {
	if [ "${V}" = "1" ]; then
		(>&2 echo "$@")
	fi
}

for tag in ${orig_tags[@]}; do
	case "${tag}" in
	no_*_provider)
		# filtered
		debug "filtered old, invalid ${tag} from tag list, provider is already excluded"

		# store just in case no "proper" provider tags were provided
		# In such cases we'd build everything, but we don't want to build these
		p="${tag#no_}"
		p="${p%_*}"
		omit_providers+=("${p}")
		;;
	*_provider)
		found_provider+=1
		new_tags+=("${tag}")
		;;
	*)
		new_tags+=("${tag}")
		;;
	esac
done

if [ ${found_provider} -eq 0 ]; then
	# include all providers
	for i in $(ls providers/register/provider_*.go); do
		p="${i#*provider_}"
		p="${p%.*}"

		if [[ ! "${omit_providers[*]}" =~ "${p}"  ]]; then
			debug "including provider ${p}"
			new_tags+=("${p}_provider")
		else
			debug "excluding provider ${p}"
		fi
	done
fi



echo "${new_tags[@]}"
