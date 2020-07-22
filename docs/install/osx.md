# Install GitLab Runner on macOS

GitLab Runner can be installed and updated on macOS.

## Installing

There are two methods for installing GitLab Runner on macOS:

- [Manual installation](#manual-installation-official). This method is officially supported and recommended by GitLab.
- [Homebrew installation](#homebrew-installation-alternative). Install with [Homebrew](https://brew.sh) as an alternative to manual installation.

### Manual installation (official)

NOTE: **Note:**
For documentation on GitLab Runner 9 and earlier, [visit this documentation](old.md).

1. Download the binary for your system:

   ```shell
   sudo curl --output /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-darwin-amd64
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

**The rest of commands execute as the user who will run the Runner.**

1. [Register the Runner](../register/index.md)
1. Install the Runner as service and start it:

   ```shell
   cd ~
   gitlab-runner install
   gitlab-runner start
   ```

Runner is installed and will be run after a system reboot.

### Homebrew installation (alternative)

A Homebrew [formula is available](https://formulae.brew.sh/formula/gitlab-runner) to install GitLab.

CAUTION: **Caution:**
GitLab does not maintain the Homebrew formula.

To install GitLab Runner using Homebrew:

1. Install the GitLab Runner.

   ```shell
   brew install gitlab-runner
   ```

1. Install the Runner as a service and start it.

   ```shell
   brew services start gitlab-runner
   ```

Runner is installed and running.

### Limitations on macOS

>**Note:**
The service needs to be installed from the Terminal by running its GUI
interface as your current user. Only then will you be able to manage the service.

Currently, the only proven to work mode for macOS is running service in user-mode.

Since the service will be running only when the user is logged in, you should
enable auto-login on your macOS machine.

The service will be launched as one of `LaunchAgents`. By using `LaunchAgents`,
the builds will be able to do UI interactions, making it possible to run and
test on the iOS simulator.

It's worth noting that macOS also has `LaunchDaemons`, the services running
completely in background. `LaunchDaemons` are run on system startup, but they
don't have the same access to UI interactions as `LaunchAgents`. You can try to
run the Runner's service as `LaunchDaemon`, but this mode of operation is not
currently supported.

You can verify that the Runner created the service configuration file after
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

## Manual update

1. Stop the service:

   ```shell
   gitlab-runner stop
   ```

1. Download the binary to replace the Runner's executable:

   ```shell
   sudo curl -o /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-darwin-amd64
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

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

## Upgrade the service file

In order to upgrade the `LaunchAgent` configuration, you need to uninstall and
install the service:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## Using codesign with the GitLab Runner Service

If you installed `gitlab-runner` on macOS with homebrew and your build calls
`codesign`, you may need to set `<key>SessionCreate</key><true/>` to have
access to the user keychains. In the following example we run the builds as the `gitlab`
user and want access to the signing certificates installed by that user for codesigning:

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
