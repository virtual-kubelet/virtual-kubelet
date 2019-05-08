# Contributing Guidelines

The Virtual Kubelet accepts contributions via GitHub pull requests. This document outlines the process to help get your contribution
accepted.

## Contributor License Agreements

If you are providing provider support for the Virtual Kubelet then we have to jump through some legal hurdles first.

The [CNCF CLA](https://github.com/kubernetes/community/blob/master/CLA.md) must be signed by all
contributors. Please fill out either the individual or corporate Contributor
License Agreement (CLA). Once you are CLA'ed, we'll be able to accept your pull
requests.

***NOTE***: Only original source code from you and other people that have
signed the CLA can be accepted into the repository.

## Maintainers

Each provider is responsible for reviewing PRs. Each provider has a primary and secondary maintainer for the purposes of maintaining their own code.
Here's the current list of maintainers.

Otherwise for the primary Virtual Kubelet code, and overall project maintenance, these are the current maintainers. If you want to become a maintainer for the overall project, or be a provider for Virtual Kubelet, please email Ria Bhatia at ria.bhatia@microsoft.com.

### Overall Maintainers

Ria Bhatia (ribhatia@microsoft.com)

Robbie Zhang (junjiez@microsoft.com)

Brian Goff (brian.goff@microsoft.com)

Paulo Pires (Mesosphere)


### Provider maintainers

**Azure**

Robbie Zhang (junjiez@microsoft.com)

**AWS Fargate**

Onur Filiz (onfiliz@amazon.com)

**Alibaba Cloud**

(TBA)

**Huawei**

Fei Xu (xufei40@huawei.com)

**Azure Batch**

Lawrence Gripper (lawrence.gripper@microsoft.com)

**CRI**

Ben Corrie (bcorrie@vmware.com)

**HashiCorp Nomad**

Anubhav Mishra (mishra@hashicorp.com)

**OpenStack**

OpenStack Zun team (https://governance.openstack.org/tc/reference/projects/zun.html)

## Support Channels

This is an open source project and as such no formal support is available.
However, like all good open source projects we do offer "best effort" support
through [github issues](https://github.com/virtual-kubelet/virtual-kubelet).

Before opening a new issue or submitting a new pull request, it's helpful to
search the project - it's likely that another user has already reported the
issue you're facing, or it's a known issue that we're already aware of.

## Issues

Issues are used as the primary method for tracking anything to do with the
Virtual Kubelet.

### Issue Lifecycle

The issue lifecycle is mainly driven by the core maintainers, but is good
information for those contributing to Virtual Kubelet. All issue types
follow the same general lifecycle. Differences are noted below.

1. Issue creation
1. Triage
    - The maintainer in charge of triaging will apply the proper labels for the
    issue. This includes labels for priority, type, and metadata. If additional
    labels are needed in the future, we will add them.
    - If needed, clean up the title to succinctly and clearly state the issue.
    Also ensure that proposals are prefaced with "Proposal".
    - Add the issue to the correct milestone. If any questions come up, don't
    worry about adding the issue to a milestone until the questions are
    answered.
    - We attempt to do this process at least once per work day.
1. Discussion
    - "Feature" and "Bug" issues should be connected to the PR that resolves it.
    - Whoever is working on a "Feature" or "Bug" issue (whether a maintainer or
    someone from the community), should either assign the issue to themself or
    make a comment in the issue saying that they are taking it.
    - "Proposal" and "Question" issues should remain open until they are
    either resolved or have remained inactive for more than 30 days. This will
    help keep the issue queue to a manageable size and reduce noise. Should the
    issue need to stay open, the `keep open` label can be added.
1. Issue closure

## How to Contribute a Patch

1. If you haven't already done so, sign a Contributor License Agreement
(see details above).
2. Fork the repository, develop and test your code changes.

You can use the following command to clone your fork to your local
```
cd $GOPATH
mkdir -p {src,bin,pkg}
mkdir -p src/github.com/virtual-kubelet/
cd src/github.com/virtual-kubelet/
git clone git@github.com:<your-github-account-name>/virtual-kubelet.git # OR: git clone https://github.com/<your-github-account-name>/virtual-kubelet.git
cd virtual-kubelet
go get ./...
# add the virtual-kubelet as the upstream
git remote add upstream git@github.com:virtual-kubelet/virtual-kubelet.git
```
3. Submit a pull request.

## Code of conduct

Virtual Kubelet follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).
