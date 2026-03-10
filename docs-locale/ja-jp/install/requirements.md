---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: CI/CDジョブ用ソフトウェア
title: システム要件とサポートされているプラットフォーム
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

## サポートされているオペレーティングシステム {#supported-operating-systems}

GitLab Runnerは次の環境にインストールできます:

- [GitLabリポジトリ](linux-repository.md)または[手動](linux-manually.md)でLinuxに
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

[最先端バイナリ](bleeding-edge.md)も利用可能です。

別のオペレーティングシステムを使用するには、そのオペレーティングシステムがGoバイナリをコンパイルできることを確認してください。

## サポートされているコンテナ {#supported-containers}

GitLab Runnerは以下を使用してインストールできます:

- [Docker](docker.md)
- [The GitLab Helmチャート](kubernetes.md)
- [The Kubernetes向けGitLabエージェント](kubernetes-agent.md)
- [The GitLab Operator](operator.md)

## サポートされているアーキテクチャ {#supported-architectures}

GitLab Runnerは以下のアーキテクチャで利用可能です:

- x86
- AMD64
- ARM64
- ARM
- s390x
- ppc64le
- riscv64
- loong64

## システム要件 {#system-requirements}

GitLab Runnerのシステム要件は、以下の考慮事項によって異なります:

- CI/CDジョブの予想されるCPU負荷
- CI/CDジョブの予想されるメモリ使用量
- 同時実行されるCI/CDジョブの数
- アクティブに開発されているプロジェクトの数
- 並行して作業するデベロッパーの予想数

GitLab.comで利用可能なマシンタイプについては、[GitLabホスト型Runner](https://docs.gitlab.com/ci/runners/)を参照してください。

## FIPS準拠のGitLab Runner {#fips-compliant-gitlab-runner}

FIPS 140-2準拠のGitLab Runnerバイナリは、Red Hat Enterprise Linux（RHEL）ディストリビューションおよびAMD64アーキテクチャで利用可能です。他のディストリビューションおよびアーキテクチャのサポートは、[28814イシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814)で提案されています。

このバイナリは、[Red Hat Goコンパイラ](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux)でビルドされており、FIPS 140-2で検証された暗号学的ライブラリを呼び出しています。A [UBI-8 minimal image](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images)は、GitLab Runner FIPSイメージを作成するためのベースとして使用されます。

RHELでFIPS準拠のGitLab Runnerを使用する方法の詳細については、[Switching RHEL to FIPS mode](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/switching-rhel-to-fips-mode_security-hardening)を参照してください。
