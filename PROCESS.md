## GitLab core team & GitLab Inc. contribution process

---

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Be kind](#be-kind)
- [Feature freeze on the 7th for the release on the 22nd](#feature-freeze-on-the-7th-for-the-release-on-the-22nd)
  - [Between the 1st and the 7th](#between-the-1st-and-the-7th)
    - [What happens if these deadlines are missed?](#what-happens-if-these-deadlines-are-missed)
  - [On the 7th](#on-the-7th)
  - [After the 7th](#after-the-7th)
  - [Asking for an exception](#asking-for-an-exception)
- [Bugs](#bugs)
  - [Regressions](#regressions)
  - [Managing bugs](#managing-bugs)
- [Supported releases](#supported-releases)
- [Releasing GitLab Runner](#releasing-gitlab-runner)
  - [Security release](#security-release)
- [Renew expired GPG key](#renew-expired-gpg-key)
- [Copy & paste responses](#copy--paste-responses)
  - [Improperly formatted issue](#improperly-formatted-issue)
  - [Issue report for old version](#issue-report-for-old-version)
  - [Support requests and configuration questions](#support-requests-and-configuration-questions)
  - [Code format](#code-format)
  - [Issue fixed in newer version](#issue-fixed-in-newer-version)
  - [Improperly formatted merge request](#improperly-formatted-merge-request)
  - [Accepting merge requests](#accepting-merge-requests)
  - [Only accepting merge requests with green tests](#only-accepting-merge-requests-with-green-tests)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

---

## Be kind

Be kind to people trying to contribute. Be aware that people may be a non-native
English speaker, they might not understand things or they might be very
sensitive as to how you word things. Use Emoji to express your feelings (heart,
star, smile, etc.). Some good tips about code reviews can be found in our
[Code Review Guidelines].

[Code Review Guidelines]: https://docs.gitlab.com/ce/development/code_review.html

## Feature freeze on the 7th for the release on the 22nd

After 7th at 23:59 (Pacific Time Zone) of each month, stable branch and RC1
of the upcoming release (to be shipped on the 22nd) is created and deployed to GitLab.com.
The stable branch is frozen at the most recent "qualifying commit" on `main`.
A "qualifying commit" is one that is pushed before the feature freeze cutoff time
and that passes all CI jobs (green pipeline).

Merge requests may still be merged into `main` during this
period, but they will go into the _next_ release, unless they are manually
cherry-picked into the stable branch.

By freezing the stable branches 2 weeks prior to a release, we reduce the risk
of a last minute merge request potentially breaking things.

Any release candidate that gets created after this date can become a final
release, hence the name release candidate.

### Between the 1st and the 7th

These types of merge requests for the upcoming release need special consideration:

- **Large features**: a large feature is one that is highlighted in the kick-off
  and the release blogpost; typically this will have its own channel in Slack
  and a dedicated team with front-end, back-end, and UX.
- **Small features**: any other feature request.

It is strongly recommended that **large features** be with a maintainer **by the
1st**. This means that:

- There is a merge request (even if it's WIP).
- The person (or people, if it needs a frontend and backend maintainer) who will
  ultimately be responsible for merging this have been pinged on the MR.

It's OK if merge request isn't completely done, but this allows the maintainer
enough time to make the decision about whether this can make it in before the
freeze. If the maintainer doesn't think it will make it, they should inform the
developers working on it and the Product Manager responsible for the feature.

The maintainer can also choose to assign a reviewer to perform an initial
review, but this way the maintainer is unlikely to be surprised by receiving an
MR later in the cycle.

It is strongly recommended that **small features** be with a reviewer (not
necessarily a maintainer) **by the 3rd**.

Most merge requests from the community do not have a specific release
target. However, if one does and falls into either of the above categories, it's
the reviewer's responsibility to manage the above communication and assignment
on behalf of the community member.

Every new feature or change should be shipped with its corresponding documentation
in accordance with the
[documentation process](https://docs.gitlab.com/ee/development/documentation/feature-change-workflow.html)
and [structure](https://docs.gitlab.com/ee/development/documentation/structure.html) guides.
Note that a technical writer will review all changes to documentation. This can occur
in the same MR as the feature code, but [if there is not sufficient time or need,
it can be planned via a follow-up issue for doc review](https://docs.gitlab.com/ee/development/documentation/feature-change-workflow.html#1-product-managers-role),
and another MR, if needed. Regardless, complete docs must be merged with code by the freeze.

#### What happens if these deadlines are missed?

If a small or large feature is _not_ with a maintainer or reviewer by the
recommended date, this does _not_ mean that maintainers or reviewers will refuse
to review or merge it, or that the feature will definitely not make it in before
the feature freeze.

However, with every day that passes without review, it will become more likely
that the feature will slip, because maintainers and reviewers may not have
enough time to do a thorough review, and developers may not have enough time to
adequately address any feedback that may come back.

A maintainer or reviewer may also determine that it will not be possible to
finish the current scope of the feature in time, but that it is possible to
reduce the scope so that something can still ship this month, with the remaining
scope moving to the next release. The sooner this decision is made, in
conversation with the Product Manager and developer, the more time there is to
extract that which is now out of scope, and to finish that which remains in scope.

For these reasons, it is strongly recommended to follow the guidelines above,
to maximize the chances of your feature making it in before the feature freeze,
and to prevent any last minute surprises.

### On the 7th

Merge requests should still be complete, following the [definition of
done](https://docs.gitlab.com/ee/development/contributing/merge_request_workflow.html#definition-of-done).

If a merge request is not ready, but the developers and Product Manager
responsible for the feature think it is essential that it is in the release,
they can [ask for an exception](#asking-for-an-exception) in advance. This is
preferable to merging something that we are not confident in, but should still
be a rare case: most features can be allowed to slip a release.

### After the 7th

Once the stable branch is frozen, the only MRs that can be cherry-picked into
the stable branch are:

- Fixes for [regressions](#regressions) where the affected version `xx.x` in `regression:xx.x` is the current release. See [Managing bugs](#managing-bugs) section.
- Fixes for security issues.
- Fixes or improvements to automated QA scenarios.
- [Documentation improvements](https://docs.gitlab.com/ee/development/documentation/workflow.html) for feature changes made in the same release, though initial docs for these features should have already been merged by the freeze, as required.
- New or updated translations (as long as they do not touch application code).
- Changes that are behind a feature flag and have the ~"feature flag" label.

During the feature freeze all merge requests that are meant to go into the
upcoming release should have the correct milestone assigned _and_ the
`Pick into X.Y` label where `X.Y` is equal to the milestone, so that release
managers can find and pick them.
Merge requests without this label will not be picked into the stable release.

For example, if the upcoming release is `10.2.0` you will need to set the
`Pick into 10.2` label.

Fixes marked like this will be shipped in the next RC (before the 22nd), or the
next patch release.

If a merge request is to be picked into more than one release it will need one
`Pick into X.Y` label per release where the merge request should be back-ported
to. For example:

- `Pick into 10.1`
- `Pick into 10.0`
- `Pick into 9.5`

### Asking for an exception

If you think a merge request should go into an RC or patch even though it does not meet these requirements,
you can ask for an exception to be made, by opening an isssue and
tagging the Release Manager.

To find out who the current Release Manager is find the latest release
checklist inside the issue tracker with the ~release label.  For example
[this issues](https://gitlab.com/gitlab-org/gitlab-runner/issues/4333)
specifies that `@tmaczukin` is the release manager for 12.0.

## Bugs

A ~bug is a defect, error, failure which causes the system to behave incorrectly or prevents it from fulfilling the product requirements.

The level of impact of a ~bug can vary from blocking a whole functionality
or a feature usability bug. A bug should always be linked to a severity level.
Refer to our [severity levels](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#severity-labels)

Whether the bug is also a regression or not, the triage process should start as soon as possible.
Ensure that the Engineering Manager and/or the Product Manager for the relative area is involved to prioritize the work as needed.

### Regressions

A ~regression implies that a previously **verified working functionality** no longer works.
Regressions are a subset of bugs. We use the ~regression label to imply that the defect caused the functionality to regress.
The label tells us that something worked before and it needs extra attention from Engineering and Product Managers to schedule/reschedule.

The regression label does not apply to ~bugs for new features for which functionality was **never verified as working**.
These, by definition, are not regressions.

A regression should always have the `regression:xx.x` label on it to designate when it was introduced.

Regressions should be considered high priority issues that should be solved as soon as possible, especially if they have severe impact on users.

### Managing bugs

**Prioritization:** We give higher priority to regressions on features that worked in the last recent monthly release and the current release candidates, for example:

- A regression which worked in the **Last monthly release**
  - **Example:** In 11.0 we released a new `feature X` that is verified as working. Then in release 11.1 the feature no longer works, this is regression for 11.1. The issue should have the `regression:11.1` label.
  - *Note:* When we say `the last recent monthly release`, this can refer to either the version currently running on GitLab.com, or the most recent version available in the package repositories.
- A regression which worked in the **Current release candidates**
  - **Example:** In 11.1-RC3 we shipped a new feature which has been verified as working. Then in 11.1-RC5 the feature no longer works, this is regression for 11.1. The issue should have the `regression:11.1` label.
  - *Note:* Because GitLab.com runs release candidates of new releases, a regression can be reported in a release before its 'official' release date on the 22nd of the month.

When a bug is found:

1. Create an issue describing the problem in the most detailed way possible.
1. If possible, provide links to real examples and how to reproduce the problem.
1. Label the issue properly, using the [team label](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#team-labels),
   the [subject label](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#subject-labels)
   and any other label that may apply in the specific case
1. Notify the respective Engineering Manager to evaluate and apply the [Severity label](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#severity-labels) and [Priority label](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#priority-labels).
The counterpart Product Manager is included to weigh-in on prioritization as needed.
1. If the ~bug is **NOT** a regression:
   1. The Engineering Manager decides which milestone the bug will be fixed. The appropriate milestone is applied.
1. If the bug is a ~regression:
   1. Determine the release that the regression affects and add the corresponding `regression:xx.x` label.
      1. If the affected release version can't be determined, add the generic ~regression label for the time being.
   1. If the affected version `xx.x` in `regression:xx.x` is the **current release**, it's recommended to schedule the fix for the current milestone.
      1. This falls under regressions which worked in the last release and the current RCs. More detailed explanations in the **Prioritization** section above.
   1. If the affected version `xx.x` in `regression:xx.x` is older than the **current release**
      1. If the regression is an ~S1 severity, it's recommended to schedule the fix for the current milestone. We would like to fix the highest severity regression as soon as we can.
      1. If the regression is an ~S2, ~S3 or ~S4 severity, the regression may be scheduled for later milestones at the discretion of the Engineering Manager and Product Manager.

## Supported releases

The _last three releases_ are supported. Meaning if the latest version
is `11.11`, the supported versions are `11.11`, `11.10`, `11.9`

Each support requests for previous versions will be closed with
a ~wontfix label.

**What is supported?**

By the _release support_ we understand:

- fixes for security bugs
- fixes for other bugs
- requests for documentation
- questions of type _"How can I ...?"_ related to a supported version

Proposals for new features or improvements are welcome, but will be not
prepared for supported releases. Instead - if we decide to implement
them - they will be planned for one of the upcoming releases.

## Releasing GitLab Runner

All the technical details of how the Runner is released can be found in
the [Release
Checklist](https://gitlab.com/gitlab-org/ci-cd/runner-release-helper/-/tree/main/templates/issues)
which is split into multiple templates.

### Security release

In addition to the Release Manager, the security process involves many
other people and roles.

We follow the GitLab Security process with the following exceptions.

- [Overview](https://gitlab.com/gitlab-org/release/docs/-/blob/master/general/security/process.md)
    - To create the release task issue, we use a different command than
      `/chatops run release prepare --security`.
- [Developer](https://gitlab.com/gitlab-org/release/docs/-/blob/master/general/security/developer.md)
    - For mentions of `gitlab-org/gitlab` assume `gitlab-org/gitlab-runner` and
      for `gitlab-org/security/gitlab` assume `gitlab-org/security/gitlab-runner`.
    - We have our own [Security Implementation
      Issue](https://gitlab.com/gitlab-org/release/docs/-/blob/master/general/security/developer.md#security-implementation-issue)
      that can be found
      [here](https://gitlab.com/gitlab-org/security/gitlab-runner/-/issues/new?issuable_template=Security+developer+workflow).
- [Release Manager](https://gitlab.com/gitlab-org/release/docs/-/blob/master/general/security/release-manager.md)
    - To create the security release task, run this command:

      ```shell
      # Using rrhelper https://gitlab.com/gitlab-org/ci-cd/runner-release-helper
      # $LINK_TO_MAIN_RELEASE_ISSUE can found in the #releases slack channel
      rrhelper create-security-release-checklist --runner-tags 13.2.2,13.1.2,13.0.2 --helm-tags 0.19.2,0.18.2,0.17.2 --project-id 250833 --security-url $LINK_TO_MAIN_RELEASE_ISSUE`
      ```

- [Security Engineer](https://gitlab.com/gitlab-org/release/docs/-/blob/master/general/security/security-engineer.md)
    - The Runner Application Security Engineer part is listed [here](https://about.gitlab.com/handbook/product/product-categories/#runner-group).

## Renew expired GPG key

We sign all of our packages with GPG, and this key is short-lived (1
year) so every year we have to renew it. For this, we have a tool called
[Key expiration
wrapper](https://gitlab.com/gitlab-org/ci-cd/runner-tools/key-expiration-wrapper)
that documents and automates the process.

## Copy & paste responses

### Improperly formatted issue

```
Thank you for the issue report. Please reformat your issue to conform to the
[contribution guidelines](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#issue-tracker-guidelines).
```

### Issue report for old version

```
Thank you for the issue report. We only support issues for the latest stable version of GitLab.
I'm closing this issue, however if you still experience this problem in the latest stable version,
please open a new issue (and please reference the old issue(s)).
Make sure to also include the necessary debugging information conforming to the issue tracker
guidelines found in our [contribution guidelines](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#issue-tracker-guidelines).
```

### Support requests and configuration questions

```
Thank you for your interest in GitLab. We don't use the issue tracker for support
requests and configuration questions. Please check our
[Support](https://about.gitlab.com/support/) page to see all of the available
support options. Also, have a look at the [contribution guidelines](https://docs.gitlab.com/ee/development/contributing/index.html)
for more information.

You can read more about this policy in our
[README.md](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/README.md#closing-issues)
```

### Code format

```
Please enclose console output, logs, and code in backticks (`` ` ``), as it's
very hard to read otherwise. For more information, read our
[guide on code and codeblocks in markdown](https://docs.gitlab.com/ee/development/documentation/styleguide/index.html#code-blocks)
```

### Issue fixed in newer version

```
Thank you for the issue report. This issue has already been fixed in newer versions of GitLab.
Due to the size of this project and our limited resources we are only able to support the
latest stable release as outlined in our [contribution guidelines](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html).
In order to get this bug fix and enjoy many new features please
[upgrade](https://gitlab.com/gitlab-org/gitlab-ce/tree/master/doc/update).
If you still experience issues at that time, please open a new issue following our issue
tracker guidelines found in the [contribution guidelines](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#issue-tracker-guidelines).
```

### Improperly formatted merge request

```
Thanks for your interest in improving the GitLab codebase!
Please update your merge request according to the [contribution guidelines](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/development/contributing/merge_request_workflow.md#merge-request-guidelines).
```

### Accepting merge requests

```
Is there an issue on the
[issue tracker](https://gitlab.com/gitlab-org/gitlab-ce/issues) that is
similar to this? Could you please link it here?
Please be aware that new functionality that is not marked
[`Accepting merge requests`](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#label-for-community-contributors)
might not make it into GitLab.
```

### Only accepting merge requests with green tests

```
We can only accept a merge request if all the tests are green. I've just
restarted the build. If the tests are still not green after this restart and
you're sure that is does not have anything to do with your code changes, please
rebase with main to see if that solves the issue.
```
