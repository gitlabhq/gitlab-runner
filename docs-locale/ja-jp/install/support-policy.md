---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runnerのサポートポリシー
---

GitLab Runnerのサポートポリシーは、オペレーティングシステムのライフサイクルポリシーによって決定されます。

## コンテナイメージのサポート {#container-images-support}

GitLab Runnerコンテナイメージの作成に使用されるベースディストリビューション（Ubuntu、Alpine、Red Hat Universal Base Image）のサポートライフサイクルに従います。

ベースディストリビューションの公開終了日は、GitLabのメジャーリリースサイクルと必ずしも一致しません。つまり、マイナーリリースにおいて、GitLab Runnerコンテナイメージの特定のバージョンの公開を停止する場合があります。これにより、アップストリームのディストリビューションで更新されなくなったイメージを公開し続けることを防いでいます。

### コンテナイメージと公開終了日 {#container-images-and-end-of-publishing-date}

| ベースコンテナ                 | ベースコンテナのバージョン | ベンダーのサポート終了日 | GitLabのサポート終了日 |
|--------------------------------|------------------------|-----------------|-----------------|
| Ubuntu                         | 24.04                  | 2027-04-30      | 2027-05-20      |
| Ubuntu                         | 20.04                  | 2025-05-31      | 2025-06-19      |
| Alpine                         | 3.12                   | 2022-05-01      | 2023-05-22      |
| Alpine                         | 3.13                   | 2022-11-01      | 2023-05-22      |
| Alpine                         | 3.14                   | 2023-05-01      | 2023-05-22      |
| Alpine                         | 3.15                   | 2023-11-01      | 2024-01-18      |
| Alpine                         | 3.16                   | 2024-05-23      | 2024-06-22      |
| Alpine                         | 3.17                   | 2024‑11‑22      | 2024-12-22      |
| Alpine                         | 3.18                   | 2025‑05‑09      | 2025-05-22      |
| Alpine                         | 3.19                   | 2025‑11‑01      | 2025-11-22      |
| Alpine                         | 3.21                   | 2026‑11‑01      | 2026-11-22      |
| Alpine                         | latest                 |                 |                 |
| Red Hat Universal Base Image 9 | 9.5                    | 2025-04-31      | 2025-05-22      |

GitLab Runnerバージョン17.7以降は、特定のバージョンの代わりに、単一のAlpineバージョン（`latest`）のみをサポートします。Alpineバージョン3.21は、記載されているサポート終了日までサポートされます。それに対して、Ubuntu 24.04は、サポート終了日までサポートされ、その時点で最新のLTSリリースに移行します。

## Windowsバージョンのサポート {#windows-version-support}

GitLabは、Microsoft WindowsオペレーティングシステムのLTSバージョンを正式にサポートしており、Microsoft[サービスチャネル](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels)のライフサイクルポリシーに従います。

つまり、次のようになります:

- [長期サービスチャネル](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel)バージョンは、リリース日から5年間サポートされます。

  5年後、Microsoftはさらに5年間の延長サポートを提供します。この延長期間中、当社は運用上可能な範囲でサポートを提供します。GitLabのメジャーリリース時に、告知したうえでこのサポートを終了する場合があります。
- [半期チャネル](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel)バージョンは、リリース日から18か月間サポートされます。メインストリームサポートが終了すると、これらのバージョンはサポートされません。

このサポートポリシーは、配布する[Windowsバイナリ](windows.md#installation)および[Docker executor](../executors/docker.md#supported-windows-versions)に適用されます。

> [!note]
> Windowsコンテナ用のDocker executorは、コンテナがホストOSのバージョンと一致する必要があるため、厳格なバージョン要件があります。詳細については、[サポートされているWindowsコンテナの一覧](../executors/docker.md#supported-windows-versions)を参照してください。

信頼できる唯一の情報源として、<https://learn.microsoft.com/en-us/lifecycle/products/>を使用します。ここにはリリース日、メインストリームサポート終了日、延長サポート終了日が記載されています。

以下は、一般的に使用されるバージョンとそのサポート終了日の一覧です:

| オペレーティングシステム           | メインストリームサポート終了日 | 延長サポート終了日 |
|----------------------------|-----------------------------|---------------------------|
| Windows Server 2019（1809） | 2024年1月                | 2029年1月              |
| Windows Server 2022（21H2） | 2026年10月                | 2031年10月              |
| Windows Server 2025（24H2） | 2029年10月                | 2034年10月              |

### 今後のリリース {#future-releases}

Microsoftは年2回、[半期チャネル](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel)で新しいWindows Server製品をリリースし、2〜3年ごとに[長期サービスチャネル（LTSC）](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc)でWindows Severの新しいメジャーバージョンをリリースします。

GitLabは、Microsoftの公式リリース日から1か月以内に、Google Cloud Platform上で最新のWindows Serverバージョン（半期チャネル）を含む新しいGitLab Runnerヘルパーイメージをテストしてリリースすることを目指しています。提供開始日については、[サービスオプション別のWindows Serverの現行バージョン一覧](https://learn.microsoft.com/en-us/windows/release-health/windows-server-release-info#windows-server-current-versions-by-servicing-option)を参照してください。
