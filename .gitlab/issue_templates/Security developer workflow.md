<!--

# Read me first!

Create this issue under https://gitlab.com/gitlab-org/security/gitlab-runner

Set the title to: `Description of the original issue`
-->

## Prior to starting the security release work

- [ ] Read the [security process for developers] if you are not familiar with it.
- [ ] Mark this [issue as related] to the [upcoming Security Release Tracking Issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues?scope=all&utf8=%E2%9C%93&state=opened&label_name[]=security&label_name[]=upcoming%20security%20release).
- Fill out the [Links section](#links):
    - [ ] Next to **Issue on GitLab**, add a link to the `gitlab-org/gitlab-runner` issue that describes the security vulnerability.

## Development

- [ ] Run `scripts/security-harness` in your local repository to prevent accidentally pushing to any remote branch besides `gitlab.com/gitlab-org/security`.
- [ ] Create a new branch prefixing it with `security-`.
- [ ] Create a merge request targeting `main` on `gitlab.com/gitlab-org/security/gitlab-runner` and use the [Security Release merge request template].

After your merge request has been approved according to our [approval guidelines] and by a team member of the AppSec team, you're ready to prepare the backports.

## Backports

- [ ] Once the MR is ready to be merged, create MRs targeting the latest 3 stable branches.
   * At this point, it might be easier to squash the commits from the MR into one.
- [ ] Create each MR targeting the stable branch `X-Y-stable`, using the [Security Release merge request template].
   * Every merge request will have its own set of TODOs, so make sure to complete those.
- [ ] On the "Related merge requests" section, ensure all MRs are linked to this issue.
   * This section should only list the merge requests created for this issue: One targeting `main` and the 3 backports.

## Documentation and final details

- [ ] Ensure the [Links section](#links) is completed.
- [ ] Add the GitLab Runner [versions](https://gitlab.com/gitlab-org/release/docs/-/blob/master/general/security/developer.md#versions-affected) and editions affected to the [details section](#details).
  * The Git history of the files affected may help you associate the issue with a [release](https://about.gitlab.com/releases/).
- [ ] Fill in any upgrade notes that users may need to take into account in the [details section](#details).
- [ ] Add Yes/No and further details if needed to the migration and settings columns in the [details section](#details).
- [ ] Add the nickname of the external user who found the issue (and/or HackerOne profile) to the Thanks row in the [details section](#details).

## Summary

### Links

| Description | Link |
| -------- | -------- |
| Issue on [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/issues) | #TODO  |

### Details

| Description | Details | Further details|
| -------- | -------- | -------- |
| Versions affected | X.Y  | |
| Upgrade notes | | |
| GitLab Runner config updated | Yes/No| |
| Thanks | | |

[security process for developers]: https://gitlab.com/gitlab-org/release/docs/blob/master/general/security/developer.md
[security Release merge request template]: https://gitlab.com/gitlab-org/security/gitlab-runner/blob/main/.gitlab/merge_request_templates/Security%20Release.md
[approval guidelines]: https://docs.gitlab.com/ee/development/code_review.html#approval-guidelines
[issue as related]: https://docs.gitlab.com/ee/user/project/issues/related_issues.html#adding-a-related-issue

/label ~security ~"Category:Runner" ~"devops::verify" ~"group::runner"

