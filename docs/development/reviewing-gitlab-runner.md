# Reviewing GitLab Runner

This document contains rules and suggestions for GitLab Runner project reviewers.

## Reviewing tests coverage reports

In GitLab Runner project, we have a lot of code. Unfortunately, the code coverage is not comprehensive.
Currently (early 2019), the coverage is on the level of ~55%.

While adding tests to a legacy code is a hard task, we should ensure that new code that is being
added to the project has good tests coverage. Code reviewers are encouraged to look on the
coverage reports and ensure new code is covered.

We should aim for as much test coverage for new code as possible. Defining the level of
required coverage for a specific change is left for the reviewer judgment. Sometimes 100% coverage
will be something simple to achieve. Sometimes adding code with only 20% of the coverage will be
realistic and will ensure that the most important things are being tested. Dear reviewer - chose wisely :)

Getting back to the technical details...

Runner's CI Pipeline helps us here and provides the coverage reports in HTML format, for tests
executed in regular (`count`) and race (`atomic`) modes.

There are two places where test coverage reports can be seen. For:

- Contributions made directly to <https://gitlab.com/gitlab-org/gitlab-runner> project, changes merged to `master`
  branch and for all tagged releases.
- Community contributions and contributions made directly to <https://gitlab.com/gitlab-org/gitlab-runner> project.

### Test coverage report from S3

This report has a long-term life but, because it uses the `gitlab-runners-download` S3 bucket, it's available
only for contributions made directly to <https://gitlab.com/gitlab-org/gitlab-runner>. It is also available
for all jobs started from `master` branch (so mostly Merge Requests merges) and for all tagged releases.

To open the report:

1. Find the Pipeline related to the change that we want to review. It may be the latest Pipeline for the
   Merge Requests or a Pipeline for the tag. For example, we can look at this one:
   <https://gitlab.com/gitlab-org/gitlab-runner/pipelines/48686952>, which released the `v11.8.0` version of Runner.

1. In the pipeline, find the `stable S3` (for tagged releases), `bleeding edge S3` (for `master` and RC tagged releases),
   or `development S3` (for regular commits) job which should be present at the `release` stage. In our example
   pipeline, it will be: <https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/165757556>.

1. At the end of the job's log, we should see a line like:

   ```plaintext
   ==> Download index file: https://gitlab-runner-downloads.s3.amazonaws.com/latest/index.html
   ```

   Because when this job was triggered, and `v11.8.0` was also the `latest` release, we see a link to the
   `latest` version bucket. The problem with `latest` is that the content there changes when
   new stable/patch versions are released.

   Each pipeline also creates a deployment for a specific reference (a branch name
   or a tag name). Several lines above we can see:

   ```plaintext
   ==> Download index file: https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0/index.html
   ```

   This URL points to a bucket, that should not be changed in the future. For a `bleeding edge S3` started
   from a `master` branch, the URL should look like <https://gitlab-runner-downloads.s3.amazonaws.com/master/index.html>
   (which obviously also changes over time) and for the one started from a RC tag, it should look
   like <https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0-rc1/index.html>. For the `development S3` job, started
   from a regular commit (mostly tracked within a Merge Request), the URL should look like
   <https://gitlab-runner-downloads.s3.amazonaws.com/mask-trace/index.html>. In this case the `mask-trace` is the
   name of the branch, which was used as Merge Request source.

1. Open the S3 link gathered from the job's log. Following our example, let's open the
   <https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0/index.html> one. We can see here several files that
   are published as part of the release. We're interested in the content of the `coverage/` directory.

   In this directory, we can see three files with `.race.` as part of the filename, and three similar files
   but with `.regular.` as part of the filename. The files are tracking output of `go test` command executed
   with coverage options. The `.race.` files contain sources and reports for tests started with `-race` flag,
   while the `.regular.` files are sources and reports for tests started without this option.

   For those who are interested in details, the `-race` tests are using `atomic` coverage mode, while the standard
   tests are using `count` coverage mode.

   For our case, the `coverage/coverprofile.regular.html` file is what we should look at. `.race.` tests can fail
   in race condition situations (this is why we're executing them) and currently we have several of them that
   are constantly failing. This means that the coverage profile may not be full.

   The `.regular.` tests, instead, should give us the full overview of what's tested inside of our code. To inspect them:

1. Open wanted report HTML page. As stated above, `coverage/coverprofile.regular.html` is what we're interested
   in, so using our initial example we should open the <https://gitlab-runner-downloads.s3.amazonaws.com/v11.8.0/coverage/coverprofile.regular.html#file0>
   file.

1. At this moment, we can see a file browser showing test coverage details. In the drop-down select at the top,
   we can now start choosing files related to the reviewed modification and check how the coverage is changing.

### Test coverage report from job artifact

As written above, reports hosted on S3 buckets are available only for pipelines started directly
from <https://gitlab.com/gitlab-org/gitlab-runner> project. But many of the contributions that the reviewers
are handling are contributions coming from community forks.

In this case, we have the same two types of reports - `.regular.` and `.race.` - generated in exactly same
way. The only difference is the place where they can be found and their lifespan. Reports are
saved as job artifacts so they can be next passed to the release stage). There is a 7 day expiration
time set on them. So when reviewing a change that executed its pipeline more than a week before, the report
will be unavailable. But, a new pipeline execution, even without changes in the code, will resolve the problem.

To view a code coverage report for a merge request:

1. In the merge request's **Overview** tab, under the pipeline
   result, click on **View exposed artifact** to expand the section.
1. Click on **Code Coverage**.
1. Use the artifact browser to navigate to the `out/coverage/`
   directory. For example,
   <https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/browse/out/coverage/>.
   This directory will always contain six files - three `.race.` files
   and three `.regular.` files, as explained in the [S3 coverage report
   strategy](#test-coverage-report-from-s3).

   For reviewing changes, we're mostly interested in looking at the `.regular.` HTML
   report (the `coverprofile.regular.html` file). As you can see, all files are visible
   as external links, so for our example we will open
   <https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/172824578/artifacts/file/out/coverage/coverprofile.regular.html>
   which will redirect us to
   <https://gitlab-org.gitlab.io/-/gitlab-runner/-/jobs/172824578/artifacts/out/coverage/coverprofile.regular.html>
   where the report is stored.
1. At this moment, we can see the same file browser with coverage details as we seen with the S3 source.
   We can do the same. The only difference is that it will disappear in maximum of 7 days.

## Reviewing the Merge Request title

Because we generate [`CHANGELOG.md`](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/master/CHANGELOG.md) entries
from the merge request titles, making sure that the title is valid and informative is a part
of the reviewer and maintainer's responsibilities.

Before merging a merge request, check the title and update it if you think it will not be clear in the
`CHANGELOG.md` file. Keep in mind that the changelog will have only this one line, without the merge
request description, discussion or diff that provide more context.

As an example, look at <https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1812> and compare:

- `yml to yaml` - which is the original title and was added to changelog with our script,
- `Fix values.yaml file name in documentation` - which is what I've updated it to in the changelog.

What will `yml to yaml` tell a Runner administrator if they review the changelog before updating
to a newer version? Does it show the risks behind the update, the implemented behavior changes, a new
behavior/features that were added? Keep these questions in mind when reviewing the merge request and its title.

Contributors may not be aware of the above information, and that their titles
may not match our requirements. Try to educate the contributor about this.
In the end, it's your responsibility to verify and update the title **before the merge request is merged**.

## Summary

Dear reviewer, you've got your sword. Now go fight with the dragons!
