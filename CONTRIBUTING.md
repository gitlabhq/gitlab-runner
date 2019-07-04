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

The following contents has to be considered as an extension over [gitlab-ce contributing guidelines](https://docs.gitlab.com/ce/development/contributing/index.html).

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

## Workflow lables

We have some additional labels plus those defined in [gitlab-ce workflow labels](https://docs.gitlab.com/ce/development/contributing/issue_workflow.html)

- Additional subjects: ~cache, ~executors, ~"git operations"
- OS: ~"os::Linux" ~"os::MacOSX" ~"os:FreeBSD" ~"os::Windows" 
- executor: ~"executor::docker" ~"executor::kubernetes" ~"executor::docker\-machine" ~"executor::docker\-machine" ~"executor::shell" ~"executor::parallels" ~"executor::virtualbox" 

