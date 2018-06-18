# VIC Branching

This document is used to define a strategy to enable development and testing of
release candidate, patch found issues, while allowing development of future
releases by way of branching and tagging.

## Requirements

_(Not in priority order)_
* To insulate development of future releases from RC / GA testing and development.
  * Allow development of future releases.
  * Avoid breaking RC releases while in testing.

_Non requirements_
* Define how to enable development of new features on MASTER or otherwise.
* Define how to test RC / GA releases.
* Define criteria for backporting or forward porting of found bugfixes, features, etc.
* Define release trains for patching GA released builds.
* Define release criteria for RC

## Proposal

We can keep changes isolated from RC / GA testing by way of `git` branches.

###Branching###
* Use [master](http://github.com/vmware/vic) for future release work.
* Use RC branch (`releases/MAJOR.MINOR.MACRO`) for RC release work.
  * `TAG` branch for each RC
  * `TAG` with `MACRO++` for each patch.

###Accounting###
* Targeting
  * Bugs found in RC branch need an issue before merging fix to branch targeted to RC, e.g. `targeted/<RC branch name>`
  * Ideally patch should be merged to `MASTER` first if it exists there too.
  * Only if issue is targeted for RC, a different PR with the same issue number for the RC branch.
  * Only close issue after each relevant and targeted release has had a fixed merged to it.  We can use targeting to verify this.
* Bugs found in `MASTER` or any RC branch need to be tagged, e.g.`exists/<branches>`
* The release branch will live as long as our support contract exists on that branch, once support sunsets we can remove the branch
* The CI system will build all branches within the `releases/*` naming convention on push or tag event and publish the binary to our public binary location

```

MASTER -------------------------------------------------->
           \                                 \
           0.7----------------                \
               \              \                \
               TAG 0.7.1   TAG 0.7.2            \
                                                 \
                                                 0.8---->


```
