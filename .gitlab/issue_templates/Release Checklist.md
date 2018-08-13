/label ~"CI/CD"
/label ~release
/label ~Deliverable
/milestone %"{{.Major}}.{{.Minor}}"
/assign @{{.ReleaseManagerHandle}}

# GitLab Runner {{.Major}}.{{.Minor}} release checklist

GitLab Runner Release manager: **@{{.ReleaseManagerHandle}}**

Release blog post MR: **gitlab-com/www-gitlab-com!{{.ReleaseBlogPostMR}}**

Runner entries need to be added to blog post until: **{{.ReleaseBlogPostDeadline}}**

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

- [ ] Update the `.Major` and `.Minor` to a specific release version

## First working day after 7th - **v{{.Major}}.{{.Minor}}.0-rc1 release**

- [ ] check if Pipeline for `master` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/master/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/master)
    - [ ] add all required fixes to make `master` Pipeline passing
- [ ] `git checkout master && git pull` in your local working copy!
- [ ] add **v{{.Major}}.{{.Minor}}.0-rc1** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md; git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0-rc1
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0-rc1**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0-rc1 -m "Version v{{.Major}}.{{.Minor}}.0-rc1"; git push origin v{{.Major}}.{{.Minor}}.0-rc1
    ```

- [ ] create and push `{{.Major}}-{{.Minor}}-stable` branch:

    ```bash
    git checkout -b {{.Major}}-{{.Minor}}-stable; git push -u origin {{.Major}}-{{.Minor}}-stable
    ```

- [ ] checkout to `master`, update `VERSION` file to `{{.Major}}.{{inc .Minor}}.0` and push `master`:

    ```bash
    git checkout master; echo -n "{{.Major}}.{{inc .Minor}}.0" > VERSION; git add VERSION; git commit -m "Bump version to {{.Major}}.{{inc .Minor}}.0"; git push
    ```

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0-rc1` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0-rc1/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0-rc1)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0-rc1` passing
- [ ] deploy **v{{.Major}}.{{.Minor}}.0-rc1** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)

_New features_ window is closed - things not merged into `master` up to
this day, will be released with next release.

## 7 working days before 22th (**{{.ReleaseBlogPostDeadline}}**)

- [ ] prepare entries for the release blog post. Items can be generated with `./scripts/changelog2releasepost | less`
- [ ] add release entry:

    Add description to the `SECONDARY FEATURES` list using following template:

    ```markdown
    - name: GitLab Runner {{.Major}}.{{.Minor}}
      available_in: [core, starter, premium, ultimate]
      documentation_link: 'https://docs.gitlab.com/runner'
      documentation_text: "Read through the documentation of GitLab Runner"
      description: |
        We're also releasing GitLab Runner {{.Major}}.{{.Minor}} today! GitLab Runner is the open source project
        that is used to run your CI/CD jobs and send the results back to GitLab.

        ##### Most interesting changes:

        * [__Title__](https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/__ID__)

        List of all changes can be found in GitLab Runner's [CHANGELOG](https://gitlab.com/gitlab-org/gitlab-runner/blob/v{{.Major}}.{{.Minor}}.0/CHANGELOG.md).
    ```

## At 20th - next RC release

At this day we should release an RC version, if there was no RC recently - especially
if the only RC version was the _RC1_ released near 7th day of month.

- [ ] check if Pipeline for `{{.Major}}-{{.Minor}}-stable` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/{{.Major}}-{{.Minor}}-stable/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/{{.Major}}-{{.Minor}}-stable)
    - [ ] add all required fixes to make `{{.Major}}-{{.Minor}}-stable` Pipeline passing
- [ ] `git checkout {{.Major}}-{{.Minor}}-stable && git pull` in your local working copy!
- [ ] add **v{{.Major}}.{{.Minor}}.0-rcZ** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md; git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0-rcZ
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0-rcZ**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0-rcZ -m "Version v{{.Major}}.{{.Minor}}.0-rcZ"; git push origin v{{.Major}}.{{.Minor}}.0-rcZ
    ```

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0-rcZ` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0-rcZ/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0-rcZ)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0-rcZ` passing
- [ ] deploy **v{{.Major}}.{{.Minor}}.0-rcZ** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)

## At 22th - the release day

- [ ] Before 12:00 UTC
    - [ ] `git checkout {{.Major}}-{{.Minor}}-stable && git pull` in your local working copy!
    - [ ] merge all RCx CHANGELOG entries into release entry and commit

    ```bash
    git add CHANGELOG.md && git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0" -S
    ```

    - [ ] tag and push **v{{.Major}}.{{.Minor}}.0**:

        ```bash
        git tag -s v{{.Major}}.{{.Minor}}.0 -m "Version v{{.Major}}.{{.Minor}}.0" && git push origin {{.Major}}-{{.Minor}}-stable v{{.Major}}.{{.Minor}}.0
        ```

    - [ ] checkout to `master` and merge `{{.Major}}-{{.Minor}}-stable` into `master` (only this one time, to update CHANGELOG.md and make the tag available for ./scripts/prepare-changelog-entries.rb in next stable release), push `master`:

        ```bash
        git checkout master; git merge --no-ff {{.Major}}-{{.Minor}}-stable
        # check that the only changes are in CHANGELOG.md
        git push
        ```

- [ ] Before 15:00 UTC
    - [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0)
        - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0` passing
    - [ ] deploy stable version to all production Runners


**RC release template**

There should be at least one RC version between RC1 and stable release. If there are any
important changes merged into stable branch (like bug/security fixes) the RC should be
prepared and deployed as soon as possible. For a less important changes (documentation,
simple fixes of typos etc.) the RC can wait a little.

When deciding to release a new RC version, please update the checklist using the following
template:

```markdown
## At _day here_ - **v{{.Major}}.{{.Minor}}.0-rcZ** release

- [ ] check if Pipeline for `{{.Major}}-{{.Minor}}-stable` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/{{.Major}}-{{.Minor}}-stable/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/{{.Major}}-{{.Minor}}-stable)
    - [ ] add all required fixes to make `{{.Major}}-{{.Minor}}-stable` Pipeline passing
- [ ] `git checkout {{.Major}}-{{.Minor}}-stable && git pull` in your local working copy!
- [ ] add **v{{.Major}}.{{.Minor}}.0-rcZ** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md; git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0-rcZ
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0-rcZ**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0-rcZ -m "Version v{{.Major}}.{{.Minor}}.0-rcZ"; git push origin v{{.Major}}.{{.Minor}}.0-rcZ
    ```

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0-rcZ` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0-rcZ/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0-rcZ)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0-rcZ` passing
- [ ] deploy **v{{.Major}}.{{.Minor}}.0-rcZ** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)
```
