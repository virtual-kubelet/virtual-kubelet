# Contributing to VIC Engine

## Community

In addition to using the GitHub issue tracker, contributors and users are encouraged to collaborate using the following
resources:

- [Slack](https://vmwarecode.slack.com/messages/vic-engine): This is the primary community channel. **If you don't have
an @vmware.com or @emc.com email, please sign up at https://code.vmware.com/join to get a Slack invite.**

- [Gitter](https://gitter.im/vmware/vic): Gitter is monitored, but please use the Slack channel if you need a response
quickly.

## Getting started

First, fork the repository on GitHub to your personal account.

Note that _GOPATH_ can be any directory, the example below uses _$HOME/vic_.
Change _$USER_ below to your GitHub username.

``` shell
export GOPATH=$HOME/vic
mkdir -p $GOPATH/src/github.com/vmware
go get github.com/vmware/vic
cd $GOPATH/src/github.com/vmware/vic
git config push.default nothing # anything to avoid pushing to vmware/vic by default
git remote rename origin vmware
git remote add $USER git@github.com:$USER/vic.git
git fetch $USER
```

See the [README](README.md#building) for build instructions.

## Contribution flow

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where you want to base your work.
- Make commits of logical units.
- Make sure your commit messages are in the proper format (see below).
- Push your changes to a topic branch in your fork of the repository.
- Test your changes as detailed in the [Automated Testing](#automated-testing) section.
- Submit a pull request to vmware/vic.
- Your PR must receive approvals from component owners and at least two approvals overall from maintainers before merging.

Example:

``` shell
git checkout -b my-new-feature vmware/master
git commit -a
git push $USER my-new-feature
```

### Stay in sync with upstream

When your branch gets out of sync with the vmware/master branch, use the following to update it:

``` shell
git checkout my-new-feature
git fetch -a
git rebase vmware/master
git push --force-with-lease $USER my-new-feature
```

### Updating pull requests

If your PR fails to pass CI or needs changes based on code review, you'll most likely want to squash these changes into
existing commits.

If your pull request contains a single commit or your changes are related to the most recent commit, you can simply
amend the commit.

``` shell
git add .
git commit --amend
git push --force-with-lease $USER my-new-feature
```

If you need to squash changes into an earlier commit, you can use:

``` shell
git add .
git commit --fixup <commit>
git rebase -i --autosquash vmware/master
git push --force-with-lease $USER my-new-feature
```

Be sure to add a comment to the PR indicating your new changes are ready to review, as GitHub does not generate a
notification when you git push.

### Code style

VIC Engine uses the coding style suggested by the Golang community. See the
[style doc](https://github.com/golang/go/wiki/CodeReviewComments) for details.

Try to limit column width to 120 characters for both code and markdown documents such as this one.

### Format of the Commit Message

We follow the conventions on [How to Write a Git Commit Message](http://chris.beams.io/posts/git-commit/).

Be sure to include any related GitHub issue references in the commit message. See
[GFM syntax](https://guides.github.com/features/mastering-markdown/#GitHub-flavored-markdown) for referencing issues and
commits.

To help write conforming commit messages, we recommend setting up the [git-good-commit][commithook] commit hook. Run this
command in the VIC repo's root directory:

```shell
curl https://cdn.rawgit.com/tommarshall/git-good-commit/v0.6.1/hook.sh > .git/hooks/commit-msg && chmod +x .git/hooks/commit-msg
```

[dronevic]:https://ci-vic.vmware.com/vmware/vic
[dronesrc]:https://github.com/drone/drone
[dronecli]:http://docs.drone.io/cli-installation/
[commithook]:https://github.com/tommarshall/git-good-commit

## Automated Testing

Automated testing uses [Drone][dronesrc].

Pull requests must pass unit tests and integration tests before being merged into the master branch. A standard PR builds
the project and runs unit and regression tests. To customize the integration test suite that runs in your pull request,
you can use these keywords in your PR body:

- To skip running tests (e.g. for a work-in-progress PR), use `[ci skip]` or `[skip ci]`.
  - This customization must be set at the beginning of the PR title, not the PR body.
- To run the full test suite, use `[full ci]`.
- To run _specific_ integration test or group, use `[specific ci=$test]`. This will run the regression test as well. Examples:
  - To run the `1-01-Docker-Info` suite: `[specific ci=1-01-Docker-Info]`
  - To run all suites under the `Group1-Docker-Commands` group: `[specific ci=Group1-Docker-Commands]`
  - To run several specific suites: `[specific ci=$test1 --suite $test2 --suite $test3]`.
- To skip running the unit tests, use `[skip unit]`.
- To fail fast (make normal failures fatal) during the integration testing, use `[fast fail]`.
- To specify a specific datastore you want, use `[shared datastore=nfs-datastore]`.
- To specify the number of parallel jobs you want, use `[parallel jobs=2]`.

You can run the tests locally before making a PR or view the Drone build results for [unit tests and integration tests][dronevic].

If you don't have a running ESX required for tests, you can leverage the automated Drone servers for
running tests. Add `WIP` (work in progress) to the PR title to alert reviewers that the PR is not ready to be merged.

If your Drone build needs to be restarted, fork the build:
```shell
export DRONE_TOKEN=<Drone Token>
export DRONE_SERVER=https://ci-vic.vmware.com

drone build start vmware/vic <Build Number>
```
If you are not a member of `vmware` org in github, then your PR build may fail. In that case, request one of the existing members / reviewers to fork your failed build to skip membership checking.
```shell
drone build start --param SKIP_CHECK_MEMBERSHIP=true vmware/vic <Build Number>
```

### Testing locally

Developers need to install [Drone CLI][dronecli].

#### Unit tests

``` shell
VIC_ESX_TEST_URL="<USER>:<PASS>@<ESX IP>" drone exec .drone.yml
```

If you don't have a running ESX, tests requiring an ESX can be skipped with the following:

``` shell
drone exec
```

#### Integration tests

Integration tests require a running ESX on which to deploy VIC Engine. See [VIC Integration & Functional Test Suite](tests/README.md).

## Reporting Bugs and Creating Issues

When opening a new issue, try to roughly follow the commit message format conventions above.

We use [Zenhub](https://www.zenhub.io/) for project management on top of GitHub issues.  Once you have the Zenhub
browser plugin installed, click on the [Boards](https://github.com/vmware/vic/issues#boards) tab to open the Zenhub task
board.

Our task board practices are as follows:

### New Issues

The New Issues are triaged by the team at least once a week.  We try to keep issues from staying in this pipeline for
too long.  After triaging and issue, it will likely be moved to the backlog or stay under [Not Ready](#not-ready) for deferred
discussion.

For VIC engineers, you should set the priority based on the below guidelines. Everyone else, do not set the priority of a new issue.

#### Priorities

| Priority | Bugs | Features | Non Bugs |
| -------- | ---- | -------- | -------- |
| priority/p0 | Bugs that NEED to be fixed immediately as they either block meaningful testing or are release stoppers for the current release. | No Feature should be p0. | An issue that is not a bug and is blocking meaningful testing. eg. builds are failing because the syslog server is out of space. |
| priority/p1 | Bugs that NEED to be fixed by the assigned phase of the current release. | A feature that is required for the next release, typically an anchor feature; a large feature that is the focus for the release and drives the release date. | An issue that must be fixed for the next release. eg. Track build success rates. |
| priority/p2 | Bugs that SHOULD be fixed by the assigned phase of the current release, time permitting. | A feature that is desired for the next release, typically a pebble; a feature that has been approved for inclusion but is not considered the anchor feature or is considered good to have for the anchor feature. | An issue that we should fix in the next release. eg. A typo in the UI. |
| priority/p3 | Bugs that SHOULD be fixed by a given release, time permitting. | A feature that can be fixed in the next release. eg. Migrate to a new kernel version. Or a feature that is nice to have for a pebble. | An issue that can be fixed in the next release. eg. Low hanging productivity improvements. |
| priority/p4 | Bugs that SHOULD be fixed in a future (to be determined) release. | An issue or feature that will be fixed in a future release. | An issue or feature that will be fixed in a future release. |

### Not Ready

The Not Ready column is for issues that need more discussion, details and/or triaging before being put in the [Backlog](#backlog). Issues in Not Ready should have assignee(s) to track whose input is needed to put the issue in the Backlog. For issues reported by VIC engineers: if the issue's details aren't fleshed out, the reporter should set themselves as the assignee.

### Backlog

Issues in Backlog should be ready to be worked on in future sprints. For example, they may be feature requests or ideas for a future version of
the project. When moving issues to the Backlog, add more information (like requirements and outlines) into each issue. It's useful to
get ideas out of your head, even if you will not be touching them for a while.

To move an issue into the Backlog swim lane, it must have:

1. a `priority/...` label
2. a `team/...` label
3. an estimated level of effort (see [Story point estimates](#story-point-estimates) for guidance for mapping effort to story points)
4. no assignee (assignees are set when the issue is selected to work on)

Other labels should be added as needed.

Prioritize issues by dragging and dropping their placement in the pipeline. Issues higher in the pipeline are higher
priority; accordingly, they should contain all the information necessary to get started when the time
comes. Low-priority issues should still contain at least a short description.

### To Do

This is the team's current focus and the issues should be well-defined. This pipeline should contain the high-priority
items for the current milestone. These issues must have an assignee, milestone, estimate and tags. Items are moved
from this pipeline to In Progress when work has been started.

To move an issue into the To Do swim lane, the assignee and milestone fields should be set.

### In Progress

This is the answer to, "What are you working on right now?" Ideally, this pipeline will not contain more issues than
members of the team; each team member should be working on one thing at a time.

Issues in the In Progress swim lane must have an assignee.

After an issue is In Progress, it is best practice to update the issue with current progress and any discussions that may occur via the various collaboration tools used. An issue that is in progress should not go more than 2 days without updates.

Note: Epics should never be In Progress.

### Verify

A "Verify" issue normally means the feature or fix is in code review and/or awaiting further testing. These issues require one final QE sign off or at the end of a sprint another dev that didn't work on the issue can verify the issue.

In most cases, an issue should be in Verify _before_ the corresponding PR is merged. The developer can then close the issue while merging the PR.

### Closed

This pipeline includes all closed issues. It can be filtered like the rest of the Board – by Label, Assignee or Milestone.

This pipeline is also interactive: dragging issues into this pipeline will close them, while dragging them out will re-open them.

## Story point estimates

* Use the fibonacci pattern
* All bugs are a 2 unless we know it is significantly more or less work than the average bug
* 1 is easier than the average bug
* 3 is slightly more work than the average bug and probably should be about an average feature work for an easy feature (which includes design doc, implementation, testing, review)
* 5 is about 2x more work than the average bug and the highest single issue value we want
* Issues with an estimate higher than 5 should be decomposed further
* Unless otherwise necessary, estimates for EPICs are the sum of their sub-issues' estimates - EPICs aren't assigned an estimate themselves

## High level project planning

We use the following structure for higher level project management:
* Epic (zenhub) - implements a functional change - for example 'attach, stdout only', may span milestones and releases. Expected to be broken down from larger Epics into smaller epics prior to commencement.
* Milestones - essentially higher level user stories
* Labels - either by functional area (`component/...`) or feature (`feature/...`)

## Repository structure

The layout in the repo is as follows - this is a recent reorganisation so there is still some mixing between directories:
* cmd - the main packages for compiled components
* doc - all project documentation other than the standard files in the root
* infra - supporting scripts, utilities, et al
* isos - ISO mastering scripts and uncompiled content
* lib - common library packages that are tightly coupled to vmware/vic
* pkg - packages that are not tightly coupled to vmware/vic and could be usefully consumed in other projects. There is still some sanitization to do here.
* tests - integration and system test code that doesn't use go test
* vendor - standard Go vendor model

## Troubleshooting

* If you're building the project in a VM, ensure that it has at least 4GB memory to avoid memory issues during a build.

