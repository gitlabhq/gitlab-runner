## What does this MR do?

<!-- Briefly describe what this MR is about. -->

## Related issues

<!-- Link related issues below. -->

## Author's checklist

- [ ] Optional. Consider taking [the GitLab Technical Writing Fundamentals course](https://university.gitlab.com/courses/gitlab-technical-writing-fundamentals).
- [ ] Follow the:
  - [Documentation process](https://docs.gitlab.com/development/documentation/workflow/).
  - [Documentation guidelines](https://docs.gitlab.com/development/documentation/).
  - [Style Guide](https://docs.gitlab.com/development/documentation/styleguide/).
- [ ] If you're adding or changing the main heading of the page (H1), ensure that the [product availability details](https://docs.gitlab.com/development/documentation/styleguide/availability_details/) are added.
- [ ] If you are a GitLab team member, [request a review](https://docs.gitlab.com/development/code_review/#dogfooding-the-reviewers-feature) based on:
  - The documentation page's [metadata](https://docs.gitlab.com/development/documentation/metadata/).
  - The [associated Technical Writer](https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments).

If you are a GitLab team member and only adding documentation, do not add any of the following labels:

- `~"frontend"`
- `~"backend"`
- `~"type::bug"`
- `~"database"`

These labels cause the MR to be added to code verification QA issues.

## Reviewer's checklist

Documentation-related MRs should be reviewed by a Technical Writer for a non-blocking review, based on [Documentation Guidelines](https://docs.gitlab.com/development/documentation/) and the [Style Guide](https://docs.gitlab.com/development/documentation/styleguide/).

If you aren't sure which tech writer to ask, use [roulette](https://gitlab-org.gitlab.io/gitlab-roulette/?sortKey=stats.avg30&order=-1&hourFormat24=true&visible=maintainer%7Cdocs) or ask in the [#docs](https://gitlab.slack.com/archives/C16HYA2P5) Slack channel.

- [ ] If the content requires it, ensure the information is reviewed by a subject matter expert.
- Technical writer review items:
  - [ ] Ensure docs metadata is present and up-to-date.
  - [ ] Ensure the appropriate [labels](https://docs.gitlab.com/development/documentation/workflow/#labels) are added to this MR.
  - [ ] Ensure a release milestone is set.
  - If relevant to this MR, ensure [content topic type](https://docs.gitlab.com/development/documentation/topic_types/) principles are in use, including:
    - [ ] The headings should be something you'd do a Google search for. Instead of `Default behavior`, say something like `Default behavior when you close an issue`.
    - [ ] The headings (other than the page title) should be active. Instead of `Configuring GDK`, say something like `Configure GDK`.
    - [ ] Any task steps should be written as a numbered list.
    - If the content still needs to be edited for topic types, you can create a follow-up issue with the ~"docs-technical-debt" label.
- [ ] Review by assigned maintainer, who can always request/require the above reviews. Maintainer's review can occur before or after a technical writer review.

/label ~documentation ~"devops::verify" ~"group::runner" ~"Category:Runner"  ~"type::maintenance" ~"maintenance::refactor"
/assign me
