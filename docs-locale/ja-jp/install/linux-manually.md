---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: LinuxにGitLab Runnerのバイナリを手動でダウンロードしてインストールします。
title: GNU/LinuxにGitLab Runnerを手動でインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、`deb`パッケージ、`rpm`パッケージ、またはバイナリファイルを使用して手動でインストールできます。この方法は、以下の状況で最後の手段として使用してください。

- deb/rpmリポジトリを使用してGitLab Runnerをインストールできない場合
- ご使用のGNU/Linux OSがサポートされていない場合

## 前提条件 {#prerequisites}

GitLab Runnerを手動で実行する前に:

- Docker executorを使用する予定がある場合は、まずDockerをインストールしてください。
- よくある問題と解決策については、FAQセクションを確認してください。

## deb/rpmパッケージを使用する {#using-debrpm-package}

`deb`パッケージまたは`rpm`パッケージを使用して、GitLab Runnerをダウンロードしてインストールできます。

### ダウンロード {#download}

システムに対応するパッケージをダウンロードするには、次の手順に従います。

1. 最新のファイル名とオプションを<https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html>で確認します。
1. パッケージマネージャーまたはアーキテクチャに対応するRunner-helperバージョンをダウンロードします。
1. GitLab Runner bleeding edgeリリースの[その他のタグ付きリリースのダウンロード](bleeding-edge.md#download-any-other-tagged-release)に関するドキュメントの説明に従って、バージョンを選択し、バイナリをダウンロードします。

たとえば、DebianまたはUbuntuの場合は次のようになります。

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner_${arch}.deb"
```

たとえば、CentOSまたはRed Hat Enterprise Linuxの場合は次のようになります。

```shell
# Replace ${arch} with any of the supported architectures, e.g. x86_64, aarch64, armhfp
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_${arch}.rpm"
```

例えば、RHEL上のFIPS準拠GitLab Runnerの場合:

```shell
# Currently only x86_64 is a supported arch
# The FIPS compliant GitLab Runner version continues to include the helper images in one package.
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_x86_64-fips.rpm"
```

### インストール {#install}

1. ご使用のシステムに対応するパッケージを次のようにインストールします。

   たとえば、DebianまたはUbuntuの場合は次のようになります。

   ```shell
   dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
   ```

   たとえば、CentOSまたはRed Hat Enterprise Linuxの場合は次のようになります。

   ```shell
   dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
   ```

### アップグレード {#upgrade}

ご使用のシステムに対応する最新パッケージをダウンロードし、次のようにしてアップグレードします。

たとえば、DebianまたはUbuntuの場合は次のようになります。

```shell
dpkg -i gitlab-runner_<arch>.deb
```

たとえば、CentOSまたはRed Hat Enterprise Linuxの場合は次のようになります。

```shell
dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## バイナリファイルを使用する {#using-binary-file}

バイナリファイルを使用して、GitLab Runnerをダウンロードしてインストールできます。

### インストール {#install-1}

1. ご使用のシステムに対応するバイナリのいずれかをダウンロードします。

   ```shell
   # Linux x86-64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"

   # Linux x86
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386"

   # Linux arm
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm"

   # Linux arm64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm64"

   # Linux s390x
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-s390x"

   # Linux ppc64le
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-ppc64le"

   # Linux riscv64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-riscv64"

   # Linux loong64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-loong64"

   # Linux x86-64 FIPS Compliant
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64-fips"
   ```

   [Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。

1. 実行のための権限を付与します。

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. GitLab CIユーザーを作成します。

   ```shell
   sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash
   ```

1. インストールしてサービスとして実行します。

   ```shell
   sudo gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
   sudo gitlab-runner start
   ```

   rootの`$PATH`に`/usr/local/bin/`があることを確認してください。ない場合は、`command not found`エラーが発生する可能性があります。または、`gitlab-runner`を`/usr/bin/`のような別の場所にインストールすることもできます。

> [!note]
> `gitlab-runner`がサービスとしてインストールされ実行される場合、rootとして実行されますが、`install`コマンドで指定されたユーザーとしてジョブを実行します。つまり、キャッシュやアーティファクトなどの一部のジョブ機能は`/usr/local/bin/gitlab-runner`コマンドを実行する必要があります。したがって、ジョブ実行ユーザーが実行可能ファイルにアクセスできる必要があります。

### アップグレード {#upgrade-1}

1. サービスを停止します（以前と同様に、管理者権限でのコマンドプロンプトが必要です）。

   ```shell
   sudo gitlab-runner stop
   ```

1. GitLab Runner実行可能ファイルを置き換えるバイナリをダウンロードします。例: 

   ```shell
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"
   ```

   [Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。

1. 実行のための権限を付与します。

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. サービスを開始します:

   ```shell
   sudo gitlab-runner start
   ```

## 次の手順 {#next-steps}

インストール後、セットアップを完了するために[Runnerを登録](../register/_index.md)してください。

Runnerバイナリには、ビルド済みのヘルパーイメージは含まれていません。これらのコマンドを使用して、対応するバージョンのヘルパーイメージアーカイブをダウンロードし、適切な場所にコピーできます:

```shell
mkdir -p /usr/local/bin/out/helper-images
cd /usr/local/bin/out/helper-images
```

アーキテクチャに適したヘルパーイメージを選択してください:

<details>
<summary>Ubuntuヘルパーイメージ</summary>

```shell
# Linux x86-64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64.tar.xz

# Linux x86-64 ubuntu pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64-pwsh.tar.xz

# Linux s390x ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-s390x.tar.xz

# Linux ppc64le ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-ppc64le.tar.xz

# Linux arm64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm64.tar.xz

# Linux arm ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm.tar.xz

# Linux x86-64 ubuntu specific version - v17.10.0
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v17.10.0/helper-images/prebuilt-ubuntu-x86_64.tar.xz
```

</details>

<details>
<summary>alpineヘルパーイメージ</summary>

```shell
# Linux x86-64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64.tar.xz

# Linux x86-64 alpine pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64-pwsh.tar.xz

# Linux s390x alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-s390x.tar.xz

# Linux riscv64 alpine edge
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-edge-riscv64.tar.xz

# Linux arm64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm64.tar.xz

# Linux arm alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm.tar.xz
```

</details>

## 追加リソース {#additional-resources}

- [Docker executorのドキュメント](../executors/docker.md)
- [Dockerをインストールする](https://docs.docker.com/engine/install/centos/)
- [その他のRunnerバージョンをダウンロード](bleeding-edge.md#download-any-other-tagged-release)
- [連邦情報処理規格準拠のRunner情報](requirements.md#fips-compliant-gitlab-runner)
- [Runner FAQ](../faq/_index.md)
- [deb/rpmリポジトリのインストール](linux-repository.md)
