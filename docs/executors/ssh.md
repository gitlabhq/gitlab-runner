---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> The SSH executor supports only scripts generated in Bash and the caching feature
> is not supported.

This executor allows you to execute builds on a remote machine
by executing commands over SSH.

> [!note]
> Ensure you meet [common prerequisites](_index.md#git-requirements-for-non-docker-executors)
> on any remote systems where GitLab Runner uses the SSH executor.

## Use the SSH executor

To use the SSH executor, specify `executor = "ssh"` in the
[`[runners.ssh]`](../configuration/advanced-configuration.md#the-runnersssh-section) section. For example:

```toml
[[runners]]
  executor = "ssh"
  [runners.ssh]
    host = "example.com"
    port = "22"
    user = "root"
    password = "password"
    identity_file = "/path/to/identity/file"
```

You can use `password` or `identity_file` or both to authenticate against the
server. GitLab Runner doesn't implicitly read `identity_file` from
`/home/user/.ssh/id_(rsa|dsa|ecdsa)`. The `identity_file` needs to be
explicitly specified.

The project's source is checked out to:
`~/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`.

Where:

- `<short-token>` is a shortened version of the runner's token (first 8 letters)
- `<concurrent-id>` is the index of the runner from the list of all runners that run a build for the same project concurrently (accessible through the
  `CI_CONCURRENT_PROJECT_ID` [pre-defined variable](https://docs.gitlab.com/ci/variables/predefined_variables/)).
- `<namespace>` is the namespace where the project is stored on GitLab
- `<project-name>` is the name of the project as it is stored on GitLab

To overwrite the `~/builds` directory, specify the `builds_dir` options under
`[[runners]]` section in [`config.toml`](../configuration/advanced-configuration.md).

If you want to upload job artifacts, install `gitlab-runner` on the host you are
connecting to through SSH.

## Configure strict host key checking

SSH `StrictHostKeyChecking` is [enabled](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28192) by default.
To disable SSH `StrictHostKeyChecking`, set `[runners.ssh.disable_strict_host_key_checking]` to `true`.
The current default value is `false`.
