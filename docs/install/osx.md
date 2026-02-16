---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install GitLab Runner on macOS.
title: Install GitLab Runner on macOS
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

This page contains instructions about installing GitLab Runner on macOS (Apple Silicon and Intel x86-64).

{{< alert type="note" >}}

macOS users who install GitLab Runner typically
[install GitLab](https://docs.gitlab.com/install/install_methods/)
on a container or virtual machine that runs locally or remotely.

{{< /alert >}}

1. Download the binary for your system:

   - For Intel-based systems:

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - For Apple Silicon-based systems:

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. In the user account where you run the GitLab Runner application:

   1. [Register a runner](../register/_index.md) configuration. Choose
      the [shell executor](../executors/shell.md) during the registration process. When you build iOS or macOS applications on macOS, jobs run directly on the host, and use the identity of the authenticated user. The jobs do not run in a container, which is less secure than using container executors.
      For details, see the [security implications documentation](../security/_index.md#usage-of-shell-executor).

   1. Open a terminal and switch to the current user.

      ```shell
      su - <username>
      ```

   1. Install GitLab Runner as a service and start it:

      ```shell
      cd ~
      gitlab-runner install
      gitlab-runner start
      ```

   See the [troubleshooting section](#macos-troubleshooting) for details on resolving potential errors when running these commands.

1. Reboot your system.

If you followed these instructions, the GitLab Runner configuration file (`config.toml`) is in `/Users/<username>/.gitlab-runner/`. [Learn more about configuring runners](../configuration/advanced-configuration.md).

For more information, see the [glossary](../_index.md#glossary).

## Known issues

{{< alert type="note" >}}

The service needs to be installed from a Terminal window logged in
as your current user. Only then can you manage the service.

{{< /alert >}}

To sign in as your current user, run the command `su - <username>` in the terminal. You can obtain your username by running the command `ls /users`.

The only proven way for it to work in macOS is by running the service in user-mode.

Because the service runs only when the user is logged in, you should enable auto-login on your macOS machine.

The service is launched as a `LaunchAgent`. By using `LaunchAgents`,
the builds are able to perform UI interactions, making it possible to run and
test in the iOS simulator.

It's worth noting that macOS also has `LaunchDaemons`, services running
completely in background. `LaunchDaemons` are run on system startup, but they
don't have the same access to UI interactions as `LaunchAgents`. You can try to
run the Runner's service as a `LaunchDaemon`, but this mode of operation is not
supported.

You can verify that GitLab Runner created the service configuration file after
executing the `install` command, by checking the
`~/Library/LaunchAgents/gitlab-runner.plist` file.

If Homebrew was used to install `git`, it may have added a `/usr/local/etc/gitconfig` file
containing:

```ini
[credential]
  helper = osxkeychain
```

This tells Git to cache user credentials in the keychain, which may not be what you want
and can cause fetches to hang. You can remove the line from the system `gitconfig`
with:

```shell
git config --system --unset credential.helper
```

Alternatively, you can just disable `credential.helper` for the GitLab user:

```shell
git config --global --add credential.helper ''
```

You can check the status of the `credential.helper` with:

```shell
git config credential.helper
```

## Upgrade GitLab Runner

1. Stop the service:

   ```shell
   gitlab-runner stop
   ```

1. Download the binary to replace the GitLab Runner executable:

   - For Intel-based systems:

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - For Apple Silicon-based systems:

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Start the service:

   ```shell
   gitlab-runner start
   ```

## Upgrade the service file

To upgrade the `LaunchAgent` configuration, you must uninstall and
install the service:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## Using `codesign` with the GitLab Runner Service

If you installed `gitlab-runner` on macOS with Homebrew and your build calls
`codesign`, you may have to set `<key>SessionCreate</key><true/>` to have
access to the user keychains. GitLab does not maintain the Homebrew formula and
you should use the official binary to install GitLab Runner.

In the following example we run the builds as the `gitlab`
user and want access to the signing certificates installed by that user for code-signing:

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

## macOS troubleshooting

The following relate to troubleshooting on macOS. For more information about general troubleshooting, see [Troubleshooting GitLab Runner](../faq/_index.md).

### `killed: 9`

On Apple Silicon-based systems, you might encounter this error when running the `gitlab-runner install`, `gitlab-runner start`, or `gitlab-runner register` commands.

To resolve these errors, ensure that the directories specified in `~/Library/LaunchAgents/gitlab-runner.plist` values `StandardOutPath` and `StandardErrorPath` are writable.

In the following example, the `/Users/USERNAME/Library/LaunchAgents/gitlab-runner.plist` file has been edited to include a new writable directory, `gitlab-runner-log`, for the log files.

```xml
 <key>StandardErrorPath</key>
  <string>/Users/USERNAME/gitlab-runner-log/gitlab-runner.err.log</string>
 <key>StandardOutPath</key>
  <string>/Users/USERNAME/gitlab-runner-log/gitlab-runner.out.log</string>
</dict>

```

### Error: `"launchctl" failed: exit status 112, Could not find domain for`

This message may occur when you try to install GitLab Runner on macOS. Make sure
that you manage GitLab Runner service from the GUI Terminal application, not
the SSH connection.

### Message: `Failed to authorize rights (0x1) with status: -60007.`

If GitLab Runner is stuck on the above message when using macOS, there are two
causes to why this happens:

1. Make sure that your user can perform UI interactions:

   ```shell
   DevToolsSecurity -enable
   sudo security authorizationdb remove system.privilege.taskport is-developer
   ```

   The first command enables access to developer tools for your user.
   The second command allows the user who is member of the developer group to
   do UI interactions (for example, run the iOS simulator).

1. Make sure that your GitLab Runner service doesn't use `SessionCreate = true`.
   Previously, when running GitLab Runner as a service, we were creating
   `LaunchAgents` with `SessionCreate`. At that point (**Mavericks**), this was
   the only solution to make Code Signing work. That changed recently with
   **OS X El Capitan** which introduced a lot of new security features that
   altered this behavior.

   `SessionCreate`. However, to upgrade, you must manually
   reinstall the `LaunchAgent` script:

   ```shell
   gitlab-runner uninstall
   gitlab-runner install
   gitlab-runner start
   ```

   Then you can verify that `~/Library/LaunchAgents/gitlab-runner.plist` has
   `SessionCreate` set to `false`.

### Job error: `Failed to connect to path port 3000: Operation timed out`

If one of the jobs fails with this error, make sure the runner can connect to your GitLab instance. The connection could be blocked by things like:

- firewalls
- proxies
- permissions
- routing configurations

### Error: `FATAL: Failed to start gitlab-runner: exit status 134` on `gitlab-runner start` command

This error indicates that the GitLab Runner service is not installed properly.
Run the following commands to resolve the error:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

If the error persists, do a graphical login. A graphical login bootstraps the `LaunchAgent`, which is required to launch the service.
For more information, see [known issues](osx.md#known-issues).

The macOS instances hosted on AWS must perform [additional steps](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-to-mac-instance.html) to connect to the instance's GUI.
Use the `ssh -L` option to enable SSH port forwarding to allow a remote desktop client, like `vnc`, to connect to the remote instance.
You must also configure `AllowTcpForwarding yes` in the `/private/etc/ssh/sshd_config` on the AWS hosted macOS instance. Restart the instance to apply the change to the `sshd` configuration.
After you sign in to the GUI, repeat the GitLab Runner troubleshooting steps from a terminal in the GUI to resolve the error.

### Error: `"launchctl" failed with stderr: Load failed: 5: Input/output error`

If you encounter this error when you run the `gitlab-runner start` command, first check if the runner is already running:

```shell
gitlab-runner status
```

If the runner is already running, you don't need to start it again. If it's not running and you still encounter this error,
ensure that the directories specified in the
`~/Library/LaunchAgents/gitlab-runner.plist` values `StandardOutPath` and `StandardErrorPath` exist:

```xml
<key>StandardOutPath</key>
<string>/usr/local/var/log/gitlab-runner.out.log</string>
<key>StandardErrorPath</key>
<string>/usr/local/var/log/gitlab-runner.err.log</string>
```

If the directories do not exist, create them and ensure that the runner service user has appropriate permissions to read and write to them.
Then start the runner:

```shell
gitlab-runner start
```

### Error: `Error on fetching TLS Data from API response... error  error=couldn't build CA Chain`

The following error may occur if you upgrade to GitLab Runner v15.5.0 or later:

```plaintext
Certificate doesn't provide parent URL: exiting the loop  Issuer=Baltimore CyberTrust Root IssuerCertURL=[] Serial=33554617 Subject=Baltimore CyberTrust Root context=certificate-chain-build
Verifying last certificate to find the final root certificate  Issuer=Baltimore CyberTrust Root IssuerCertURL=[] Serial=33554617 Subject=Baltimore CyberTrust Root context=certificate-chain-build
ERROR: Error on fetching TLS Data from API response... error  error=couldn't build CA Chain: error while fetching certificates from TLS ConnectionState: error while fetching certificates into the CA Chain: couldn't resolve certificates chain from the leaf certificate: error while resolving certificates chain with verification: error while verifying last certificate from the chain: x509: “Baltimore CyberTrust Root” certificate is not permitted for this usage runner=x7kDEc9Q
```

If you encounter this error, you may need to:

1. Upgrade to GitLab Runner v15.5.1 or later.
1. Set `FF_RESOLVE_FULL_TLS_CHAIN` to `false` in the [`[runners.feature_flags]` configuration](../configuration/feature-flags.md#enable-feature-flag-in-runner-configuration). For example:

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_RESOLVE_FULL_TLS_CHAIN = false
```

Disabling this feature flag might fix TLS connectivity issues for
HTTPS endpoints that use SHA-1 signature or other deprecated root certificate signatures.
