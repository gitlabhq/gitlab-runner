---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/marketing/product-and-technical-marketing/technical-writing/#assignments>
title: SSH
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> This executor is in maintenance mode. It receives critical security updates but no new features are planned.
> For new projects, consider using one of the [actively developed executors](_index.md#selecting-the-executor).

The SSH executor is included for completeness, but it's among the least supported executors.
GitLab Runner connects to an external server and runs builds there over SSH.
While some organizations use this executor successfully, it's generally better to use a different executor type.

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
GitLab Runner checks the host key of the remote server against a `known_hosts` file.
By default, GitLab Runner uses `.ssh/known_hosts` in the home directory of the user that runs GitLab Runner.
To use a different file, set `known_hosts_file` in the `[runners.ssh]` section:

```toml
[runners.ssh]
  host = "example.com"
  port = "22"
  user = "root"
  identity_file = "/path/to/identity/file"
  known_hosts_file = "/path/to/known_hosts"
```

Use an absolute path for `known_hosts_file`. GitLab Runner does not expand `~`.

If the job fails, check the error message:

- `knownhosts: key is unknown`: The host key is not in the `known_hosts` file. Add the host key.
- `knownhosts: key mismatch`: The `known_hosts` file has an entry for the host, but none of its keys match the key that the server presented. The host key might have changed, or the file might not contain every key type that the server offers. Verify the fingerprints, then replace the host key.
- `getting host key callback: open <path>: no such file or directory`: The `known_hosts` file does not exist. Add the host key, which creates the file.

### Add the host key of the remote server

To add the host key of the remote server to the `known_hosts` file:

1. On the GitLab Runner host, as the user that runs GitLab Runner, scan the remote server.
   Replace `example.com` and `22` with the `host` and `port` values from `[runners.ssh]`:

   ```shell
   ssh-keyscan -p 22 example.com > host_keys.txt
   ssh-keygen -lf host_keys.txt
   ```

   Do not limit the scan with `-t`. The file must contain every host key type that the server offers.

1. On the remote server, run `ssh-keygen -lf <public_host_key_file>` for each public host key.
   These files are usually in `/etc/ssh/` and named `ssh_host_*_key.pub`.
   Confirm that every fingerprint from the previous step appears in this output.
   If one does not, do not continue.

1. Add the keys to the `known_hosts` file:

   ```shell
   mkdir -p ~/.ssh
   cat host_keys.txt >> ~/.ssh/known_hosts
   ```

   The commands create the `.ssh` directory and the `known_hosts` file if they do not exist.

   If you set `known_hosts_file`, replace `~/.ssh/known_hosts` with that path and make sure its parent directory exists.

### Replace a changed host key

To replace the host key of the remote server after it changes:

1. On the GitLab Runner host, as the user that runs GitLab Runner, remove the old entry.
   Replace `<host>` with the `host` value from `[runners.ssh]`.
   For a port other than `22`, use `[<host>]:<port>`.

   ```shell
   ssh-keygen -R <host>
   ```

   If you set `known_hosts_file`, add `-f <known_hosts_file>`.

1. Add the new host key by repeating the steps to add the host key of the remote server.

### Disable strict host key checking

> [!warning]
> When you disable strict host key checking, GitLab Runner does not verify the identity of the
> remote server. This setting makes the connection vulnerable to man-in-the-middle attacks.
> Add the host key to the `known_hosts` file instead.

To disable strict host key checking, set `disable_strict_host_key_checking` to `true` in the `[runners.ssh]` section.
The default value is `false`.
