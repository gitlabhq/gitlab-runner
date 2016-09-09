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
- [Documentation](#documentation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Release roadmap

Starting with `v1.4` GitLab Runner is released on the 22th day of each month - together
with GitLab CE and GitLab EE projects.

### Stable release timeline

- 12th day of a month:
    - tag first RC version on `master` branch
    - deploy the RC version to `docker-ci-X.gitlap.com`
      (each next RC version should be deployed to above hosts)

- 17th day of a month:
    - tag next RC version on `master` branch
    - deploy the current RC version to `shared-runners-manager-X.gitlab.com`
      (each next RC version should be deployed to above hosts)

- 20th day of a month:
    - close the _new features_ window.

      From now on we're merging only features that are ready and
      features that are _almost ready_ but are waiting for
      bugfixes/documentation

      > There is nothing bad in moving a feature to the next release at this stage,
      > if it's still not _production ready_!

- 21th day of a month:
    - tag last RC version

- 22th day of a month:
    - tag a stable version on `master` branch
    - deploy stable version to `docker-ci-X.gitlap.com` and `shared-runners-manager-X.gitlab.com`
    - create the `X-Y-stable` on the current master
    - increase version number in `VERSION` file
    - add a new entry to the `CHANGELOG` file
    - open the _new features_ window
    - announce the new release in _GitLab's release blog post_
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

- start work from `master` branch and open a MR into `master` branch
- assign the MR to a milestone related to the currently upcoming release
- add the `CHANGELOG` entry to a related version
- if feature is planned for the current release (confirmed by the
  assigned milestone) - merge branch into master

### Patch releases

- if bug exists in the currently upcoming version:
  - start work from `master` branch and open a MR into `master` branch
  - assign the MR to a milestone related to the oldest version in which the bug exists
  - assign the `pick-into-stable` label
  - continue to work like with the _Stable release_
  - after branch is merged - cherry-pick the merge commit to each
    `X-Y-stable` branch starting from the branch related to the assigned
    milestone up to the latest release
- if bug doesn't exist in the currently upcoming version:
  - start work from `X-Y-stable` of the most recent version where the
    bug exists and open a MR into this branch
  - assign the MR to a milestone related to the oldest version in which the bug exists
  - assign the `pick-into-stable` label
  - continue to work like with the _Stable release_
  - after branch is merged - cherry-pick the merge commit to each
    `X-Y-stable` branch starting from the branch related to the assigned
    milestone up to the latest release before the MR target
- while cherry-picking add the `CHANGELOG` entry to each patch version
- for each `X-Y-stable` branch - if the release should be published -
  create the `vX.Y.patch` tag
- add created patch versions `CHANGELOG` entries into `CHANGELOG` file
  on the `master` branch

## Documentation

Some documentation tips:

1. Create documentation as late as it's possible, but before the change
   is added into `master`/`X-Y-stable` branch (before the MR is merged).

1. How to mark features that need modifications in both Runner and GitLab CE/EE???
    > Need to describe this

[runner-milestones]: https://gitlab.com/gitlab-org/gitlab-ci-multi-runner/milestones
