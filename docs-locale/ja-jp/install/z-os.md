---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: z/OSにGitLab Runnerを手動でインストールします。
title: z/OSにGitLab Runnerを手動でインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

IBM z/OS向けGitLab RunnerはGitLabによって認定されており、z/OSメインフレーム環境でCI/CDジョブをネイティブに実行できます。

[`pax`](https://www.ibm.com/docs/en/aix/7.1.0?topic=p-pax-command)アーカイブをダウンロードして、z/OSにGitLab Runnerを手動でインストールできます。

## 前提条件 {#prerequisites}

- GitLab Runnerを使用するには、プログラム一時修正（`PTFs`）を適用した次の認定プログラム分析レポート（`APARs`）が必要です:
  - z/OS 2.5
    - OA62757
    - PH45182
  - z/OS 3.1
    - OA62757
    - PH57159
- GitLab Runnerは、Shellコマンドを実行するために、`/bin/bash`にbashがインストールされていることを想定しています。bashがこの場所にインストールされていない場合は、インストール済みのバージョンへのシンボリックリンクを作成します:

  ```shell
  ln -s <TARGET_BASH> /bin/bash
  ```

## GitLab Runnerをインストールする {#install-gitlab-runner}

GitLab Runnerをインストールするには、次の手順に従います:

1. 選択したインストールディレクトリに`paxfile`をダウンロードします。
1. ご使用のシステムに対応するパッケージをインストールします:

   ```shell
   pax -ppx -rf gitlab-runner-<VERSION>.pax.Z
   ```

   インストールされたファイルは、インストール先の`gitlab-runner`ディレクトリに展開されます。

1. ファイルに実行権限を付与します:

   ```shell
   chmod +x <INSTALL_PATH>/bin/gitlab-runner
   ```

1. GitLab Runnerをエクスポートし、`PATH`に追加します:

   ```shell
   export GITLAB_RUNNER=<INSTALL_PATH>/gitlab-runner/bin
   export PATH=${GITLAB_RUNNER}:${PATH}
   ```

1. [Runnerを登録します](../register/_index.md)。

## GitLab Runnerを実行する {#run-gitlab-runner}

GitLab Runnerは直接実行するか、または開始タスクとして実行できます。

### GitLab Runnerを直接実行する {#run-gitlab-runner-directly}

実行可能ファイルを呼び出してGitLab Runnerを実行するには:

1. `<INSTALL_PATH>/bin`ディレクトリに移動します。
1. サービスを開始します:

   ```shell
   gitlab-runner start
   ```

### 開始タスクとしてGitLab Runnerを実行する {#run-gitlab-runner-as-a-started-task}

GitLab Runnerプロセスを利用可能な状態に保つには、開始タスクとして実行します。

1. 実行可能ファイルをShellスクリプト`gitlab-runner.sh`でラップします:

   ```shell
   #! /bin/sh
   <INSTALL_PATH>/bin/gitlab-runner start
   ```

1. `jcl`の開始タスクプログラムを定義し、実行して継続的なプロセスとして動作させます:

   ```jcl
   //GLRST  PROC CNFG='<PATH_TO_SCRIPT>'
   //*
   //GLRST  EXEC PGM=BPXBATSL,REGION=0M,TIME=NOLIMIT,
   //            PARM='PGM &CNFG./gitlab-runner.sh'
   //STDOUT   DD SYSOUT=*
   //STDERR   DD SYSOUT=*
   //*
   //        PEND
   ```
