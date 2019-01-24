/label ~"devops:verify"
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
- [ ] Update the `.HelmChartMajor` and `.HelmChartMinor` to a specific release version

## First working day after 7th - **v{{.Major}}.{{.Minor}}.0-rc1 release**

- [ ] check if Pipeline for `master` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/master/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/master)
    - [ ] add all required fixes to make `master` Pipeline passing
- [ ] `git checkout master && git pull` in your local working copy!
- [ ] prepare CHANGELOG entries

    ```bash
    ./scripts/prepare-changelog-entries.rb
    ```

    Copy the lines to the beginning of `CHANGELOG.md` file and add a proper header:

    ```markdown
    v{{.Major}}.{{.Minor}}.0-rc1 (TODAY_DATE_HERE)
    ```

- [ ] add **v{{.Major}}.{{.Minor}}.0-rc1** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md; git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0-rc1" -S
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0-rc1**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0-rc1 -m "Version v{{.Major}}.{{.Minor}}.0-rc1" && git push origin v{{.Major}}.{{.Minor}}.0-rc1
    ```

- [ ] create and push `{{.Major}}-{{.Minor}}-stable` branch:

    ```bash
    git checkout -b {{.Major}}-{{.Minor}}-stable; git push -u origin {{.Major}}-{{.Minor}}-stable
    ```

- [ ] checkout to `master`, update `VERSION` file to `{{.Major}}.{{inc .Minor}}.0` and push `master`:

    ```bash
    git checkout master; echo -n "{{.Major}}.{{inc .Minor}}.0" > VERSION; git add VERSION; git commit -m "Bump version to {{.Major}}.{{inc .Minor}}.0" -S && git push
    ```

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0-rc1` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0-rc1/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0-rc1)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0-rc1` passing
- [ ] deploy **v{{.Major}}.{{.Minor}}.0-rc1** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)
- [ ] update runner [helm chart](https://gitlab.com/charts/gitlab-runner) to use `v{{.Major}}.{{.Minor}}.0-rc1` version
    - [ ] check if Pipeline for `master` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/master/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/master)
        - [ ] add all required fixes to make `master` Pipeline passing
    - [ ] go to your local working copy of https://gitlab.com/charts/gitlab-runner
    - [ ] `git checkout master && git pull` in your local working copy!
    - [ ] set Helm Chart to use `v{{.Major}}.{{.Minor}}.0-rc1` version of Runner
        - [ ] create new branch, update Runner version and push the branch:

            ```bash
            git checkout -b update-runner-to-{{.Major}}-{{.Minor}}-0-rc1 && sed -i "s/^appVersion: .*/appVersion: {{.Major}}.{{.Minor}}.0-rc1/" Chart.yaml && git add Chart.yaml && git commit -m "Bump used Runner version to {{.Major}}.{{.Minor}}.0-rc1" -S && git push -u origin update-runner-to-{{.Major}}-{{.Minor}}-0-rc1
            ```

        - [ ] create Merge Request pointing `master`: [link to MR here]
        - [ ] manage to merge the MR
    - [ ] check if Pipeline for `master` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/master/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/master)
        - [ ] add all required fixes to make `master` Pipeline passing
    - [ ] `git checkout master && git pull` in your local working copy!
    - [ ] prepare CHANGELOG entries

        ```bash
        ./scripts/prepare-changelog-entries.rb
        ```

        Copy the lines to the beginning of `CHANGELOG.md` file and add a proper header:

        ```markdown
        ## v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1 (TODAY_DATE_HERE)
        ```

    - [ ] add **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1** CHANGELOG entries and commit

        ```bash
        git add CHANGELOG.md && git commit -m "Update CHANGELOG for v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1" -S
        ```

    - [ ] bump version of the Helm Chart to `{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1`

        ```bash
        sed -i "s/^version: .*/version: {{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1/" Chart.yaml && git add Chart.yaml && git commit -m "Bump version to {{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1" -S
        ```

    - [ ] tag and push **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1**:

        ```bash
        git tag -s v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1 -m "Version v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1" && git push origin v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rc1
        ```

    - [ ] create and push `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` branch:

        ```bash
        git checkout -b {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git push -u origin {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable
        ```

    - [ ] checkout to `master`, bump version of the Helm Chart to `{{.HelmChartMajor}}.{{inc .HelmChartMinor}}.0-beta` and push `master`:

        ```bash
        git checkout master; sed -i "s/^version: .*/version: {{.HelmChartMajor}}.{{inc .HelmChartMinor}}.0-beta/" Chart.yaml && git add Chart.yaml && git commit -m "Bump version to {{.HelmChartMajor}}.{{inc .HelmChartMinor}}.0-beta" -S && git push
        ```

_New features_ window is closed - things not merged into `master` up to
this day, will be released with next release.

## 7 working days before 22th (**{{.ReleaseBlogPostDeadline}}**)

- [ ] prepare GitLab Runner entries for the release blog post. Items can be generated with `./scripts/changelog2releasepost | less` (executed in Runner's local working copy directory)
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

> **Notice:** If there was no new commits picked into `{{.Major}}-{{.Minor}}-stable` branch since
previous RC, we can skip this step. There is no need in releasing and deploying an RC identical
to the one that already exists.

- [ ] check if Pipeline for `{{.Major}}-{{.Minor}}-stable` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/{{.Major}}-{{.Minor}}-stable/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/{{.Major}}-{{.Minor}}-stable)
    - [ ] add all required fixes to make `{{.Major}}-{{.Minor}}-stable` Pipeline passing
- [ ] `git checkout {{.Major}}-{{.Minor}}-stable && git pull` in your local working copy!
- [ ] prepare CHANGELOG entries

    ```bash
    ./scripts/prepare-changelog-entries.rb
    ```

    Copy the lines to the beginning of `CHANGELOG.md` file and add a proper header:

    ```markdown
    v{{.Major}}.{{.Minor}}.0-rcZ (TODAY_DATE_HERE)
    ```

- [ ] add **v{{.Major}}.{{.Minor}}.0-rcZ** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md; git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0-rcZ" -S
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0-rcZ** and **{{.Major}}-{{.Minor}}-stable**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0-rcZ -m "Version v{{.Major}}.{{.Minor}}.0-rcZ" && git push origin v{{.Major}}.{{.Minor}}.0-rcZ {{.Major}}-{{.Minor}}-stable
    ```

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0-rcZ` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0-rcZ/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0-rcZ)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0-rcZ` passing
- [ ] deploy **v{{.Major}}.{{.Minor}}.0-rcZ** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)
- [ ] update runner [helm chart](https://gitlab.com/charts/gitlab-runner) to use `v{{.Major}}.{{.Minor}}.0-rcZ` version
    - [ ] check if Pipeline for `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable)
        - [ ] add all required fixes to make `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` Pipeline passing
    - [ ] go to your local working copy of https://gitlab.com/charts/gitlab-runner
    - [ ] `git checkout {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git pull` in your local working copy!
    - [ ] set Helm Chart to use `v{{.Major}}.{{.Minor}}.0-rcZ` version of Runner
        - [ ] create new branch, update Runner version and push the branch:

            ```bash
            git checkout -b update-runner-to-{{.Major}}-{{.Minor}}-0-rcZ && sed -i "s/^appVersion: .*/appVersion: {{.Major}}.{{.Minor}}.0-rcZ/" Chart.yaml && git add Chart.yaml && git commit -m "Bump used Runner version to {{.Major}}.{{.Minor}}.0-rcZ" -S && git push -u origin update-runner-to-{{.Major}}-{{.Minor}}-0-rcZ
            ```

        - [ ] create Merge Request pointing `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable`: [link to MR here]
        - [ ] manage to merge the MR
    - [ ] check if Pipeline for `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable)
        - [ ] add all required fixes to make `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` Pipeline passing
    - [ ] `git checkout {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git pull` in your local working copy!
    - [ ] prepare CHANGELOG entries

        ```bash
        ./scripts/prepare-changelog-entries.rb
        ```

        Copy the lines to the beginning of `CHANGELOG.md` file and add a proper header:

        ```markdown
        ## v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ (TODAY_DATE_HERE)
        ```

    - [ ] add **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ** CHANGELOG entries and commit

        ```bash
        git add CHANGELOG.md && git commit -m "Update CHANGELOG for v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ" -S
        ```

    - [ ] bump version of the Helm Chart to `{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ`

        ```bash
        sed -i "s/^version: .*/version: {{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ/" Chart.yaml && git add Chart.yaml && git commit -m "Bump version to {{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ" -S
        ```

    - [ ] tag and push **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ** and **{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable**:

        ```bash
        git tag -s v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ -m "Version v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ" && git push origin v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable
        ```

## At 22th - the release day

#### Before 12:00 UTC

- [ ] check if Pipeline for `{{.Major}}-{{.Minor}}-stable` is passing: [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/{{.Major}}-{{.Minor}}-stable/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/{{.Major}}-{{.Minor}}-stable)
    - [ ] add all required fixes to make `{{.Major}}-{{.Minor}}-stable` Pipeline passing
- [ ] `git checkout {{.Major}}-{{.Minor}}-stable && git pull` in your local working copy!
- [ ] merge all RCx CHANGELOG entries into release entry

    Put a proper header at the begining:

    ```markdown
    v{{.Major}}.{{.Minor}}.0 (TODAY_DATE_HERE)
    ```

- [ ] add **v{{.Major}}.{{.Minor}}.0** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md && git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0" -S
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0** and **{{.Major}}-{{.Minor}}-stable**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0 -m "Version v{{.Major}}.{{.Minor}}.0" && git push origin v{{.Major}}.{{.Minor}}.0 {{.Major}}-{{.Minor}}-stable
    ```

- [ ] checkout to `master` and merge `{{.Major}}-{{.Minor}}-stable` into `master` (only this one time, to update CHANGELOG.md and make the tag available for ./scripts/prepare-changelog-entries.rb in next stable release), push `master`:

    ```bash
    git checkout master; git merge --no-ff {{.Major}}-{{.Minor}}-stable
    # check that the only changes are in CHANGELOG.md
    git push
    ```

- [ ] update runner [helm chart](https://gitlab.com/charts/gitlab-runner) to use `v{{.Major}}.{{.Minor}}.0` version
    - [ ] check if Pipeline for `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable)
        - [ ] add all required fixes to make `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` Pipeline passing
    - [ ] go to your local working copy of https://gitlab.com/charts/gitlab-runner
    - [ ] `git checkout {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git pull` in your local working copy!
    - [ ] set Helm Chart to use `v{{.Major}}.{{.Minor}}.0` version of Runner
        - [ ] create new branch, update Runner version and push the branch:

            ```bash
            git checkout -b update-runner-to-{{.Major}}-{{.Minor}}-0 && sed -i "s/^appVersion: .*/appVersion: {{.Major}}.{{.Minor}}.0/" Chart.yaml && git add Chart.yaml && git commit -m "Bump used Runner version to {{.Major}}.{{.Minor}}.0" -S && git push -u origin update-runner-to-{{.Major}}-{{.Minor}}-0
            ```

        - [ ] create Merge Request pointing `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable`: [link to MR here]
        - [ ] manage to merge the MR
    - [ ] check if Pipeline for `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable)
        - [ ] add all required fixes to make `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` Pipeline passing
    - [ ] `git checkout {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git pull` in your local working copy!
    - [ ] merge all RCx CHANGELOG entries into release entry

        Put a proper header at the begining:

        ```markdown
        ## v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0 (TODAY_DATE_HERE)
        ```

    - [ ] add **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0** CHANGELOG entries and commit

        ```bash
        git add CHANGELOG.md && git commit -m "Update CHANGELOG for v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0" -S
        ```

    - [ ] bump version of the Helm Chart to `{{.HelmChartMajor}}.{{.HelmChartMinor}}.0`

        ```bash
        sed -i "s/^version: .*/version: {{.HelmChartMajor}}.{{.HelmChartMinor}}.0/" Chart.yaml && git add Chart.yaml && git commit -m "Bump version to {{.HelmChartMajor}}.{{.HelmChartMinor}}.0" -S
        ```

    - [ ] tag and push **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0** and **{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable**:

        ```bash
        git tag -s v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0 -m "Version v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0" && git push origin v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0 {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable
        ```

    - [ ] checkout to `master` and merge `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` into `master` (only this one time, to update CHANGELOG.md and make the tag available for `./scripts/prepare-changelog-entries.rb` in next stable release), push `master`:

        ```bash
        git checkout master; git merge --no-ff {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable
        # check that the only changes are in CHANGELOG.md
        git push
        ```

    - [ ] update Runner's chart version [used by GitLab](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/app/models/clusters/applications/runner.rb): [link to MR here]
    - [ ] update Runner's chart version [used by GitLab chart](https://gitlab.com/charts/gitlab/blob/master/requirements.yaml#L16): [link to MR here]

#### Before 15:00 UTC

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0` passing
- [ ] deploy stable version to all production Runners


## RC release template

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
- [ ] prepare CHANGELOG entries

    ```bash
    ./scripts/prepare-changelog-entries.rb
    ```

    Copy the lines to the beginning of `CHANGELOG.md` file and add a proper header:

    ```markdown
    v{{.Major}}.{{.Minor}}.0-rcZ (TODAY_DATE_HERE)
    ```

- [ ] add **v{{.Major}}.{{.Minor}}.0-rcZ** CHANGELOG entries and commit

    ```bash
    git add CHANGELOG.md; git commit -m "Update CHANGELOG for v{{.Major}}.{{.Minor}}.0-rcZ" -S
    ```

- [ ] tag and push **v{{.Major}}.{{.Minor}}.0-rcZ** and **{{.Major}}-{{.Minor}}-stable**:

    ```bash
    git tag -s v{{.Major}}.{{.Minor}}.0-rcZ -m "Version v{{.Major}}.{{.Minor}}.0-rcZ" && git push origin v{{.Major}}.{{.Minor}}.0-rcZ {{.Major}}-{{.Minor}}-stable
    ```

- [ ] wait for Pipeline for `v{{.Major}}.{{.Minor}}.0-rcZ` to pass [![pipeline status](https://gitlab.com/gitlab-org/gitlab-runner/badges/v{{.Major}}.{{.Minor}}.0-rcZ/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/v{{.Major}}.{{.Minor}}.0-rcZ)
    - [ ] add all required fixes to make `v{{.Major}}.{{.Minor}}.0-rcZ` passing
- [ ] deploy **v{{.Major}}.{{.Minor}}.0-rcZ** (https://gitlab.com/gitlab-com/runbooks/blob/master/howto/update-gitlab-runner-on-managers.md)
- [ ] update runner [helm chart](https://gitlab.com/charts/gitlab-runner) to use `v{{.Major}}.{{.Minor}}.0-rcZ` version
    - [ ] check if Pipeline for `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable)
        - [ ] add all required fixes to make `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` Pipeline passing
    - [ ] go to your local working copy of https://gitlab.com/charts/gitlab-runner
    - [ ] `git checkout {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git pull` in your local working copy!
    - [ ] set Helm Chart to use `v{{.Major}}.{{.Minor}}.0-rcZ` version of Runner
        - [ ] create new branch, update Runner version and push the branch:

            ```bash
            git checkout -b update-runner-to-{{.Major}}-{{.Minor}}-0-rcZ && sed -i "s/^appVersion: .*/appVersion: {{.Major}}.{{.Minor}}.0-rcZ/" Chart.yaml && git add Chart.yaml && git commit -m "Bump used Runner version to {{.Major}}.{{.Minor}}.0-rcZ" -S && git push -u origin update-runner-to-{{.Major}}-{{.Minor}}-0-rcZ
            ```

        - [ ] create Merge Request pointing `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable`: [link to MR here]
        - [ ] manage to merge the MR
    - [ ] check if Pipeline for `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` is passing: [![pipeline status](https://gitlab.com/charts/gitlab-runner/badges/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable/pipeline.svg)](https://gitlab.com/charts/gitlab-runner/commits/{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable)
        - [ ] add all required fixes to make `{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable` Pipeline passing
    - [ ] `git checkout {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable && git pull` in your local working copy!
    - [ ] prepare CHANGELOG entries

        ```bash
        ./scripts/prepare-changelog-entries.rb
        ```

        Copy the lines to the beginning of `CHANGELOG.md` file and add a proper header:

        ```markdown
        ## v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ (TODAY_DATE_HERE)
        ```

    - [ ] add **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ** CHANGELOG entries and commit

        ```bash
        git add CHANGELOG.md && git commit -m "Update CHANGELOG for v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ" -S
        ```

    - [ ] bump version of the Helm Chart to `{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ`

        ```bash
        sed -i "s/^version: .*/version: {{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ/" Chart.yaml && git add Chart.yaml && git commit -m "Bump version to {{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ" -S
        ```

    - [ ] tag and push **v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ** and **{{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable**:

        ```bash
        git tag -s v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ -m "Version v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ" && git push origin v{{.HelmChartMajor}}.{{.HelmChartMinor}}.0-rcZ {{.HelmChartMajor}}-{{.HelmChartMinor}}-0-stable
        ```
```
