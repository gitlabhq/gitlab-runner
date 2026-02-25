---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerのサポートポリシー
---

GitLab Runnerのサポートポリシーは、オペレーティングシステムのライフサイクルポリシーによって決定されます。

## コンテナイメージのサポート {#container-images-support}

GitLab Runnerコンテナイメージの作成に使用されるベースイメージディストリビューション（Ubuntu、Alpine、Red Hat Universalベースイメージ）のサポートライフサイクルに従います。

ベースディストリビューションの公開終了日は、必ずしもGitLabのメジャーリリースサイクルと一致するとは限りません。つまり、マイナーリリースでは、GitLab Runnerコンテナイメージのバージョンの公開を停止します。これにより、アップストリームディストリビューションが更新しなくなったイメージは公開されなくなります。

### コンテナイメージと公開終了日 {#container-images-and-end-of-publishing-date}

| ベースコンテナ                 | ベースコンテナのバージョン | ベンダーのサービス終了日 | GitLabのサービス終了日 |
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
| Red Hat Universalベースイメージ9 | 9.5                    | 2025-04-31      | 2025-05-22      |

GitLab Runnerバージョン17.7以降は、特定のバージョンの代わりに、単一のAlpineバージョン（`latest`）のみをサポートします。Alpineバージョン3.21は、明記されているサービス終了日までサポートされます。対照的に、Ubuntu 24.04はサービス終了日までサポートされ、その時点で最新のLTSリリースに移行します。

## Windowsバージョンのサポート {#windows-version-support}

GitLabは、Microsoft WindowsオペレーティングシステムのLTSバージョンを正式にサポートしているため、Microsoftの[Servicing Channels](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels)ライフサイクルポリシーに従います。

これは、以下をサポートすることを意味します:

- [Long-Term Servicing Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel)バージョンは、リリース日から5年間サポートされます。

  5年後、Microsoftはさらに5年間の延長サポートを提供します。この延長期間中、可能な限りサポートを提供します。GitLabのメジャーリリースでは、発表をもってこのサポートを終了する場合があります。
- [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel)バージョンは、リリース日から18か月間サポートされます。メインストリームサポートが終了すると、これらのバージョンはサポートされません。

このサポートポリシーは、配布する[Windows binaries](windows.md#installation)と[Docker executor](../executors/docker.md#supported-windows-versions)に適用されます。

{{< alert type="note" >}}

WindowsコンテナのDockerexecutorには、ホストOSのバージョンと一致する必要があるため、厳格なバージョン要件があります。詳細については、[サポートされているWindowsコンテナの一覧](../executors/docker.md#supported-windows-versions)を参照してください。

{{< /alert >}}

信頼できる唯一の情報源として、<https://learn.microsoft.com/en-us/lifecycle/products/>を使用します。これには、リリース日、メインストリームサポート日、および延長サポート日が指定されています。

以下は、一般的に使用されるバージョンとそのサービス終了日の一覧です:

| オペレーティングシステム           | メインストリームサポート終了日 | 延長サポート終了日 |
|----------------------------|-----------------------------|---------------------------|
| Windows Server 2019（1809） | 2024年1月                | 2029年1月              |
| Windows Server 2022（21H2） | 2026年10月                | 2031年10月              |
| Windows Server 2025（24H2） | 2029年10月                | 2034年10月              |

### 今後のリリース {#future-releases}

Microsoftは、[Semi-Annual Channel](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel)で新しいWindows Server製品を年に2回リリースし、2〜3年ごとに、Windows Severの新しいメジャーバージョンが[Long-Term Servicing Channel（LTSC）](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc)でリリースされます。

GitLabは、Google Cloud Platform上のMicrosoftの公式リリース日から1か月以内に、最新のWindows Serverバージョン（Semi-Annual Channel）を含む新しいGitLab Runnerヘルパーイメージをテストおよびリリースすることを目指しています。利用可能日は、[サービスオプションリスト別のWindows Server現在のバージョン](https://learn.microsoft.com/en-us/windows/release-health/windows-server-release-info#windows-server-current-versions-by-servicing-option)を参照してください。
