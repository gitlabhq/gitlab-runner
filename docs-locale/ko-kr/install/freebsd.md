---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: FreeBSD 시스템에 GitLab Runner를 설치합니다.
title: FreeBSD에 GitLab Runner 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> FreeBSD 버전은 [bleeding edge](bleeding-edge.md) 릴리스로도 제공됩니다. GitLab Runner와 관련된 가장 일반적인 문제를 설명하는 [FAQ](../faq/_index.md) 섹션을 읽어야 합니다.

## GitLab Runner 설치 {#installing-gitlab-runner}

FreeBSD에 GitLab Runner를 설치하고 구성하는 단계는 다음과 같습니다:

1. `gitlab-runner` 사용자 및 그룹을 생성합니다:

   ```shell
   sudo pw group add -n gitlab-runner
   sudo pw user add -n gitlab-runner -g gitlab-runner -s /usr/local/bin/bash
   sudo mkdir /home/gitlab-runner
   sudo chown gitlab-runner:gitlab-runner /home/gitlab-runner
   ```

1. 시스템에 맞는 바이너리를 다운로드합니다:

   ```shell
   # For amd64
   sudo fetch -o /usr/local/bin/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-freebsd-amd64

   # For i386
   sudo fetch -o /usr/local/bin/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-freebsd-386
   ```

   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release)에 설명된 대로 모든 사용 가능한 버전에 대한 바이너리를 다운로드할 수 있습니다.

1. 실행 권한을 부여합니다:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. 올바른 권한으로 빈 로그 파일을 생성합니다:

   ```shell
   sudo touch /var/log/gitlab_runner.log && sudo chown gitlab-runner:gitlab-runner /var/log/gitlab_runner.log
   ```

1. 디렉터리가 없는 경우 `rc.d` 디렉터리를 생성합니다:

   ```shell
   mkdir -p /usr/local/etc/rc.d
   ```

1. `rc.d` 내에 `gitlab_runner` 스크립트를 생성합니다:

   Bash 사용자는 다음을 수행할 수 있습니다:

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

   bash를 사용하지 않는 경우 `/usr/local/etc/rc.d/gitlab_runner` 파일을 생성하고 다음 내용을 포함합니다:

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

1. `gitlab_runner` 스크립트를 실행 가능하게 합니다:

   ```shell
   sudo chmod +x /usr/local/etc/rc.d/gitlab_runner
   ```

1. [러너 등록](../register/_index.md)
1. `gitlab-runner` 서비스를 활성화하고 시작합니다:

   ```shell
   sudo sysrc gitlab_runner_enable=YES
   sudo service gitlab_runner start
   ```

   재부팅 후 `gitlab-runner` 서비스를 시작하지 않으려면 다음을 사용합니다:

   ```shell
   sudo service gitlab_runner onestart
   ```
