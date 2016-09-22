# GitLab Runner release process

To handle the growth of this project, in `v1.6` we've introduced a release process correlated
with GitLab's CE/EE release process.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Release roadmap](#release-roadmap)
  - [Stable release timeline](#stable-release-timeline)
  - [Supported releases](#supported-releases)
  - [Release planning](#release-planning)
- [Workflow, merging and tagging strategy](#workflow-merging-and-tagging-strategy)
  - [Stable release](#stable-release)
  - [Patch releases](#patch-releases)
  - [Branch naming](#branch-naming)
- [Documentation](#documentation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Release roadmap

Starting with `v1.4` GitLab Runner is released on the 22th day of each month - together
with GitLab CE and GitLab EE projects.

### Stable release timeline

- 12th day of a month:
    - tag first RC version on `master` branch, eg. `v1.6.0-rc.1`
    - deploy the RC version to `docker-ci-X.gitlap.com`
      (each next RC version should be deployed to above hosts)

- 17th day of a month:
    - tag next RC version on `master` branch, eg. `v1.6.0-rc.2`
    - deploy the current RC version to `shared-runners-manager-X.gitlab.com`
      (each next RC version should be deployed to above hosts)

- 20th day of a month:
    - close the _new features_ window.

        From now on we're merging only features that are ready and are eventually
        waiting for documentation or last fixes. But until _new features_ window
        is closed we will not merge any feature that wasn't discussed even if it
        contains all needed changes (code, new tests, documentation) and for which
        all tests are passing.

        > There is nothing bad in moving a feature to the next release at this stage,
        > if it's still not _production ready_!

- 21th day of a month:
    - tag last RC version, eg. 'v1.6.0-rc.5'

- 22th day of a month:
    - update the `CHANGELOG` file with entries for current release
    - tag a stable version **on `master` branch**
    - create the `X-Y-stable` on the current master
    - increase version number in `VERSION` file
    - deploy stable version to `docker-ci-X.gitlap.com` and `shared-runners-manager-X.gitlab.com`
    - announce the new release in _GitLab's release blog post_
    - open the _new features_ window
    - start the `pick-into-stable` strategy for the `X-Y-stable` branch

### Supported releases

Because of a fast development and release cycle - we release a new version
each 22th day of a month! - we need to prepare a strict policy of releases
supporting.

With this release process description we're starting the _last three
releases support_ policy. That means when we'll release a `v1.6` version
we will still support `v1.5.x` and `v1.4.x` versions. But only them.

After releasing `v1.7` we'll support `v1.5.x` and `v1.6.x` and so on.

Each support requests for previous versions will be closed with
a ~wontfix label.

**What is supported?**

By the _release support_ we understand:
- fixes for security bugs
- fixes for other bugs
- requests for documentation
- questions of type _"How can I ...?"_ related to a supported version

Proposals of new features or improvements are welcome but will be not
prepared for supported releases. Instead - if we decide to implement
them - they will be planned for one of upcoming releases.

### Release planning

For release planning we use the [_milestones_][runner-milestones] feature.

Each issue or merge request planned for a release will be assigned to
one of `vX.Y` milestones. This rule will be very important after
releasing the version when `pick-into-stable` strategy will be used to
merge changes into release stable branch.

After releasing a version the `vX.Y` milestone will be still used to assign
issues and merge requests related to support process.

We will plan only one version ahead. However we still want to have a way
to mark issues or merge requests that we decided to include in upcoming
releases even if we don't know when we'll have time for it. For this
purpose we've prepared the %Backlog milestone.

## Workflow, merging and tagging strategy

### Stable release

For a particular change:
- start work from `master` branch and open a MR into `master` branch
- assign the MR to a milestone related to the currently upcoming release
- choose a good, descriptive title for the MR since it will be automatically
  inserted in the `CHANGELOG` before doing the release
- if feature is planned for the current release (confirmed by the
  assigned milestone) - merge branch into master

For a whole release please follow the [Stable release timeline](#stable-release-timeline).

### Patch releases

For a particular change:
- if bug exists in the currently upcoming version:
  - start work from `master` branch and open a MR into `master` branch
  - assign the MR to a milestone related to the oldest version in which the bug exists
  - choose a good, descriptive title for the MR since it will be automatically
    inserted in the `CHANGELOG` before doing the release
  - assign the `pick-into-stable` label
  - merge branch into `master` branch
  - after branch is merged into `master` branch - cherry-pick the merge commit to each
    `X-Y-stable` branch starting from the branch related to the assigned
    milestone up to the latest release
- if bug doesn't exist in the currently upcoming version:
  - start work from `X-Y-stable` of the most recent version where the
    bug exists and open a MR into this branch
  - assign the MR to a milestone related to the oldest version in which the bug exists
  - choose a good, descriptive title for the MR since it will be automatically
    inserted in the `CHANGELOG` before doing the release
  - assign the `pick-into-stable` label
  - merge branch into assigned `X-Y-stable` branch
  - after branch is merged into assigned `X-Y-stable` branch - cherry-pick the merge
    commit to each `X-Y-stable` branch starting from the branch related to the
    assigned milestone up to the latest release before the MR target

For each `X-Y-stable` branch - if the release should be published:
  - update the `CHANGELOG` file with entries for current release
  - create the `vX.Y.patch` tag on the `X-Y-stable` branch
  - add created `CHANGELOG` entries into `CHANGELOG` file on the `master` branch

### Branch naming

However we don't want to force any strict branch naming strategy, it would
be good if you will follow these advices:

1. Choose descriptive names for branches.

    For example don't name the branch `patch-1` or `test1` (which tells nothing about its
    content nor its purpose) when it could be named `remove-unused-method-from-docker-executor`.

1. Use name prefixes:
   - if the branch adds/updates documentation - start its name with `docs/`,
   - if the branch adds a new feature - start its name with `feature/`,
   - if the branch adds a new improvement - start its name with `improvement/`,
   - if the branch fixes a bug - start its name with `fix/` or `bugfix/`.

1. Including issues number in branch name is neither recommended nor discouraged.

    However if you want to link the changes with an issue it's better to
    create the MR from this branch as soon as possible (you can use the `WIP:` prefix
    in the title to prevent from unexpected merges) and link all relevant issues
    in its description.

    Use `Fixes #123, #456` or `Closes #123, #345` or mix of them if it's reasonable.
    Thanks to this the issue will be closed along with merging the MR.

## Documentation

Some documentation tips:

1. Create documentation as late as it's possible, but before the change
   is added into `master`/`X-Y-stable` branch (before the MR is merged).

1. Properly mark features that need modifications in both Runner and GitLab CE/EE.

    When we introduce new features we mostly mark them in the documentation
    with something like:

    > Notice: The feature was introduced in GitLab Runner v1.6.0`

    Most of the times it's enough but sometimes we introduce a change that
    needs to be released in both GitLab Runner and GitLab CE/EE to work,
    eg. support for artifacts expiration.

    In such case it should be properly marked in documentation, so it'll
    be clear for all **what is required** for the feature to work.

    We can mark it like:

    > Notice: The feature was introduced in GitLab Runner v1.6.0 but it
    > needs at least GitLab CE/EE 8.12 to work.

    On GitLab CE/EE side (eg. in API documentation) we would then mark it like:

    > Notice: The endpoint was introduced in GitLab CE/EE 8.12 but it needs
    > at least GitLab Runner v1.6.0 to work.

    If the changes are not released at the same time it would be good to
    mark which version is not released yet:

    > Notice: The endpoint was introduced in GitLab CE/EE 8.12 but it will
    > need at least GitLab Runner v1.7.0 (not released yet) to work.

[runner-milestones]: https://gitlab.com/gitlab-org/gitlab-ci-multi-runner/milestones
