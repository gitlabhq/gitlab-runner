<!--

# README first!

This MR should be created on `gitlab.com/gitlab-org/security/gitlab-runner`.

See [the general developer security release guidelines](https://gitlab.com/gitlab-org/release/docs/blob/master/general/security/developer.md).

-->

## Related issues

<!-- Mention the GitLab Security issue this MR is related to -->

## Developer checklist

- [ ] **In the "Related issues" section, write down the [GitLab Runner Security] issue it belongs to (i.e. `Related to <issue_id>`).**
- [ ] Merge request targets `main`, or a versioned stable branch (`X-Y-stable`).
- [ ] Milestone is set for the version this merge request applies to. A closed milestone can be assigned via [quick actions].
- [ ] Title of this merge request is the same as for all backports.
- [ ] For the MR targeting `main`:
  - [ ] Assign to a reviewer and maintainer, per our [Code Review process].
  - [ ] Ensure it's approved according to our [Approval Guidelines].
  - [ ] Ensure it's approved by an AppSec engineer.
    - If you're unsure who should approve, find the AppSec engineer associated to the issue in the [Canonical repository], or ask #sec-appsec on Slack.
    - [ ] When approving, the AppSec engineer should mention this MR on the [security release tracking issue] in the `gitlab-org/gitlab` project for awareness
  - [ ] Merge request _must_ close the corresponding security issue.
- [ ] Ensure that a backport MR targeting a versioned stable branch (`X-Y-stable`) is approved by a maintainer.

**Note:** Reviewer/maintainer should not be a Release Manager

## Maintainer checklist

- [ ] Correct milestone is applied and the title is matching across all backports.
- [ ] Assign the merge request to the release manager of the [upcoming
  security
  release](https://gitlab.com/gitlab-org/gitlab-runner/-/issues?scope=all&utf8=%E2%9C%93&state=opened&label_name[]=security&label_name[]=upcoming%20security%20release)
  with passing CI pipelines and **when all backports including the MR
  targeting main are ready.**

/label ~security ~"Category:Runner" ~"devops::verify" ~"group::runner"

[GitLab Runner Security]: https://gitlab.com/gitlab-org/security/gitlab-runner
[quick actions]: https://docs.gitlab.com/ee/user/project/quick_actions.html#quick-actions-for-issues-merge-requests-and-epics
[Code Review process]: https://docs.gitlab.com/ee/development/code_review.html
[Approval Guidelines]: https://docs.gitlab.com/ee/development/code_review.html#approval-guidelines
[Canonical repository]: https://gitlab.com/gitlab-org/gitlab-runner
[security release tracking issue]: https://gitlab.com/gitlab-org/gitlab/-/issues/?scope=all&utf8=%E2%9C%93&state=opened&label_name%5B%5D=upcoming%20security%20release

