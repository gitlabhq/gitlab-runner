<!-- Follow the documentation workflow https://docs.gitlab.com/ee/development/documentation/workflow.html -->
<!-- Additional information is located at https://docs.gitlab.com/ee/development/documentation/ -->
<!-- To find the designated Tech Writer for the stage/group, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#designated-technical-writers -->

<!-- Mention "documentation" or "docs" in the MR title -->
<!-- For changing documentation location use the "Change documentation location" template -->

## What does this MR do?

<!-- Briefly describe what this MR is about. -->

## Related issues

<!-- Link related issues below. Insert the issue link or reference after the word "Closes" if merging this should automatically close it. -->

## Author's checklist (required)

- [ ] Follow the [Documentation Guidelines](https://docs.gitlab.com/ee/development/documentation/) and [Style Guide](https://docs.gitlab.com/ee/development/documentation/styleguide/index.html).
- If you have `developer` access or higher (for example, GitLab team members or [Core Team](https://about.gitlab.com/community/core-team/) members)
  - [ ] Apply the ~documentation label.
  - [ ] Assign the [designated Technical Writer](https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments).


When applicable:

- [ ] Update the [permissions table](https://docs.gitlab.com/ee/user/permissions.html).
- [ ] Link docs to and from the higher-level index page, plus other related docs where helpful.
- [ ] Add [GitLab's version history note(s)](https://docs.gitlab.com/ee/development/documentation/styleguide/index.html#gitlab-versions).
- [ ] Add/update the [feature flag section](https://docs.gitlab.com/runner/configuration/feature-flags.html).
- [ ] If you're changing document headings, search `docs/*` for old headings and replace them with the new ones to [avoid broken anchors](https://docs.gitlab.com/ee/development/documentation/styleguide/index.html#anchor-links).

## Review checklist

All reviewers can help ensure accuracy, clarity, completeness, and adherence to the [Documentation Guidelines](https://docs.gitlab.com/ee/development/documentation/) and [Style Guide](https://docs.gitlab.com/ee/development/documentation/styleguide/index.html).

**1. Primary Reviewer**

* [ ] Review by a code reviewer or other selected colleague to confirm accuracy, clarity, and completeness. This can be skipped for minor fixes without substantive content changes.

**2. Technical Writer**

- [ ] Technical writer review. If not requested for this MR, must be scheduled post-merge. To request for this MR, assign the writer listed for the applicable [DevOps stage](https://about.gitlab.com/handbook/product/categories/#devops-stages).
  - [ ] Ensure ~"Technical Writing", ~"documentation", and a `docs::` scoped label are added.
  - [ ] Add ~docs-only when the only files changed are under `docs/*`.
  - [ ] Add ~"tw::doing" when starting work on the MR.
  - [ ] Add ~"tw::finished" if Technical Writing team work on the MR is complete but it remains open.

**3. Maintainer**

1. [ ] Review by assigned maintainer, who can always request/require the above reviews. Maintainer's review can occur before or after a technical writer review.
1. [ ] Ensure a release milestone is set.
1. [ ] If there has not been a technical writer review, [create an issue for one using the Doc Review template](https://gitlab.com/gitlab-org/gitlab/issues/new?issuable_template=Doc%20Review).

/label ~documentation ~"devops::verify" ~"group::runner" ~"Category:Runner"
