---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Set up macOS runners
---

To run a CI/CD job on a macOS runner, complete the following steps in order.

When you're done, GitLab Runner will be running on a macOS machine
and an individual runner will be ready to process jobs.

- Change the system shell to Bash.
- Install Homebrew, rbenv, and GitLab Runner.
- Configure rbenv and install Ruby.
- Install Xcode.
- Register a runner.
- Configure CI/CD.

## Prerequisites

Before you begin:

- Install a recent version of macOS. This guide was developed on 11.4.
- Ensure you have terminal or SSH access to the machine.

## Change the system shell to Bash

Newer versions of macOS use Zsh as the default shell. However, the runner's shell executor requires
Bash to ensure CI/CD scripts execute correctly because many use Bash-specific syntax and features.

1. Connect to your machine and determine the default shell:

   ```shell
   echo $SHELL
   ```

1. If the result is not `/bin/bash`, change the shell by running:

   ```shell
   chsh -s /bin/bash
   ```

1. Enter your password.
1. Restart your terminal or reconnect by using SSH.
1. Run `echo $SHELL` again. The result should be `/bin/bash`.

## Install Homebrew, rbenv, and GitLab Runner

The runner needs certain environment options to connect to the machine and run a job.

1. Install the [Homebrew package manager](https://brew.sh/):

   ```shell
   /bin/bash -c "$(curl "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")"
   ```

1. Set up [`rbenv`](https://github.com/rbenv/rbenv), which is a Ruby version manager, and GitLab Runner:

   ```shell
   brew install rbenv gitlab-runner
   brew services start gitlab-runner
   ```

## Configure rbenv and install Ruby

Now configure rbenv and install Ruby.

1. Add rbenv to the Bash environment:

   ```shell
   echo 'if which rbenv > /dev/null; then eval "$(rbenv init -)"; fi' >> ~/.bash_profile
   source ~/.bash_profile
   ```

1. Install Ruby 3.3.x and set it as the machine's global default:

   ```shell
   rbenv install 3.3.4
   rbenv global 3.3.4
   ```

## Install Xcode

Now install and configure Xcode.

1. Go to one of these locations and install Xcode:

   - The Apple App Store.
   - The [Apple Developer Portal](https://developer.apple.com/).
   - [`xcode-install`](https://github.com/xcpretty/xcode-install). This project aims to make it easier to download various
     Apple dependencies from the command line.

1. Agree to the license and install the recommended additional components.
   You can do this by opening Xcode and following the prompts, or by running the following command in the terminal:

   ```shell
   sudo xcodebuild -runFirstLaunch
   ```

1. Update the active developer directory so that Xcode loads the proper command line tools during your build:

   ```shell
   sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
   ```

### Create and register a project runner

Now [create and register](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token) a project runner.

When you create and register the runner:

- In GitLab, add the tag `macos` to ensure macOS jobs run on this macOS machine.
- In the command-line, select `shell` as the [executor](../executors/_index.md).

After you register the runner, a success message displays in the command-line:

```shell
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

To view the runner:

1. On the top bar, select **Search or go to** and find your project or group.
1. Select **Settings > CI/CD**.
1. Expand **Runners**.

### Configure CI/CD

In your GitLab project, configure CI/CD and start a build. You can use this sample `.gitlab-ci.yml` file.
Notice the tags match the tags you used to register the runner.

```yaml
stages:
  - build
  - test

variables:
  LANG: "en_US.UTF-8"

before_script:
  - gem install bundler
  - bundle install
  - gem install cocoapods
  - pod install

build:
  stage: build
  script:
    - bundle exec fastlane build
  tags:
    - macos

test:
  stage: test
  script:
    - bundle exec fastlane test
  tags:
    - macos
```

The macOS runner should now build your project.
