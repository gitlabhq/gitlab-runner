## Developer Certificate of Origin + License

By contributing to GitLab B.V., You accept and agree to the following terms and
conditions for Your present and future Contributions submitted to GitLab B.V.
Except for the license granted herein to GitLab B.V. and recipients of software
distributed by GitLab B.V., You reserve all right, title, and interest in and to
Your Contributions. All Contributions are subject to the following DCO + License
terms.

[DCO + License](https://gitlab.com/gitlab-org/dco/blob/master/README.md)

All Documentation content that resides under the [docs/ directory](/docs) of this
repository is licensed under Creative Commons:
[CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).

_This notice should stay as the first item in the CONTRIBUTING.md file._

---

## Contribute to GitLab Runner

The following content is an extension of the [GitLab contribution guidelines](https://docs.gitlab.com/ce/development/contributing/index.html).

### How we prioritize MRs from the wider community

Currently we use a system of [scoped labels](https://docs.gitlab.com/ee/user/project/labels.html#scoped-labels-premium) to help us prioritize which MRs our team will review.

| Label | Meaning | Use Cases |
| ---- | ----- | ----- |
| ~"Review::P1" | Highest priority to review. | Indicates a merge request that might solve an urgent pain point for users, contributes to the strategic direction of Runner development as laid out by the Product team, or fixes a critical issue. A hard cap on the number of contributions labelled ~"Review::P1" is set at 3. |
| ~"Review::P2" | Important merge requests. | When a merge request is important, but has lower impact to customers when compared to merge requests labelled ~"Review::P1". |
| ~"Review::P3" | Default priority to review. | All incoming merge requests should default to this. |

### Contributing new features that need new or updated `.gitlab-ci.yml` [keywords](https://docs.gitlab.com/ee/ci/yaml/)

To execute a job, the GitLab instance processes the `gitlab-ci.yml` configuration
and creates a data transfer object, containing only data relevant to a job's
execution, that GitLab Runner then receives.

Because of this workflow, when you add a keyword that affects the execution of a job, you must
make changes in both repositories: GitLab Runner and [GitLab](https://gitlab.com/gitlab-org/gitlab).

When a feature needs changes in both repositories, the GitLab Runner team can accept
a merge request only if the feature has already been accepted for inclusion in the
GitLab repository.

- Reviews in both repositories can happen in parallel.
- The GitLab project will always dictate and have authority over which keywords are added.
- The GitLab project maintainers determine what the behavior will ultimately be.

For this reason, before starting a review in the GitLab Runner project, the team
requires confirmation that a keyword or a change to a keyword is likely to be accepted.
This process helps save time and ensures that we end up with the best solution possible
for the problem being solved.

### Contributing new [executors](https://docs.gitlab.com/runner/#selecting-the-executor)

We are no longer accepting or developing new executors for a few
reasons listed below:

- Some executors require licensed software or hardware that GitLab Inc.
  doesn't have.
- Each new executor brings its own set of problems when it comes to
  testing it properly.
- Adding new executors can add new dependencies, which adds maintenance costs.
- Having a lot of executors adds to maintenance costs.

With GitLab 12.1, we introduced the [custom
executor](https://gitlab.com/gitlab-org/gitlab-runner/issues/2885),
which will provide a way to create an executor of choice.

### Contributing new hardware architectures

We're currently exploring how we can add builds for new and different hardware
architectures. Adding and supporting new architectures brings added levels of
complexity and may require hardware that GitLab Inc. doesn't have access to.

At the current time, new hardware architectures will only be considered if the
following criteria are met:

1. GitLab Inc. must be able to build and test for the new architecture on our Shared Runners on GitLab.com
1. If you add support for the new architecture in the helper image, Docker must also support the architecture upstream

As we explore adding more architectures, other requirements may come up.

We are currently discussing the ability to provide builds for architectures that we
don't have the ability to support and [we welcome contributions to that discussion](https://gitlab.com/gitlab-org/gitlab-runner/issues/4229).

### Submitting Merge Requests

#### Merge Request titles

When submitting a Merge Request please remember that we use the Merge Request titles to generate entries
for the [`CHANGELOG.md`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/CHANGELOG.md) file.
This one line will be the only thing a Runner administrator will see when reviewing
the changelog before deciding if an upgrade should be made or not. The administrator may not check the
MR description, list of changes, or diff which would give more context.

Please make the title clear, concise and informative. A title of `Fixes bug` would not be
acceptable, while `Fix timestamp in docker executor job output` would be acceptable.

### Workflow labels

We have some additional labels plus those defined in [gitlab-ce workflow labels](https://docs.gitlab.com/ce/development/contributing/issue_workflow.html)

- Additional subjects: ~cache, ~executors, ~"git operations"
- OS: ~"os::Linux" ~"os::macOS" ~"os::FreeBSD" ~"os::Windows"
- executor: ~"executor::docker" ~"executor::kubernetes" ~"executor::docker\-machine" ~"executor::shell" ~"executor::parallels" ~"executor::virtualbox"
- For any [follow-up
  issues](https://docs.gitlab.com/ee/development/contributing/issue_workflow.html#technical-debt-in-follow-up-issues)
  created during code review the ~"follow-up" label should be added to
  keep track of it.
