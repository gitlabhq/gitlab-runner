# Reviewing tips&tricks

## Tests coverage report reviewing

In GitLab Runner project we have a lot of code. But, unfortunately, the code coverage is not so big.
Currently (early 2019), the coverage is on the level of ~55%.

While adding tests to a legacy code is a hard task, we should ensure that new code that is being
added to the project, have a good tests coverage. Code Reviewers are encouraged to look on the
coverage reports and check, how the newly added code looks like.

We should aim to create as big test coverage for new code, as it's possible. Defining the level of
required coverage for a specific change is left for the reviewer judgement. Sometimes 100% coverage
will be something simple to create. Sometimes adding code with only 20% of the coverage will be
realistic and will ensure that the most important things are being tested. Dear reviewer - chose wisely :)

Getting back to the technical details...

Runner's CI Pipeline helps us here and provides the coverage reports in HTML formats, for tests
executed in a regular (`count`) and race (`atomic`) modes.

There are two places where test coverage report may be seen. One is common for community contributions
and contributions made directly to https://gitlab.com/gitlab-org/gitlab-runner project. The second
one is available only for contributions made directly, for `master` branch and for all tagged releases.

### Test coverage report from S3

This report has a long-term life, but because it uses the `gitlab-runners-download` S3 bucket, it's available
only for contributions made directly to https://gitlab.com/gitlab-org/gitlab-runner. It is also available
for all jobs started from `master` branch (so mostly Merge Requests merges) and for all tagged releases.

To open the report:

1. Find the Pipeline related to the change that we want to review. It may be the latest Pipeline for the
   Merge Requests or a Pipeline for the tag. Just chose whatever you need. For example, we can look on this one:
   https://gitlab.com/gitlab-org/gitlab-runner/pipelines/48686952, which released the `v11.8.0` version of Runner.

1. In the pipeline find the `stable S3` (for tagged releases), `bleeding edge S3` (for `master` and RC tagged releases)
   or `development S3` (for regular commits) job which should be present at the `release` stage. In our example
   Pipeline it will be: https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/165757556.

1. At the end of job's log we should see a line like

    ```
    ==> Download index file: https://gitlab-runner-downloads.s3.amazonaws.com/latest/index.html
    ```

    Because when this job was triggered, `v11.8.0` was also the `latest` release, we see link to the
    `latest` version bucket. The problem with `latest` is that the content there changes in time, when
    new stable/patch versions are released.

    But each Pipeline creates also a deployment for a specific reference - whether it will be a branch name
    or a tag name - and several lines above we can see:

    ```
    ==> Download index file: https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0/index.html
    ```

    This URL points to a bucket, that should not be changed in the future. For a `bleeding edge S3` started
    from a `master` branch, the URL should look like https://gitlab-runner-downloads.s3.amazonaws.com/master/index.html
    (which obviously also changes in time) and for the one started from a RC tag, it should look
    like https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0-rc1/index.html. For the `development S3` job, started
    from a regular commit (mostly tracked within a Merge Request), the URL should look like
    https://gitlab-runner-downloads.s3.amazonaws.com/mask-trace/index.html. In this case the `mask-trace` is the
    name of the branch, which was used as Merge Request source.

1. Open the S3 link gathered from job's trace. Following our example, let's open the
   https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0/index.html one. We can see here several files that
   are published as part of the release. We're interested in the content of the `coverage/` directory.

   In this directory we can see three files with `.race.` as part of the filename, and three similar files
   but with `.regular.` as part of the filename. The files are tracking output of `go test` command executed
   with coverage options. The `.race.` files are containing sources and reports for tests started with `-race` flag,
   while the `.regular.` files are sources and reports for tests started without this option.

   For those who are interested in details, the `-race` tests are using `atomic` coverage mode, while the standard
   tests are using `count` coverage mode.

   For our case the `coverage/coverprofile.regular.html` file is what we should look on. `.race.` tests can fail
   in race condition situations (this is why we're executing them) and currently we have several of them that
   are constantly failing. This means that the coverage profile may not be full.

   The `.regular.` tests, instead, should give us the full overview of what's tested inside of our code.

1. Open wanted report HTML page. As it stays above, `coverage/coverprofile.regular.html` is what we're interested
   in, so using our initial example we should open the https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0/coverage/coverprofile.regular.html#file0
   file.

1. At this moment we can see a file browser showing test coverage details. In the drop-down select at the top
   we can now start choosing files related to the reviewed modification and check how the coverage is changing.

### Test coverage report from job artifact

As it was wrote above, reports hosted on S3 bucket are available only for Pipelines started directly
from https://gitlab.com/gitlab-org/gitlab-runner project. But many of the contributions that the reviewers
are handling, are contributions coming from the community forks.

In this case we have the same two types of reports - `.regular.` and `.race.` - generated in exactly same
way. The only difference is the place where they can be found and the time of their life. Reports are being
saved as job artifacts (so they can be next passed to the release stage). And there is a 7 days expiration
time set on them. So when reviewing the change, that executed its Pipeline more than a week before, the report
will be unavailable. But a new Pipeline execution, even without changes in the code, will resolve the problem.

To open the report:

1. First step doesn't differ much from the previous strategy. We need to find the Pipeline related to the
   change that will be reviewed. For example, we can use https://gitlab.com/gitlab-org/gitlab-runner/pipelines/50600305.
   It's a branch started from `master` at https://gitlab.com/gitlab-org/gitlab-runner. Normally we could just look
   for the S3 deployment, but all Pipelines started inside of https://gitlab.com/gitlab-org/gitlab-runner have
   also stored the reports as jobs artifacts. And in this case I still have a time to click `keep`, so the future
   readers of this documentation will be able to see the artifacts ;)

1. In the pipeline find the `test coverage report` job in the `coverage` stage. In our example it's available
   at https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578.

1. On the job's page let's use the `Browse` button (on the right panel) to open Artifacts Browser. The browser
   for our example job is available at https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/browse.

1. In the browser we need to navigate to `out/coverage/` directory
   (https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/browse/out/coverage/). This directory
   will contain the same six files - three with `.race.` and three similar with `.regular.` as it was described
   in the S3 strategy.

   For change reviewing we're mostly interested in looking on the HTML report for `.regular.` type - the
   `coverprofile.regular.html` file. As we can see all files are visible as external links, so for our
   example we'll open https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/file/out/coverage/coverprofile.regular.html
   link, which will next redirect us to https://gitlab-org.gitlab.io/-/gitlab-runner/-/jobs/172824578/artifacts/out/coverage/coverprofile.regular.html
   where the report is being presented.

1. At this moment we can see the same file browser with coverage details, as we seen with the S3 source.
   We can do the same. The only difference is that it will disappear in maximum 7 days.

### Summary

Dear reviewer, you've got your sword. Now go fight with the dragons!
