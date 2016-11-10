# GitLab Runner

This is the repository of the official GitLab Runner written in Go.
It runs tests and sends the results to GitLab.
[GitLab CI](https://about.gitlab.com/gitlab-ci) is the open-source
continuous integration service included with GitLab that coordinates the testing.
The old name of this project was GitLab CI Multi Runner but please use "GitLab Runner" (without CI) from now on.

![Build Status](https://gitlab.com/gitlab-org/gitlab-ci-multi-runner/badges/master/build.svg)

## Release process

The description of release process of GitLab Runner project can be found in the [release documentation](docs/release_process/README.md).

## Contributing

Contributions are welcome, see [`CONTRIBUTING.md`](CONTRIBUTING.md) for more details.

### Closing issues and merge requests

GitLab is growing very fast and we have a limited resources to deal with reported issues
and merge requests opened by the community volunteers. We appreciate all the contributions
coming from our community. But to help all of us with issues and merge requests management
we need to create some closing policy.

If an issue or merge request has a ~"waiting for feedback" label and the response from the
reporter has not been received for 14 days, we can close it using the following response
template:

```
We haven't received an update for more than 14 days so we will assume that the
problem is fixed or is no longer valid. If you still experience the same problem
try upgrading to the latest version. If the issue persists, reopen this issue
or merge request with the relevant information.
```

## Documentation

The documentation source files can be found under the [docs/](docs/) directory. You can
read the documentation online at https://docs.gitlab.com/runner/.

## Requirements

[Read about the requirements of GitLab Runner.](https://docs.gitlab.com/runner/#requirements)

## Features

[Read about the features of GitLab Runner.](https://docs.gitlab.com/runner/#features)

## Compatibility chart

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

## Extra projects?

If you want to add another project, token or image simply RE-RUN SETUP.
*You don't have to re-run the runner. It will automatically reload configuration once it changes.*

## Changelog

Visit the [Changelog](CHANGELOG.md) to view recent changes.

### Version 0.5.0

Version 0.5.0 introduces many security related changes.
One of such changes is the different location of `config.toml`.
Previously (prior 0.5.0) config was read from current working directory.
Currently, when `gitlab-runner` is executed by `root` or with `sudo` config is read from `/etc/gitlab-runner/config.toml`.
If `gitlab-runner` is executed by non-root user, the config is read from `$HOME/.gitlab-runner/config.toml`.
However, this doesn't apply to Windows where config is still read from current working directory, but this most likely will change in future.

The config file is automatically migrated when GitLab Runner was installed from GitLab's repository.
**For manual installations the config needs to be moved by hand.**

## The future

* Please see the [GitLab Direction page](https://about.gitlab.com/direction/).
* Feel free submit issues with feature proposals on the issue tracker.

## Author

```
2014 - 2015   : [Kamil Trzciński](mailto:ayufan@ayufan.eu)
2015 - now    : GitLab Inc. team and contributors
```

## License

This code is distributed under the MIT license, see the [LICENSE](LICENSE.md) file.
