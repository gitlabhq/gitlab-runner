---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: CI/CDジョブ用ソフトウェア
title: GitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

[GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner)は、GitLabで定義されたCI/CDジョブを実行します。GitLab Runnerは、単一のバイナリとして実行でき、言語固有の要件はありません。

セキュリティとパフォーマンス上の理由から、GitLab Runnerは、GitLabインスタンスをホストするマシンとは別のマシンにインストールしてください。

## サポート対象のオペレーティングシステム {#supported-operating-systems}

GitLab Runnerは以下にインストールできます:

- [GitLabリポジトリ](linux-repository.md)または[手動](linux-manually.md)によるLinux
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

[Bleeding-エッジバイナリ](bleeding-edge.md)も利用できます。

別のオペレーティングシステムを使用するには、そのオペレーティングシステムがGoバイナリをビルドできることを確認してください。

## サポートされているコンテナ {#supported-containers}

GitLab Runnerは以下とともにインストールできます:

- [Docker](docker.md)
- [GitLab Helmチャート](kubernetes.md)
- [Kubernetes向けGitLabエージェント](kubernetes-agent.md)
- [GitLab Operator](operator.md)を使用する

## サポートされているアーキテクチャ {#supported-architectures}

GitLab Runnerは、次のアーキテクチャで使用できます:

- x86
- AMD64
- ARM64
- ARM
- s390x
- ppc64le
- riscv64

## システム要件 {#system-requirements}

GitLab Runnerのシステム要件は、以下によって異なります:

- CI/CDジョブの予想されるCPU負荷
- CI/CDジョブの予想されるメモリ使用量
- 同時CI/CDジョブの数
- アクティブな開発中のプロジェクト数
- 並行して作業することが予想されるデベロッパーの数

GitLab.comで利用可能なマシンの種類について詳しくは、[GitLabホストされたランナー](https://docs.gitlab.com/ci/runners/)を参照してください。

## FIPS準拠GitLab Runner {#fips-compliant-gitlab-runner}

FIPS 140-2に準拠したGitLab Runnerバイナリは、Red Hat Enterprise Linux（RHEL）ディストリビューションおよびAMD64アーキテクチャで利用できます。他のディストリビューションとアーキテクチャのサポートは、[イシュー28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814)で提案されています。

このバイナリは[Red Hat Goコンパイラ](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux)でビルドされ、FIPS 140-2で検証された暗号学的ライブラリに呼び出す。[UBI-8ミニマルイメージ](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images)は、GitLab Runner FIPSイメージを作成するためのベースとして使用されます。

RHELでFIPS準拠のGitLab Runnerを使用する方法について詳しくは、[FIPSモードへのRHELのスイッチ](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/switching-rhel-to-fips-mode_security-hardening)を参照してください。
