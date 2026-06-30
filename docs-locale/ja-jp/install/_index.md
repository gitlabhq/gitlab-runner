---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: CI/CDジョブ用ソフトウェア
title: GitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

[GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner)は、GitLabで定義されたCI/CDジョブを実行します。GitLab Runnerは単一のバイナリとして実行でき、言語に固有の要件はありません。

セキュリティとパフォーマンスの理由から、GitLab Runnerは、GitLabインスタンスをホストするマシンとは別のマシンにインストールしてください。

インストールする前に、[システム要件とサポートされているプラットフォーム](requirements.md)を確認してください。

## オペレーティングシステム {#operating-systems}

{{< cards >}}

- [Linux](linux-repository.md)
- [Linux手動インストール](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

{{< /cards >}}

## コンテナ {#containers}

{{< cards >}}

- [Docker](docker.md)
- [Helmチャート](kubernetes.md)
- [GitLabエージェント](kubernetes-agent.md)
- [Operator](operator.md)

{{< /cards >}}

## その他のインストールオプション {#other-installation-options}

{{< cards >}}

- [最新リリース](bleeding-edge.md)

{{< /cards >}}
