---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Reviewing GitLab Runner

This document contains rules and suggestions for GitLab Runner project reviewers.

## Reviewing tests coverage reports

In the GitLab Runner project, we have a lot of code. Unfortunately, the code coverage is not comprehensive.
Currently, (early 2019), the coverage is on the level of ~55%.

While adding tests to a legacy code is a hard task, we should ensure that new code that is being
added to the project has good tests coverage. Code reviewers are encouraged to look on the
coverage reports and ensure new code is covered.

We should aim for as much test coverage for new code as possible. Defining the level of
required coverage for a specific change is left for the reviewer judgment. Sometimes 100% coverage
will be something simple to achieve. Sometimes adding code with only 20% of the coverage will be
realistic and will ensure that the most important things are being tested. Dear reviewer - chose wisely :)

Getting back to the technical details...

The GitLab Runner CI/CD pipeline helps us here and provides the coverage reports in HTML format, for tests
executed in regular (`count`) and race (`atomic`) modes.

We have two types of the reports: containing `.race` and `.regular` as part of the file name.
The files are tracking output of `go test` command executed with coverage options. The `.race.` files
contain sources and reports for tests started with `-race` flag, while the `.regular.` files are sources
and reports for tests started without this option.

For those who are interested in details, the `-race` tests are using `atomic` coverage mode, while the standard
tests are using `count` coverage mode.

For our case, the `coverage/coverprofile.regular.html` file is what we should look at. `.race.` tests can fail
in race condition situations (this is why we're executing them) and currently we have several of them that
are constantly failing. This means that the coverage profile may not be full.

The `.regular.` tests, instead, should give us the full overview of what's tested inside our code.

To view a code coverage report for a merge request:

1. In the merge request's **Overview** tab, under the pipeline
   result, click on **View exposed artifact** to expand the section.

1. Click on **Code Coverage**.

1. Use the artifact browser to navigate to the `out/coverage/`
   directory. For example,
   `https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/browse/out/coverage/`.
   This directory will always contain six files - three `.race.` files
   and three `.regular.` files.

   For reviewing changes, we're mostly interested in looking at the `.regular.` HTML
   report (the `coverprofile.regular.html` file). As you can see, all files are visible
   as external links, so for our example we will open
   `https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/file/out/coverage/coverprofile.regular.html`
   which will redirect us to
   `https://gitlab-org.gitlab.io/-/gitlab-runner/-/jobs/172824578/artifacts/out/coverage/coverprofile.regular.html`
   where the report is stored.

The coverage data should be also
[visible in the merge request UI](https://docs.gitlab.com/ee/ci/testing/test_coverage_visualization.html).

## Reviewing the merge request title

Because we generate [`CHANGELOG.md`](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/CHANGELOG.md) entries
from the merge request titles, making sure that the title is valid and informative is a part
of the reviewer and maintainer's responsibilities.

Before merging a merge request, check the title and update it if you think it will not be clear in the
`CHANGELOG.md` file. Keep in mind that the changelog will have only this one line, without the merge
request description, discussion or diff that provide more context.

As an example, look at <https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1812> and compare:

- `yml to yaml` - which is the original title and was added to changelog with our script,
- `Fix values.yaml file name in documentation` - which is what I've updated it to in the changelog.

What will `yml to yaml` tell a GitLab Runner administrator if they review the changelog before updating
to a newer version? Does it show the risks behind the update, the implemented behavior changes, a new
behavior/features that were added? Keep these questions in mind when reviewing the merge request and its title.

Contributors may not be aware of the above information, and that their titles
may not match our requirements. Try to educate the contributor about this.

In the end, it's your responsibility to verify and update the title **before the merge request is merged**.

## Reviewing the merge request labels

We use labels assigned to merge requests to group changelog entries in different groups and define
some special features of individual entries.

For changelog generation we're using our own [Changelog generator](https://gitlab.com/gitlab-org/ci-cd/runner-tools/changelog-generator).
The tool is using [a configuration file](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/.gitlab/changelog.yml)
that is committed to the GitLab Runner repository.

There are few important things that the reviewer should know about Changelog generator:

- GitLab Changelog analyzes merge request labels in the order in which `label_matchers` are defined.
  First matched scope is used for the analyzed merge request.

  For example, if there would be two merge request - first one containing labels `security` and `bug`, second
  one containing only the `bug` label - and there would be three matchers defined in this
  order: `[security, bug] -> [security] -> [bug]`, then the first merge request would be added to the scope matched
  by `[security, bug]` (so the first defined on the list) and the second merge request would be added to
  the scope matched by `[bug]` (so the last defined scope on the list).

- Merge requests labeled with labels defined at `authorship_labels` will be added to the changelog with the
  author's username added at the end. All `authorship_labels` labels need to be added to the merge request
  for it to be marked in this way.

- Merge requests labeled with labels defined at `skip_changelog_labels` will be skipped in the changelog. All
  `skip_changelog_labels` labels need to be added to the merge request for it to be skipped.

- Merge request not matching any of the defined `label_matchers` are added to the `Other changes` scope
  bucket.

Having all of that in mind, please follow these few rules when merging the merge request:

- Any merge request related to how GitLab Runner or its parts are distributed should be labeled with the
  `runner-distribution` label.

- Any merge request that touches security - no matter if it's a new feature or a bug fix - should have the
  `security` label. All merge requests that are not `feature::addition` will be then added to the security
  scope.

- Any bug fix merge request should have the `bug` label.

- In most merge requests that are not documentation update only or explicitly a bug fix, make sure that one of the
  `feature::` or `tooling::` labels is added. This will help us sort the changelog entries properly.

- `documentation` label is added automatically when the Technical Writing review is done. **Even when the merge
  request updates more than only documentation**. If the merge request has only the `documentation` label and
  doesn't have any other label matching any of the defined `label_matchers` - double check that the merge request
  updates the documentation only. **Otherwise use one of the specific labels matching the type of the change
  that is being added!**

- When you revert a change that was merged during the same release cycle, label the original merge request and
  the revert one with labels defined in `skip_changelog_labels`. This will reduce the manual work that release
  manager needs to do when preparing the release. We should not add entries about adding a change and reverting
  the change if both events happened in the same version.

  If the revert merge request reverts something, that was merged to an already release version of GitLab Runner,
  just make sure to label it with the right scope labels. In that case we want to mark the revert in the
  changelog.

- Please also take a moment to read through
  [Engineering metrics data classification](https://about.gitlab.com/handbook/engineering/metrics/#data-classification)
  page, which gives some guidance about when certain labels should be used.

## Summary

Dear reviewer, you've got your sword. Now go fight with the dragons!
