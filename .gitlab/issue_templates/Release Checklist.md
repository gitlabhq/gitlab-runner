# GitLab Runner X.Y release checklist

GitLab Runner Release manager: **MENTION_HERE**

Release blog post MR: **LINK_HERE**

Runner entries need to be added to blog post until: **DEADLINE_FOR_RUNNER_ITEMS_ADDITION_INTO_BLOG_POST_HERE**

Technical description of the release, with commands examples, can be found at:
https://gitlab.com/gitlab-org/gitlab-runner/blob/master/docs/release_process/how_to_release_runner.md

## Before 7th

- [ ] chose a release manager
- [ ] link release blog post's MR
- [ ] set deadline for _add entries to release blog post_

      Please check what deadline is set for `General Contributions` section in the release blog post
      Merge Request. It should be 6th working day before the 22nd. In that case we can set our
      deadline for 7th working day before 22nd, however if the deadline from the MR is earlier, then
      use the eraliest one.

- [ ] Update the `X.Y.` and `X-Y-` to a specific release version
- [ ] Add the ~release label to the issue
- [ ] Add the %X.Y milestone to the issue

## First working day after 7th - **vX.Y.0-rc.1 release**

- [ ] check if Pipeline for `master` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/master/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/master)
    - [ ] add all required fixes to make `master` Pipeline passing
- [ ] add **vX.Y.0-rc.1** CHANGELOG entries
- [ ] tag and push **vX.Y.0-rc.1**
- [ ] create and push `X-Y-stable` branch
- [ ] checkout to `master`, update `VERSION` file to `X.Y+1.0` and push `master`
- [ ] deploy **vX.Y.0-rc.1** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)

_New features_ window is closed - things not merged into `master` up to
this day, will be released with next release.

## 7 working days before 22th (**DEADLINE_FOR_RUNNER_ITEMS_ADDITION_INTO_BLOG_POST_HERE**)

- [ ] add entries to release blog post
    - [ ] add release entry:

    Add description to the `SECONDARY FEATURES` list using following template:

    ```markdown
    - name: GitLab Runner X.Y
      available_in: [ce, ees, eep]
      documentation_link: 'https://docs.gitlab.com/runner'
      documentation_text: "Read through the documentation on GitLab Runner"
      description: |
        We're also releasing GitLab Runner X.Y today! GitLab Runner is the open source project
        that is used to run your CI/CD jobs and send the results back to GitLab.

        ##### Most interesting changes:

        * __Title__ ([merge request](https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/__ID__))

        List of all changes can be found in GitLab Runner's [CHANGELOG](https://gitlab.com/gitlab-org/gitlab-runner/blob/vX.Y.0/CHANGELOG.md).
    ```

## At 20th - next RC release

At this day we should release an RC version, if there was no RC recently - especially
if the only RC version was the _RC1_ released near 7th day of month.

- [ ] check if Pipeline for `X-Y-stable` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/X-Y-stable/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/X-Y-stable)
    - [ ] add all required fixes to make `X-Y-stable` Pipeline passing
- [ ] add **vX.Y.0-rc.Z** CHANGELOG entries
- [ ] tag **vX.Y.0-rc.Z**
- [ ] deploy **vX.Y.0-rc.Z** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)

## At 22th - the release day

- [ ] Before 12:00 UTC
    - [ ] add last entries to changelog
    - [ ] merge all RC.x CHANGELOG entries into release entry
    - [ ] tag stable version
- [ ] Before 15:00 UTC
    - [ ] deploy stable version to all production Runners


**RC release template**

There should be at least one RC version between RC.1 and stable release. If there are any
important changes merged into stable branch (like bug/security fixes) the RC should be
prepared and deployed as soon as possible. For a less important changes (documentation,
simple fixes of typos etc.) the RC can wait a little.

When deciding to release a new RC version, please update the checklist using the following
template:

```markdown
## At _day here_ - **vX.Y.0-rc.Z** release

- [ ] check if Pipeline for `X-Y-stable` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/X-Y-stable/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/X-Y-stable)
    - [ ] add all required fixes to make `X-Y-stable` Pipeline passing
- [ ] add **vX.Y.0-rc.Z** CHANGELOG entries
- [ ] tag **vX.Y.0-rc.Z**
- [ ] deploy **vX.Y.0-rc.Z** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)
```
