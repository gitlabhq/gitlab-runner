---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Download, install, and configure GitLab Runner as a user-mode service on Apple Silicon and Intel x86-64 systems.
title: Install GitLab Runner on macOS
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Install GitLab Runner on macOS on Apple Silicon or Intel x86-64 systems. GitLab itself
typically runs on a container or virtual machine, either locally or remotely.

## macOS service modes

On macOS, GitLab Runner runs as a user-mode `LaunchAgent`, not as a system-level `LaunchDaemon`.
This is the only supported mode.

In user-mode, the runner:

- Runs as the currently authenticated user, not as root.
- Starts when that user signs in, and stops when they sign out.
- Has access to the user's keychain and UI session, which is required to run the iOS Simulator
  and to perform code signing.
- Stores its configuration in `~/.gitlab-runner/config.toml`.

A system-level `LaunchDaemon` starts at boot, runs as root, and has no access to a user session.
GitLab Runner does not support running as a `LaunchDaemon`.

To keep the runner available after a reboot, turn on automatic login on the macOS machine.

## Install GitLab Runner

Install GitLab Runner on macOS to run CI/CD jobs on Apple Silicon or Intel x86-64 systems.

Prerequisites:

- You must be signed in to the macOS machine as the user account that runs the jobs.
  Do not use an SSH session for this procedure. Use a local GUI terminal.

To install GitLab Runner:

1. Download the binary for your system:

   - For Intel (x86-64):

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - For Apple Silicon:

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   To download a binary for a specific tagged release, see
   [download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Make the binary executable:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. [Register a runner](../register/_index.md) configuration. Use the
   [shell executor](../executors/shell.md) for iOS and macOS builds.
   For security details, see
   [security for shell executor](../security/_index.md#usage-of-shell-executor).

1. Install and start the GitLab Runner service:

   ```shell
   cd ~
   gitlab-runner install
   gitlab-runner start
   ```

1. Reboot your system.

The `gitlab-runner install` command creates a `LaunchAgent` plist at
`~/Library/LaunchAgents/gitlab-runner.plist` and registers it with `launchctl`.
If you encounter errors, see [troubleshooting](#troubleshooting).

## Configuration file locations

| File                 | Path                                             |
|----------------------|--------------------------------------------------|
| Configuration        | `~/.gitlab-runner/config.toml`                   |
| `LaunchAgent` plist  | `~/Library/LaunchAgents/gitlab-runner.plist`     |
| Standard output log  | `~/Library/Logs/gitlab-runner.out.log`           |
| Standard error log   | `~/Library/Logs/gitlab-runner.err.log`           |

For more information about configuration options, see
[advanced configuration](../configuration/advanced-configuration.md).

## Upgrade GitLab Runner

To upgrade GitLab Runner to a newer version:

1. Stop the service:

   ```shell
   gitlab-runner stop
   ```

1. Download the binary to replace the GitLab Runner executable:

   - For Intel (x86-64):

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - For Apple Silicon:

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   To download a binary for a specific tagged release, see
   [download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Make the binary executable:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Start the service:

   ```shell
   gitlab-runner start
   ```

## Upgrade the service file

To upgrade the `LaunchAgent` configuration, uninstall and reinstall the service:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## Use `codesign` with GitLab Runner

If you installed GitLab Runner with Homebrew and your build calls `codesign`, you might need
to set `<key>SessionCreate</key><true/>` to access the user keychain.

> [!note]
> GitLab does not maintain the Homebrew formula. Use the official binary to install GitLab Runner.

In the following example, the runner runs builds as the `gitlab` user and needs access to
that user's signing certificates:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>SessionCreate</key><true/>
    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>Disabled</key><false/>
    <key>Label</key>
    <string>com.gitlab.gitlab-runner</string>
    <key>UserName</key>
    <string>gitlab</string>
    <key>GroupName</key>
    <string>staff</string>
    <key>ProgramArguments</key>
    <array>
      <string>/usr/local/opt/gitlab-runner/bin/gitlab-runner</string>
      <string>run</string>
      <string>--working-directory</string>
      <string>/Users/gitlab/gitlab-runner</string>
      <string>--config</string>
      <string>/Users/gitlab/gitlab-runner/config.toml</string>
      <string>--service</string>
      <string>gitlab-runner</string>
      <string>--syslog</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
      <key>PATH</key>
      <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
  </dict>
</plist>
```

## Troubleshooting

When installing GitLab Runner on macOS, you might encounter the following issues.

For general troubleshooting, see [troubleshooting GitLab Runner](../faq/_index.md).

### Error: `killed: 9`

On Apple Silicon, you might get this error when you run the `gitlab-runner install`,
`gitlab-runner start`, or `gitlab-runner register` commands.

To resolve this error, ensure the directories for `StandardOutPath` and `StandardErrorPath`
in `~/Library/LaunchAgents/gitlab-runner.plist` exist and are writable. For example:

```xml
<key>StandardErrorPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.err.log</string>
<key>StandardOutPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.out.log</string>
```

### Error: `"launchctl" failed: Could not find domain for`

This error occurs when you manage the GitLab Runner service over SSH instead of a local
GUI terminal.

To resolve this error, open a terminal application directly on the macOS machine and run
the `install` and `start` commands from there.

### Error: `Failed to authorize rights (0x1) with status: -60007`

This error has two possible causes.

Your user account does not have developer tools access. To grant access:

```shell
DevToolsSecurity -enable
sudo security authorizationdb remove system.privilege.taskport is-developer
```

Or, the `LaunchAgent` plist has `SessionCreate` set to `true`. To fix this issue, reinstall
the service:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

Verify that `~/Library/LaunchAgents/gitlab-runner.plist` now has `SessionCreate`
set to `false`.

### Error: `Failed to connect to path port 3000: Operation timed out`

The runner cannot reach your GitLab instance. Check for firewalls, proxies, routing
configuration, or permission issues that might be blocking the connection.

### Error: `FATAL: Failed to start gitlab-runner: exit status 134`

This error indicates the GitLab Runner service is not installed correctly.

To resolve this error, reinstall the service:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

If the error persists, sign in to the macOS GUI desktop instead of using SSH, and run the
commands from a terminal there. The `LaunchAgent` requires a graphical login session to
bootstrap.

For macOS instances on AWS, follow the
[AWS documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-to-mac-instance.html)
to connect to the GUI, then retry from a terminal in that session.

### Error: `launchctl failed: Load failed: 5: Input/output error`

If you encounter this error when you run the `gitlab-runner start` command, first check
if the runner is already running:

```shell
gitlab-runner status
```

If the runner is not running, ensure the directories for `StandardOutPath` and
`StandardErrorPath` in `~/Library/LaunchAgents/gitlab-runner.plist` exist and that
the runner's user account has read and write access to them. Then start the runner:

```shell
gitlab-runner start
```

### Error: `couldn't build CA Chain`

This error can occur after upgrading to GitLab Runner v15.5.0. The full error message is:

```plaintext
ERROR: Error on fetching TLS Data from API response... error  error=couldn't build CA Chain:
error while fetching certificates from TLS ConnectionState: error while fetching certificates
into the CA Chain: couldn't resolve certificates chain from the leaf certificate: error while
resolving certificates chain with verification: error while verifying last certificate from
the chain: x509: "Baltimore CyberTrust Root" certificate is not permitted for this usage
runner=x7kDEc9Q
```

To resolve this error:

1. Upgrade to GitLab Runner v15.5.1 or later.
1. If you cannot upgrade, set `FF_RESOLVE_FULL_TLS_CHAIN` to `false` in the
   [`[runners.feature_flags]` configuration](../configuration/feature-flags.md#enable-feature-flag-in-runner-configuration):

   ```toml
   [[runners]]
     name = "example-runner"
     url = "https://gitlab.com/"
     token = "TOKEN"
     executor = "docker"
     [runners.feature_flags]
       FF_RESOLVE_FULL_TLS_CHAIN = false
   ```

### Homebrew Git credential helper causes fetches to hang

If Homebrew installed Git, it may have added a `credential.helper = osxkeychain` entry to
`/usr/local/etc/gitconfig`. This caches credentials in the macOS keychain and can cause
`git fetch` to hang.

To remove the credential helper system-wide:

```shell
git config --system --unset credential.helper
```

To disable it only for the GitLab Runner user:

```shell
git config --global --add credential.helper ''
```

To check the current setting:

```shell
git config credential.helper
```
