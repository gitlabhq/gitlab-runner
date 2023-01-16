---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Install GitLab Runner on FreeBSD **(FREE)**

NOTE:
The FreeBSD version is also available as a [bleeding edge](bleeding-edge.md)
release. Make sure that you read the [FAQ](../faq/index.md) section which
describes some of the most common problems with GitLab Runner.

WARNING:
If you are using or upgrading from a version prior to GitLab Runner 10, read how
to [upgrade to the new version](#upgrading-to-gitlab-runner-10).

## Installing GitLab Runner

Here are the steps to install and configure GitLab Runner under FreeBSD:

1. Create the `gitlab-runner` user and group:

   ```shell
   sudo pw group add -n gitlab-runner
   sudo pw user add -n gitlab-runner -g gitlab-runner -s /usr/local/bin/bash
   sudo mkdir /home/gitlab-runner
   sudo chown gitlab-runner:gitlab-runner /home/gitlab-runner
   ```

1. Download the binary for your system:

   ```shell
   # For amd64
   sudo fetch -o /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-freebsd-amd64

   # For i386
   sudo fetch -o /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-freebsd-386
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Create an empty log file with correct permissions:

   ```shell
   sudo touch /var/log/gitlab_runner.log && sudo chown gitlab-runner:gitlab-runner /var/log/gitlab_runner.log
   ```

1. Create the `rc.d` directory in case it doesn't exist:

   ```shell
   mkdir -p /usr/local/etc/rc.d
   ```

1. Create the `gitlab_runner` script inside `rc.d`:

   Bash users can do the following:

      ```shell
      sudo bash -c 'cat > /usr/local/etc/rc.d/gitlab_runner' << "EOF"
      #!/bin/sh
      # PROVIDE: gitlab_runner
      # REQUIRE: DAEMON NETWORKING
      # BEFORE:
      # KEYWORD:

      . /etc/rc.subr

      name="gitlab_runner"
      rcvar="gitlab_runner_enable"

      user="gitlab-runner"
      user_home="/home/gitlab-runner"
      command="/usr/local/bin/gitlab-runner"
      command_args="run"
      pidfile="/var/run/${name}.pid"

      start_cmd="gitlab_runner_start"

      gitlab_runner_start()
      {
         export USER=${user}
         export HOME=${user_home}
         if checkyesno ${rcvar}; then
            cd ${user_home}
            /usr/sbin/daemon -u ${user} -p ${pidfile} ${command} ${command_args} > /var/log/gitlab_runner.log 2>&1
         fi
      }

      load_rc_config $name
      run_rc_command $1
      EOF
      ```

   If you are not using bash, create a file named `/usr/local/etc/rc.d/gitlab_runner` and include the following content:

   ```shell
      #!/bin/sh
      # PROVIDE: gitlab_runner
      # REQUIRE: DAEMON NETWORKING
      # BEFORE:
      # KEYWORD:

      . /etc/rc.subr

      name="gitlab_runner"
      rcvar="gitlab_runner_enable"

      user="gitlab-runner"
      user_home="/home/gitlab-runner"
      command="/usr/local/bin/gitlab-runner"
      command_args="run"
      pidfile="/var/run/${name}.pid"

      start_cmd="gitlab_runner_start"

      gitlab_runner_start()
      {
         export USER=${user}
         export HOME=${user_home}
         if checkyesno ${rcvar}; then
            cd ${user_home}
            /usr/sbin/daemon -u ${user} -p ${pidfile} ${command} ${command_args} > /var/log/gitlab_runner.log 2>&1
         fi
      }

      load_rc_config $name
      run_rc_command $1
   ```

1. Make the `gitlab_runner` script executable:

   ```shell
   sudo chmod +x /usr/local/etc/rc.d/gitlab_runner
   ```

1. [Register a runner](../register/index.md)
1. Enable the `gitlab-runner` service and start it:

   ```shell
   sudo sysrc gitlab_runner_enable=YES
   sudo service gitlab_runner start
   ```

   If you don't want to enable the `gitlab-runner` service to start after a
   reboot, use:

   ```shell
   sudo service gitlab_runner onestart
   ```

## Upgrading to GitLab Runner 10

To upgrade GitLab Runner from a version prior to 10.0:

1. Stop GitLab Runner:

   ```shell
   sudo service gitlab_runner stop
   ```

1. Optionally, preserve the previous version of GitLab Runner just in case:

   ```shell
   sudo mv /usr/local/bin/gitlab-ci-multi-runner{,.$(/usr/local/bin/gitlab-ci-multi-runner --version| grep Version | cut -d ':' -f 2 | sed 's/ //g')}
   ```

1. Download the new GitLab Runner and make it executable:

   ```shell
   # For amd64
   sudo fetch -o /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-freebsd-amd64

   # For i386
   sudo fetch -o /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-freebsd-386

   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Edit `/usr/local/etc/rc.d/gitlab_runner` and change:

   ```shell
   command="/usr/local/bin/gitlab-ci-multi-runner run"
   ```

   to:

   ```shell
   command="/usr/local/bin/gitlab-runner run"
   ```

1. Start GitLab Runner:

   ```shell
   sudo service gitlab_runner start
   ```

1. After you confirm all is working correctly, you can remove the old binary:

   ```shell
   sudo rm /usr/local/bin/gitlab-ci-multi-runner.*
   ```
