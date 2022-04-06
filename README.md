# GitLab Runner

This is the repository of the official GitLab Runner written in Go.
It runs tests and sends the results to GitLab.
[GitLab CI](https://about.gitlab.com/gitlab-ci) is the open-source
continuous integration service included with GitLab that coordinates the testing.
The old name of this project was GitLab CI Multi Runner but please use "GitLab Runner" (without CI) from now on.

[![Pipeline Status](https://gitlab.com/gitlab-org/gitlab-runner/badges/main/pipeline.svg)](https://gitlab.com/gitlab-org/gitlab-runner/commits/main)
[![Go Report Card](https://goreportcard.com/badge/gitlab.com/gitlab-org/gitlab-runner)](https://goreportcard.com/report/gitlab.com/gitlab-org/gitlab-runner)

## Runner and GitLab CE/EE compatibility

For a list of compatible versions between GitLab and GitLab Runner, consult
the [compatibility section](https://docs.gitlab.com/runner/#compatibility-with-gitlab-versions).

## Release process

The description of release process of GitLab Runner project can be found inside of [`PROCESS.md`](PROCESS.md).

## Contributing

Contributions are welcome, see [`CONTRIBUTING.md`](CONTRIBUTING.md) for more details.

### Closing issues

GitLab is growing very fast and we have limited resources to deal with
issues opened by community volunteers. We appreciate all the
contributions coming from our community, but we need to create some
closing policy to help all of us with issue management.

The issue tracker is not used for support or configuration questions. We
have dedicated [channels](https://about.gitlab.com/support/) for these
kinds of questions. The issue tracker should only be used for feature
requests, bug reports, and other tasks that need to be done for the
Runner project.

It is up to a project maintainer to decide if an issue is actually a
support/configuration question. Before closing the issue the maintainer
should leave a reason why this is a support/configuration question, to make
it clear to the issue author. They should also leave a comment using
[our template](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/PROCESS.md#support-requests-and-configuration-questions)
before closing the issue. The issue author has every right to disagree and
reopen the issue for further discussion.

### Contributing to documentation

If your contribution contains only documentation changes, you can speed up the CI process
by following some branch naming conventions, as described in <https://docs.gitlab.com/ce/development/documentation/index.html#branch-naming>

## Documentation

The documentation source files can be found under the [docs/](docs/) directory. You can
read the documentation online at <https://docs.gitlab.com/runner/>.

## Requirements

[Read about the requirements of GitLab Runner.](https://docs.gitlab.com/runner/#requirements)

## Features

[Read about the features of GitLab Runner.](https://docs.gitlab.com/runner/#features)

## Executors compatibility chart

[Read about what options each executor can offer.](https://docs.gitlab.com/runner/executors/#compatibility-chart)

## Install GitLab Runner

Visit the [installation documentation](https://docs.gitlab.com/runner/install/).

## Use GitLab Runner

See [https://docs.gitlab.com/runner/#using-gitlab-runner](https://docs.gitlab.com/runner/#using-gitlab-runner).

## Select executor

See [https://docs.gitlab.com/runner/executors/#selecting-the-executor](https://docs.gitlab.com/runner/executors/#selecting-the-executor).

## Troubleshooting

Read the [FAQ](https://docs.gitlab.com/runner/faq/).

## Advanced Configuration

See [https://docs.gitlab.com/runner/#advanced-configuration](https://docs.gitlab.com/runner/#advanced-configuration).

## Building and development

See [https://docs.gitlab.com/runner/development/](https://docs.gitlab.com/runner/development/).

## Changelog

Visit the [Changelog](CHANGELOG.md) to view recent changes.

## The future

- Please see the [GitLab Direction page](https://about.gitlab.com/direction/).
- Feel free submit issues with feature proposals on the issue tracker.

## Author

- 2014 - 2015   : [Kamil Trzciński](mailto:ayufan@ayufan.eu)
- 2015 - now    : GitLab Inc. team and contributors

## License

This code is distributed under the MIT license, see the [LICENSE](LICENSE) file.
