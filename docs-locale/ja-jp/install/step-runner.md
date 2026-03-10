---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab Functionsを使用するために、step runnerを手動でインストールします。
title: step runnerを手動でインストールします
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

step runnerは、ネイティブ関数をサポートしないexecutorでGitLab RunnerがGitLab Functionsを実行できるようにするバイナリです。これらのexecutorでは、パイプラインで関数を使用する前に、ジョブが実行されるホストまたはコンテナにstep runnerのバイナリをインストールする必要があります。

## 手動でのstep runnerインストールが必要なexecutor {#executors-that-require-manual-step-runner-installation}

step runnerを手動でインストールする必要があるかどうかは、お使いのexecutorによって異なります。以下の表は、手動でのstep runnerのインストールが必要なexecutorを示しています:

| executor          | 手動インストールが必要 |
|-------------------|------------------------------|
| Shell             | はい                          |
| SSH               | はい                          |
| Kubernetes        | はい                          |
| VirtualBox        | はい                          |
| Parallels         | はい                          |
| カスタム            | はい                          |
| インスタンス          | はい                          |
| Docker            | Windowsのみ              |
| Docker Autoscaler | Windowsのみ              |
| Docker Machine    | Windowsのみ              |

手動インストールが不要なexecutorの場合、`gitlab-runner-helper`がstep runnerとして機能します。これらのexecutorには、`step-runner`バイナリは存在せず、必要もありません。

### 変数アクセス制限 {#variable-access-restrictions}

step runnerを手動でインストールしたexecutorでは、step runnerはジョブ変数と環境変数へのアクセスが制限されます:

| 構文               | 利用可能な値                                                                                                                                                                        |
|----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `${{ vars.<name> }}` | プレフィックスが`CI_`、`DOCKER_`、または`GITLAB_`のジョブ変数のみ。                                                                                                                      |
| `${{ env.<name> }}`  | `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`, `http_proxy`, `https_proxy`, `no_proxy`, `all_proxy`, `LANG`, `LC_ALL`, `LC_CTYPE`, `LOGNAME`, `USER`, `PATH`, `SHELL`, `TERM`, `TMPDIR`, `TZ` |

## step runnerを手動でインストールします {#install-step-runner-manually}

複数のプラットフォーム向けのコンパイルされたバイナリは、[step runnerのリリースページ](https://gitlab.com/gitlab-org/step-runner/-/releases)から入手できます。サポートされているプラットフォームには、Windows、Linux、macOS、およびFreeBSDがあり、複数のアーキテクチャ（amd64、arm64、386、ARM、s390x、ppc64le）に対応しています。

### バイナリの信頼性を検証します {#verify-authenticity-of-the-binary}

インストールする前に、バイナリが改ざんされておらず、公式のGitLabチームから提供されていることを確認してください。

1. GPG公開キーをダウンロードしてインポートします:

   ```shell
   # All platforms (requires gpg installed: https://gnupg.org/download/)
   curl -o step-runner.pub.gpg "https://gitlab.com/gitlab-org/step-runner/-/package_files/257922684/download"
   gpg --import step-runner.pub.gpg
   gpg --fingerprint
   ```

   インポートしたキーが以下と一致することを確認してください:

   | キー属性 | 値                                                |
   |---------------|------------------------------------------------------|
   | 名前          | `GitLab, Inc.`                                       |
   | メール         | `support@gitlab.com`                                 |
   | フィンガープリント   | `0FCD 59B1 6F4A 62D0 3839  27A5 42FF CA71 62A5 35F5` |
   | 有効期限        | `2029-01-05`                                         |

1. [リリースページ](https://gitlab.com/gitlab-org/step-runner/-/releases)から、以下のファイルをダウンロードしてください:

   - お使いのプラットフォーム用のバイナリ（例: `step-runner-linux-amd64`または`step-runner-darwin-arm64`）
   - `step-runner-release.sha256`
   - `step-runner-release.sha256.asc`

1. GPG署名を検証します:

   ```shell
   # All platforms (requires gpg)
   gpg --verify step-runner-release.sha256.asc step-runner-release.sha256
   ```

   出力には`Good signature`メッセージが含まれているはずです。

1. バイナリのチェックサムを検証します:

   ```shell
   # Linux
   sha256sum -c step-runner-release.sha256
   ```

   ```shell
   # macOS
   shasum -a 256 -c step-runner-release.sha256
   ```

   ```shell
   # Windows (PowerShell) — replace 'step-runner-windows-amd64.exe' with your binary name
   $binary = "step-runner-windows-amd64.exe"
   $expected = (Select-String -Path "step-runner-release.sha256" -Pattern $binary).Line.Split(" ")[0]
   $actual = (Get-FileHash -Algorithm SHA256 $binary).Hash.ToLower()
   if ($actual -eq $expected) { "OK" } else { "FAILED: checksum mismatch" }
   ```

   出力には、お使いのバイナリに対して`OK`が表示されるはずです。

### step-runnerをPATHに追加します {#add-step-runner-to-path}

バイナリをダウンロードして検証したら、ジョブが実行されるインスタンスの`PATH`で利用できるようにします。このインスタンスは、executorによってはホストマシンまたはコンテナの場合があります。

1. バイナリを`step-runner`（Windowsでは`step-runner.exe`）に名前変更します:

   ```shell
   mv step-runner-<os>-<arch> step-runner
   ```

1. Unix系システムでは、バイナリを実行可能にします:

   ```shell
   chmod +x step-runner
   ```

1. バイナリを`PATH`上のディレクトリに移動します:

   ```shell
   mv step-runner /usr/local/bin/
   ```
