---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner最新リリース
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< alert type="warning" >}}

これらのGitLab Runnerのリリースは最新であり、`main`ブランチから直接ビルドされているため、テストされていない可能性があります。ご自身の責任においてご利用ください。

{{< /alert >}}

## スタンドアロンバイナリをダウンロードする {#download-the-standalone-binaries}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-arm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-s390x>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-riscv64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-darwin-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-386.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-amd64.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-arm>

その後、次のコマンドを使用してGitLab Runnerを実行できます。

```shell
chmod +x gitlab-runner-linux-amd64
./gitlab-runner-linux-amd64 run
```

## DebianまたはUbuntu用のパッケージをダウンロードする {#download-one-of-the-packages-for-debian-or-ubuntu}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_i686.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_amd64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_armel.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_armhf.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_arm64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_aarch64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_riscv64.deb>

### エクスポートされたrunner-helperイメージパッケージをダウンロードする {#download-the-exported-runner-helper-images-package}

runner-helperイメージパッケージは、GitLab Runner `.deb`パッケージに必要な依存関係です。

次の場所からパッケージをダウンロードします。

```plaintext
https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb
```

その後、次のコマンドを使用してインストールできます。

```shell
dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
```

## Red HatまたはCentOS用のパッケージをダウンロードする {#download-one-of-the-packages-for-red-hat-or-centos}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_i686.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_amd64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_arm.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_armhf.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_arm64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_aarch64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_riscv64.rpm>

### エクスポートされたrunner-helperイメージパッケージをダウンロードする {#download-the-exported-runner-helper-images-package-1}

runner-helperイメージパッケージは、GitLab Runner `.rpm`パッケージに必要な依存関係です。

次の場所からパッケージをダウンロードします。

```plaintext
https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm
```

その後、次のコマンドを使用してインストールできます。

```shell
rpm -i gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## その他のタグ付きリリースをダウンロードする {#download-any-other-tagged-release}

`main`を`tag`（`v16.5.0`など）または`latest`（最新の安定版）のいずれかに置き換えます。タグの一覧については、<https://gitlab.com/gitlab-org/gitlab-runner/-/tags>を参照してください。次に例を示します。

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v16.5.0/binaries/gitlab-runner-linux-386>

`https`経由でのダウンロードに問題がある場合は、プレーンな`http`にフォールバックします。

- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386>
- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v16.5.0/binaries/gitlab-runner-linux-386>
