# Contributing Guidelines

The Virtual Kubelet accepts contributions via GitHub pull requests. This document outlines the process to help get your contribution accepted.

## Contributor License Agreements

If you are providing provider support for the Virtual Kubelet then we have to jump through some legal hurdles first.

The [CNCF CLA](https://github.com/kubernetes/community/blob/master/CLA.md) must be signed by all
contributors. Please fill out either the individual or corporate Contributor
License Agreement (CLA). Once you are CLA'ed, we'll be able to accept your pull
requests.

***NOTE***: Only original source code from you and other people that have
signed the CLA can be accepted into the repository.

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


## Submission and Review guidelines

We welcome and appreciate everyone to submit and review changes. Here are some guidelines to follow for help ensure
a successful contribution experience.

Please note these are general guidelines, and while they are a good starting point, they are not specifically rules.
If you have a question about something, feel free to ask:

- [#virtual-kubelet](https://kubernetes.slack.com/archives/C8YU1QP8W) on Kubernetes Slack
- [virtualkubelet-dev@lists.cncf.io](mailto:virtualkubelet-dev@lists.cncf.io)
- GitHub Issues

#### Don't make breaking API changes.

Since Virtual Kubelet has reached 1.0 it is a major goal of the project to keep a stable API.
Breaking changes must only be considered if a 2.0 release is on the table, which should only come with thoughtful
consideration of the projects users as well as maintenance burden.

Also note that behavior changes in the runtime can have cascading effects that cause unintended failures. Behavior
changes should come well documented and with ample consideration for downstream effects. If possible, they should be
opt-in.

#### Public APIs

Public API's should be extendable and flexible without requiring breaking changes.
While we can always add a new function (`Foo2()`), a new type, etc, doing so makes it harder for people to update to
the new behavior.

Build API interfaces that do not need to be changed to adopt new or improved functionality. Opinions on how a particular
thing should work should be encoded by the user rather than implicit in the runtime. Defaults are fine, but defaults
should be overridable.

The smaller the surface area of an API, the easier it is to do more interesting things with it.

#### Building blocks

Don't overload functionality. If something is complicated to setup we can provide helpers or wrappers to do that, but
don't require users to do things a certain way because this tends to diminish the usefulness, especially as it relates
to runtimes.

We also do not want the maintenance burden of every users individual edge cases.

#### Use context.Context

Probably if it is a public/exported API, it should take a `context.Context`. Even if it doesn't need one today, it may
need it tomorrow, and then we have a breaking API change.

We use `context.Context` for storing loggers, tracing spans, and cancellation all across the project. Better safe
than sorry: add a `context.Context`.

#### Errors

Callers can't handle errors if they don't know what the error is, so make sure they can figure that out.
We use a package `errdefs` to define the types of errors we currently look out for. We do not typically look for
concrete error types, so check out `errdefs` and see if there is already an error type in there for your needs, or even
create a new one.

#### Testing

Ideally all behavior would be tested, in practice this is not the case. Unit tests are great, and fast. There is also
an end-to-end test suite for testing the overall behavior of the system. Please add tests. This is also a great place
to get started if you are new to the codebase.

## Code of conduct

Virtual Kubelet follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).
