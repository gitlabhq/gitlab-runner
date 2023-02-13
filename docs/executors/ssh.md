---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# SSH **(FREE)**

NOTE:
The SSH executor supports only scripts generated in Bash and the caching feature
is currently not supported.

This is a simple executor that allows you to execute builds on a remote machine
by executing commands over SSH.

NOTE:
Ensure you meet [common prerequisites](index.md#prerequisites-for-non-docker-executors)
on any remote systems where GitLab Runner uses the SSH executor.

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
- `<concurrent-id>` is a unique number, identifying the local job ID on the
  particular runner in context of the project
- `<namespace>` is the namespace where the project is stored on GitLab
- `<project-name>` is the name of the project as it is stored on GitLab

To overwrite the `~/builds` directory, specify the `builds_dir` options under
`[[runners]]` section in [`config.toml`](../configuration/advanced-configuration.md).

If you want to upload job artifacts, install `gitlab-runner` on the host you are
connecting to via SSH.

## Configure strict host key checking

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3074) in GitLab 14.3.

To enable SSH `StrictHostKeyChecking`, make sure the `[runners.ssh.disable_strict_host_key_checking]` is set
to `false`. The current default is `true`.

[In GitLab 15.0 and later](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28192),
the default value is `false`, meaning host key checking is required.
