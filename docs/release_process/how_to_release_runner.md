# How to release GitLab Runner

Permission to push to `master` branch at https://gitlab.com/gitlab-org/gitlab-runner.git
are required to release the Runner.

## Release stable version

1. Make sure that all required changes are merged in and that the recent CI/CD pipeline
    for `master` branch is passing. The easiest way is to check https://gitlab.com/gitlab-org/gitlab-runner/commits/master.

1. Run `./scripts/prepare-changelog-entries.rb` to list merge requests merged since last
    tag, e.g.:

    ```bash
    $ ./scripts/prepare-changelog-entries.rb
    STARTING_POINT variable not set, using autodiscovered: v9.3.0

    - Warn on archiving git directory !591
    - Use Go 1.8 for CI !620
    - Add CacheClient with timeout configuration for cache operations !608
    - Remove '.git/hooks/post-checkout' hooks when using fetch strategy !603
    - Update linux-repository.md !624
    - Fix VirtualBox and Parallels executors registration bugs !589
    - Support Kubernetes PVCs !606
    - Support cache policies in .gitlab-ci.yml !621
    - Improve kubernetes volumes support !625
    - Fix incorrect substitute example in hostname in docker.md !627
    - Fix typo in debug !630
    - Adds an option `--all` to unregister command !622
    ```

1. Chose entries that should be added to the changelog. Sometimes in one release
    were creating few merge requests that are related. If one MR only fixes another
    MR that was added in the same release and that added the main part of the change,
    there is no need to add them both. Just add the main entry.

1. Fix the grammar and language in entries - they are taken from MR titles and sometimes
    they could be not fixed during MR's review.

1. Add entries to the `CHANGELOG.md` file, e.g.:

    ```markdown
    v 9.4.0 (2017-07-22
    - Warn on archiving git directory !591
    - Use Go 1.8 for CI !620
    - Add CacheClient with timeout configuration for cache operations !608
    - Remove '.git/hooks/post-checkout' hooks when using fetch strategy !603
    - Update linux-repository.md !624
    - Fix VirtualBox and Parallels executors registration bugs !589
    - Support Kubernetes PVCs !606
    - Support cache policies in .gitlab-ci.yml !621
    - Improve kubernetes volumes support !625
    - Fix incorrect substitute example in hostname in docker.md !627
    - Fix typo in debug !630
    - Adds an option `--all` to unregister command !622
    ```

1. Commit the change in `CHANGELOG.md`:

    ```bash
    $ git add CHANGELOG.md
    $ git commit -m "Update CHANGELOG for v9.4.0"
    ```

1. Make sure that `VERSION` file contains a valid version number. If no, update
    the file:

    ```bash
    $ cat VERSION
    9.3.0
    $ echo "9.4.0" > VERSION
    $ cat VERSION
    9.4.0
    $ git add VERSION
    $ git commit -m "Bump version to 9.4.0"
    ```

1. Create the `x-y-stable` branch:

    ```bash
    $ git checkout -b 9-4-stable
    ```

1. Tag the version. Use the annotated tag. If you've set up the GPG configuration
    for your git environment - please use the signed tag:

    ```bash
    # if you haven't configured GPG for git
    $ git config user.signingkey >/dev/null || git tag -a v9.4.0 -m "Version v9.4.0"
    # if you've configured GPG for git
    $ git config user.signingkey >/dev/null && git tag -s v9.4.0 -m "Version v9.4.0"
    ```

1. Push stable branch and tag to `origin`

    ```bash
    $ git push -u origin 9-4-stable
    $ git push origin v9.4.0
    ```

1. Switch to `master` branch and bump version

    ```bash
    $ git checkout master
    $ echo "9.5.0" > VERSION
    $ git add VERSION
    $ git commit -m "Bump version to 9.5.0"
    $ git push
    ```

1. Go to https://gitlab.com/gitlab-org/gitlab-runner/pipelines and wait until
    the CI/CD Pipeline for tag will pass. If the latest Pipeline for `master` was passing
    then following the process above there should be not changes that could fail the pipeline
    at this time. Any failures should be a temoprary failures related to CI infrastructure
    and GitLab stability. In that case just retry te failing job.

