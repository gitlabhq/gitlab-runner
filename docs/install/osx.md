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

   ```bash
   sudo curl --output /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-darwin-amd64
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```bash
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

**The rest of commands execute as the user who will run the Runner.**

1. [Register the Runner](../register/index.md)
1. Install the Runner as service and start it:

   ```bash
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

   ```bash
   brew install gitlab-runner
   ```

1. Install the Runner as a service and start it.

   ```bash
   brew services start gitlab-runner
   ```

Runner is installed and running.

### Limitations on macOS

>**Note:**
The service needs to be installed from the Terminal by running its GUI
interface as your current user. Only then will you be able to manage the service.

Currently, the only proven to work mode for macOS is running service in user-mode.

Since the service will be running only when the user is logged in, you should
enable auto-login on your OSX machine.

The service will be launched as one of `LaunchAgents`. By using `LaunchAgents`,
the builds will be able to do UI interactions, making it possible to run and
test on the iOS simulator.

It's worth noting that OSX also has `LaunchDaemons`, the services running
completely in background. `LaunchDaemons` are run on system startup, but they
don't have the same access to UI interactions as `LaunchAgents`. You can try to
run the Runner's service as `LaunchDaemon`, but this mode of operation is not
currently supported.

You can verify that the Runner created the service configuration file after
executing the `install` command, by checking the
`~/Library/LaunchAgents/gitlab-runner.plist` file.

## Manual update

1. Stop the service:

   ```bash
   gitlab-runner stop
   ```

1. Download the binary to replace the Runner's executable:

   ```bash
   sudo curl -o /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-darwin-amd64
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```bash
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Start the service:

   ```bash
   gitlab-runner start
   ```

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

## Upgrade the service file

In order to upgrade the `LaunchAgent` configuration, you need to uninstall and
install the service:

```bash
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```
