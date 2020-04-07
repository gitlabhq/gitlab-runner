<!--
  Update the title of this issue to: Trainee BE maintainer (GitLab Runner) - [full name]
-->

## Basic setup

1. [ ] Read the [Becoming a maintainer for one of Runner team projects](https://about.gitlab.com/handbook/engineering/development/ci-cd/verify/runner/#becoming-a-maintainer-for-one-of-our-projects).
1. [ ] Read the [code review page in the handbook](https://about.gitlab.com/handbook/engineering/workflow/code-review/) and the [code review guidelines](https://docs.gitlab.com/ee/development/code_review.html).
1. [ ] Understand [how to become a maintainer](https://about.gitlab.com/handbook/engineering/workflow/code-review/#how-to-become-a-maintainer).
1. [ ] Add yourself as a [trainee maintainer](https://about.gitlab.com/handbook/engineering/workflow/code-review/#trainee-maintainer) on the [team page](https://gitlab.com/gitlab-com/www-gitlab-com/blob/master/data/team.yml).
1. [ ] Ask your manager to set up a check in on this issue every six weeks or so.

## Working towards becoming a maintainer

There is no checklist here, only guidelines. There is no specific timeline on
this, but historically most backend trainee maintainers have become maintainers
five to seven months after starting their training.

You are free to discuss your progress with your manager or any
maintainer at any time. As in the list above, your manager should review
this issue with you roughly every six weeks; this is useful to track
your progress, and see if there are any changes you need to make to move
forward.

It is up to you to ensure that you are getting enough MRs to review, and of
varied types. All engineers are reviewers, so you should already be receiving
regular reviews from Reviewer Roulette. You could also seek out more reviews
from your team, or #backend Slack channels.

Your reviews should aim to cover maintainer responsibilities as well as reviewer
responsibilities. Your approval means you think it is ready to merge.

After each MR is merged or closed, add a discussion to this issue using this
template:

```markdown
### (Merge request title): (Merge request URL)

During review:

- (List anything of note, or a quick summary. "I suggested/identified/noted...")

Post-review:

- (List anything of note, or a quick summary. "I missed..." or "Merged as-is")

(Maintainer who reviewed this merge request) Please add feedback, and compare
this review to the average maintainer review.
```

**Note:** Do not include reviews of security MRs because review feedback might
reveal security issue details.

## When you're ready to make it official

When reviews have accumulated, you can confidently address the majority of the MR's assigned to you,
and recent reviews consistently fulfill maintainer responsibilities, then you can propose yourself as a new maintainer
for the relevant application.

Remember that even when you are a maintainer, you can still request help from other maintainers if you come across an MR
that you feel is too complex or requires a second opinion.

1. [ ] Create a merge request for [team page](https://gitlab.com/gitlab-com/www-gitlab-com/blob/master/data/team.yml) proposing yourself as a maintainer for the relevant application, assigned to your manager.
1. [ ] Ask a maintainer to add you as an Owner to the relevant maintainers list in <https://gitlab.com/gitlab-com/runner-maintainers>
1. [ ] Keep reviewing, start merging :metal:

/label ~"trainee maintainer" ~"devops::verify" ~"group::runner"
