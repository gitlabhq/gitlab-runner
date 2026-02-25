---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: z/OSにGitLab Runnerを手動でインストールします。
title: z/OSにGitLab Runnerを手動でインストール
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

IBM z/OS用のGitLab RunnerはGitLabによって認定されており、z/OSメインフレーム環境でネイティブにCI/CDジョブを実行できます。

[`pax`](https://www.ibm.com/docs/en/aix/7.1.0?topic=p-pax-command)アーカイブから、z/OS上にGitLab Runnerを手動でダウンロードしてインストールできます。

## 前提条件 {#prerequisites}

- GitLab Runnerを使用するには、次のAuthorized Program Analysisレポート（`APARs`）とProgram Temporary修正（`PTFs`）が必要です:

  - z/OS 2.5
    - OA62757
    - PH45182
  - z/OS 3.1
    - OA62757
    - PH57159

- GitLab Runnerは、Shellコマンドを実行するために、`/bin/bash`にbashがインストールされていることを想定しています。bashがこの場所にインストールされていない場合は、インストールされているバージョンへのシンボリックリンクを作成します:

  ```shell
  ln -s <TARGET_BASH> /bin/bash
  ```

## GitLab Runnerをインストールする {#install-gitlab-runner}

GitLab Runnerをインストールするには、次の手順に従います。

1. 選択したインストールディレクトリに`paxfile`をダウンロードします。

1. ご使用のシステムのパッケージをインストールします:

   ```shell
   pax -ppx -rf gitlab-runner-<VERSION>.pax.Z
   ```

   インストールされたファイルは、インストール場所の`gitlab-runner`ディレクトリに展開されます。

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

## GitLab Runnerを実行 {#run-gitlab-runner}

GitLab Runnerは、直接または開始されたタスクとして実行できます。

### GitLab Runnerを直接実行 {#run-gitlab-runner-directly}

実行可能ファイルを呼び出すことによってGitLab Runnerを実行するには:

1. `<INSTALL_PATH>/bin`ディレクトリに移動します。

1. サービスを開始します。

   ```shell
   gitlab-runner start
   ```

### 開始されたタスクとしてGitLab Runnerを実行 {#run-gitlab-runner-as-a-started-task}

GitLab Runnerプロセスを使用可能な状態に保つには、開始されたタスクとして実行します。

1. 実行可能ファイルを`gitlab-runner.sh` Shellスクリプトでラップします:

   ```shell
   #! /bin/sh
   <INSTALL_PATH>/bin/gitlab-runner start
   ```

1. `jcl`開始されたタスクプログラムを定義し、継続的なプロセスとして実行するために実行します:

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
