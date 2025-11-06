---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab RunnerでサポートされているShellの種類
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、さまざまなシステムでビルドを実行できるようにするShellスクリプトジェネレーターを実装しています。

Shellスクリプトには、ビルドのすべてのステップを実行するコマンドが含まれています:

1. `git clone`
1. ビルドキャッシュの復元
1. ビルドコマンド
1. ビルドキャッシュの更新
1. ビルドアーティファクトの生成とアップロード

Shellには設定オプションはありません。[`script`の`.gitlab-ci.yml`ディレクティブ](https://docs.gitlab.com/ci/yaml/#script)で定義されたコマンドからビルドのステップを受信します。

サポートされているShellは次のとおりです:

| Shell        | 状態          | 説明 |
|--------------|-----------------|-------------|
| `bash`       | 完全にサポート | Bash（Bourne Again Shell）。すべてのコマンドはBashコンテキストで実行されます（すべてのUnixシステムのデフォルト）。 |
| `sh`         | 完全にサポート | Sh（Bourne shell）。すべてのコマンドはShコンテキストで実行されます（すべてのUnixシステムの`bash`のフォールバック）。 |
| `powershell` | 完全にサポート | PowerShellスクリプト。すべてのコマンドはPowerShell Desktopのコンテキストで実行されます。 |
| `pwsh`       | 完全にサポート | PowerShellスクリプト。すべてのコマンドはPowerShell Coreのコンテキストで実行されます。これは、Windowsで新しいRunnerを登録する際のデフォルトです。 |

デフォルト以外の特定のShellを使用する場合は、`config.toml`ファイルで[Shellを指定する](../executors/shell.md#selecting-your-shell)必要があります。

## Sh/Bash Shell {#shbash-shells}

Sh/Bashは、すべてのUnixベースのシステムで使用されるデフォルトのShellです。`.gitlab-ci.yml`で使用されているbashスクリプトは、Shellスクリプトを次のいずれかのコマンドにパイプすることで実行されます:

```shell
# This command is used if the build should be executed in context
# of another user (the shell executor)
cat generated-bash-script | su --shell /bin/bash --login user

# This command is used if the build should be executed using
# the current user, but in a login environment
cat generated-bash-script | /bin/bash --login

# This command is used if the build should be executed in
# a Docker environment
cat generated-bash-script | /bin/bash
```

### Shellプロファイルの読み込み {#shell-profile-loading}

特定のexecutorでは、Runnerは前述のように`--login`フラグを渡します。これによりShellプロファイルも読み込みまれます。`.bashrc`、`.bash_logout`、または[その他のドットファイル](https://tldp.org/LDP/Bash-Beginners-Guide/html/sect_03_01.html#sect_03_01_02)に含まれている内容はすべてジョブで実行されます。

[`Prepare environment`ステージでジョブが失敗した](../faq/_index.md#job-failed-system-failure-preparing-environment)場合、その原因はShellプロファイル内にある可能性があります。一般的な失敗として、コンソールのクリアを試行する`.bash_logout`がある場合の失敗があります。

このエラーを解決するには、`/home/gitlab-runner/.bash_logout`を確認してください。たとえば、`.bash_logout`ファイルに次のようなスクリプトセクションがある場合は、このセクションをコメントアウトしてパイプラインを再起動します:

```shell
if [ "$SHLVL" = 1 ]; then
    [ -x /usr/bin/clear_console ] && /usr/bin/clear_console -q
fi
```

Shellプロファイルを読み込むexecutor:

- [`shell`](../executors/shell.md)
- [`parallels`](../executors/parallels.md)（*ターゲット*仮想マシンのShellプロファイルが読み込みまれます）
- [`virtualbox`](../executors/virtualbox.md)（*ターゲット*仮想マシンのShellプロファイルが読み込みまれます）
- [`ssh`](../executors/ssh.md)（*ターゲット*マシンのShellプロファイルが読み込みまれます）

## PowerShell {#powershell}

PowerShell Desktop Editionは、GitLab Runner 12.0〜13.12を使用してWindowsに新しいRunnerを登録するときのデフォルトShellです。14.0以降では、デフォルトはPowerShell Core Editionです。

PowerShellは、別のユーザーのコンテキストでビルドを実行することをサポートしていません。

生成されたPowerShellスクリプトを実行するには、そのコンテンツをファイルに保存し、ファイル名を次のコマンドに渡します:

- PowerShell Desktop Edition:

  ```batch
  powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

- PowerShell Core Edition:

  ```batch
  pwsh -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

PowerShellスクリプトの例を以下に示します:

```powershell
$ErrorActionPreference = "Continue" # This will be set to 'Stop' when targetting PowerShell Core

echo "Running on $([Environment]::MachineName)..."

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  echo "Cloning repository..."
  if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path "C:\GitLab-Runner\builds\0\project-1" -PathType Container) ) {
    Remove-Item2 -Force -Recurse "C:\GitLab-Runner\builds\0\project-1"
  } elseif(Test-Path "C:\GitLab-Runner\builds\0\project-1") {
    Remove-Item -Force -Recurse "C:\GitLab-Runner\builds\0\project-1"
  }

  & "git" "clone" "https://gitlab.com/group/project.git" "Z:\Gitlab\tests\test\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Checking out db45ad9a as main..."
  & "git" "checkout" "db45ad9af9d7af5e61b829442fd893d96e31250c"
  if(!$?) { Exit $LASTEXITCODE }

  if(Test-Path "..\..\..\cache\project-1\pages\main\cache.tgz" -PathType Leaf) {
    echo "Restoring cache..."
    & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
    if(!$?) { Exit $LASTEXITCODE }

  } else {
    if(Test-Path "..\..\..\cache\project-1\pages\main\cache.tgz" -PathType Leaf) {
      echo "Restoring cache..."
      & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
      if(!$?) { Exit $LASTEXITCODE }

    }
  }
}
if(!$?) { Exit $LASTEXITCODE }

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "`$ echo true"
  echo true
}
if(!$?) { Exit $LASTEXITCODE }

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Archiving cache..."
  & "gitlab-runner-windows-amd64.exe" "archive" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz" "--path" "vendor"
  if(!$?) { Exit $LASTEXITCODE }

}
if(!$?) { Exit $LASTEXITCODE }
```

### Windows Batchの実行 {#running-windows-batch}

PowerShellに移植されていない古いBatchスクリプトの場合は、`Start-Process
"cmd.exe" "/c C:\Path\file.bat"`を使用してPowerShellからそのBatchスクリプトを実行できます。

### PowerShellがデフォルトの場合の`CMD` Shellへのアクセス {#access-cmd-shell-when-powershell-is-the-default}

[Call `CMD` From Default PowerShell in GitLab CI](https://gitlab.com/guided-explorations/microsoft/windows/call-cmd-from-powershell)プロジェクトは、`CMD` Shellへのアクセス権を取得する方法を示しています。このアプローチは、PowerShellがRunnerのデフォルトShellである場合に機能します。

### PowerShellのサンプルの使い方を紹介するビデオチュートリアル {#video-walkthrough-of-working-powershell-examples}

[Slicing and Dicing with PowerShell on GitLab CI](https://www.youtube.com/watch?v=UZvtAYwruFc)は、[PowerShell Pipelines on GitLab CI](https://gitlab.com/guided-explorations/microsoft/powershell/powershell-pipelines-on-gitlab-ci) Guided Explorationプロジェクトのチュートリアル動画です。これは以下の環境でテストされています:

- [GitLab.com向けにWindows上でホストされるrunner](https://docs.gitlab.com/ci/runners/hosted_runners/windows/)のWindows PowerShellおよびPowerShell Core 7。
- [Docker-Machine Runner](../executors/docker_machine.md)を使用したLinux ContainersのPowerShell Core 7。

この例は、テスト用に自分のグループまたはインスタンスにコピーできます。他のGitLab CIパターンのデモについての詳細は、プロジェクトページをご覧ください。
