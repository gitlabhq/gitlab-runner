# GitLab Runner

This is the repository of the official GitLab Runner written in Go.
It runs tests and sends the results to GitLab.
[GitLab CI](https://about.gitlab.com/gitlab-ci) is the open-source
continuous integration service included with GitLab that coordinates the testing.
The old name of this project was GitLab CI Multi Runner but please use "GitLab Runner" (without CI) from now on.

![Build Status](https://gitlab.com/gitlab-org/gitlab-ci-multi-runner/badges/master/build.svg)

## Runner and GitLab CE/EE compatibility

GitLab Runner >= 9.0 requires GitLab's API v4 endpoints, which were introduced in
GitLab CE/EE 9.0.

Because of this **Runner >= 9.0 requires GitLab CE/EE >= 9.0 and will not work
with older GitLab versions**.

Old API used by Runner will be still present in GitLab >= versions until August 2017.
Until then we will also support the v1.11.x version of Runner.

> This means that if you want to have a newer version of GitLab CE/EE but for some
reason you don't want to install newer version of Runner, 1.11.x will be still
maintained and will be working with GitLab CE/EE until August 2017. It may not
support some new features, but any bugs or security violations will be handled
as for the stable version.

### Compatibility chart

|                    | 8.16.x (01.2017) | 8.17.x (02.2017) | 9.0.x (03.2017) | 9.1.x (04.2017) | 9.2.x (05.2017) | 9.3.x (06.2017) | 9.4.x (07.2017) | 9.5.x (08.2017) | 10.0.x (09.2017) |
|:------------------:|:----------------:|:----------------:|:---------------:|:---------------:|:---------------:|:---------------:|:---------------:|:---------------:|:---------------:|
| v1.10.x            | Y, s             | Y, s             | Y, s            | Y, **u**        | Y, **u**        | Y, **u**        | Y, **u**        | Y, **u**        | **N**, **u**    |
| v1.11.x            | Y                | Y, s             | Y, s            | Y, s            | Y, s            | Y, s            | Y, s            | Y, s            | **N**, **u**    |
| v9.0.x             | **N**            | **N**            | Y, s            | Y, s            | Y, s            | Y, **u**        | Y, **u**        | Y, **u**        | Y, **u**        |
| v9.1.x             | **N**            | **N**            | Y               | Y, s            | Y, s            | Y, s            | Y, **u**        | Y, **u**        | Y, **u**        |
| v9.2.x             | **N**            | **N**            | Y               | Y               | Y, s            | Y, s            | Y, s            | Y, **u**        | Y, **u**        |
| v9.3.x             | **N**            | **N**            | Y               | Y               | Y               | Y, s            | Y, s            | Y, s            | Y, **u**        |
| v9.4.x             | **N**            | **N**            | Y               | Y               | Y               | Y               | Y, s            | Y, s            | Y, s            |
| v9.5.x             | **N**            | **N**            | Y               | Y               | Y               | Y               | Y               | Y, s            | Y, s            |
| v10.0.x _(planned)_ | **N**            | **N**            | Y               | Y               | Y               | Y               | Y               | Y               | Y, s            |

**Legend**

* Y - specified Runner version is/will be working with specified GitLab version
* N - specified Runner version is/will not be working with specified GitLab version
* s - specified Runner version is supported
* u - specified Runner version is not supported

### How to install older versions

Let's assume that you want to install version 1.11.2 of Runner:

1. If you're using DEB/RPM based installation:

    ```bash
    # for DEB based systems
    root@host:# apt-get install gitlab-ci-multi-runner=1.11.2

    # for RPM based systems
    root@host:# yum install gitlab-ci-multi-runner-1.11.2-1
    ```

1. If you need to install Runner manually, you can look for a propper package/binary
   at https://gitlab-ci-multi-runner-downloads.s3.amazonaws.com/v1.11.2/index.html

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

This code is distributed under the MIT license, see the [LICENSE](LICENSE) file.
