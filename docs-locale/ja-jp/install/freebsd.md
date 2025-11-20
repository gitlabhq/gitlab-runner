---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: FreeBSDにGitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< alert type="note" >}}

FreeBSDバージョンも[bleeding edge](bleeding-edge.md)リリースとして利用できます。[FAQ](../faq/_index.md)セクションを参照してください。このセクションでは、GitLab Runnerに関する最も一般的な問題について説明しています。

{{< /alert >}}

## GitLab Runnerのインストール {#installing-gitlab-runner}

FreeBSDにGitLab Runnerをインストールして構成する手順は次のとおりです:

1. `gitlab-runner`ユーザーとグループを作成します:

   ```shell
   sudo pw group add -n gitlab-runner
   sudo pw user add -n gitlab-runner -g gitlab-runner -s /usr/local/bin/bash
   sudo mkdir /home/gitlab-runner
   sudo chown gitlab-runner:gitlab-runner /home/gitlab-runner
   ```

1. ご使用のシステムに対応するバイナリをダウンロードします:

   ```shell
   # For amd64
   sudo fetch -o /usr/local/bin/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-freebsd-amd64

   # For i386
   sudo fetch -o /usr/local/bin/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-freebsd-386
   ```

   [Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。

1. 実行権限を付与します:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. 正しい権限で空のログファイルを作成します:

   ```shell
   sudo touch /var/log/gitlab_runner.log && sudo chown gitlab-runner:gitlab-runner /var/log/gitlab_runner.log
   ```

1. `rc.d`ディレクトリが存在しない場合は作成します:

   ```shell
   mkdir -p /usr/local/etc/rc.d
   ```

1. `rc.d`内に`gitlab_runner`スクリプトを作成します:

   Bashユーザーは以下を実行できます:

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

   bashを使用していない場合は、`/usr/local/etc/rc.d/gitlab_runner`という名前のファイルを作成し、次のコンテンツを含めます:

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

1. `gitlab_runner`スクリプトを実行可能にします:

   ```shell
   sudo chmod +x /usr/local/etc/rc.d/gitlab_runner
   ```

1. [Runnerを登録する](../register/_index.md)
1. `gitlab-runner`サービスを有効にして開始します:

   ```shell
   sudo sysrc gitlab_runner_enable=YES
   sudo service gitlab_runner start
   ```

   再起動後に`gitlab-runner`サービスを起動したくない場合は、次を使用します:

   ```shell
   sudo service gitlab_runner onestart
   ```
