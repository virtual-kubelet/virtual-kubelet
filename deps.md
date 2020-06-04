# How to Upgrade Deps
Kubernetes takes a pseudo-mono-repo approach to its dependencies.
Because of this, we need to have our go.mod do a bunch of rewrites,
in order to read the actual version

## Steps
Set the versions of the top level dep `k8s.io/kubernetes` to the release.
A la:

`k8s.io/kubernetes d32e40e20d167e103faf894261614c5b45c44198`

Replace all of the "replace" the entries in go.mod with:

`replace k8s.io/component-base => github.com/kubernetes/kubernetes/staging/src/k8s.io/component-base d32e40e20d167e103faf894261614c5b45c44198`

You may need to add  additional replace entries, based repository list in
the [kubernetes repository](https://github.com/kubernetes/kubernetes/tree/release-1.17/staging).

You *must* use the sha, not a tag. The reason behind this is that git tags are handled
differently by go modules and they are prefixed with the module name.
More details about this can be found in the (go documentation)[https://github.com/golang/go/wiki/Modules#publishing-a-release]

Once this is done, run go build ./...


### Notes
All of the k8s.io/* references in go.mod should reference v0.0.0 other than k8s.io/kubernetes
