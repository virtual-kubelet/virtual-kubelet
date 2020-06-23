#!/bin/bash
set -euo pipefail

echo "Checking kubernetes version replacement / pinning"

readonly kubeversion=$(go list -m -f '{{.Version}}' k8s.io/kubernetes)

if [[ "${kubeversion}" =~ '^v.*' ]]; then
	echo "Kube Version: ${kubeversion} in unexpected format, must start with v"
	exit 1
fi

echo "Found kubernetes version: ${kubeversion}"

# We postfix the tag to search for with ^{}, as we want the annotated tag
readonly sha=$(git ls-remote --tags git@github.com:kubernetes/kubernetes.git "${kubeversion}^{}"|cut -f1 -d$'\t')

if [[ ! "${sha}" =~ ^[0-9a-f]{40}$ ]]; then
	echo "Kube Sha: ${sha} in unexpected format"
	exit 1
fi

echo "Found sha for kubernetes version ${kubeversion}: ${sha}"
# TODO: Check direct deps
# Humans should check the versions reference directly (i.e. bits under require) those should either read
# kubeversion, or v0.0.0


readonly short_sha=$(echo $sha|cut -b1-12)
readonly non_matching_versions=$(go list -m -json k8s.io/...|jq --arg short_sha "${short_sha}" 'select(.Replace != null) | select(.Replace.Version[-12:] != $short_sha)')


if [[ ! -z "${non_matching_versions}" ]]; then
	echo "Found non-matching versions: ${non_matching_versions}"
	exit 1
fi


