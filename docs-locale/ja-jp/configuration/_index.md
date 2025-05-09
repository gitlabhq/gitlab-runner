---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: Config.toml, certificates, autoscaling, proxy setup.
title: GitLab Runnerを設定する
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 製品:GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerの設定方法を説明します。

- [高度な設定オプション](advanced-configuration.md):[`config.toml`](https://github.com/toml-lang/toml)設定ファイルを使用してRunnerの設定を編集します。
- [自己署名証明書を使用する](tls-self-signed.md):GitLabサーバーへの接続時にTLSピアを検証する証明書を設定します。
- [Docker Machineで自動スケールする](autoscale.md):Docker Machineによって自動的に作成されたマシンでジョブを実行します。
- [AWS EC2でGitLab Runnerを自動スケールする](runner_autoscale_aws/_index.md):自動スケールされたAWS EC2インスタンスでジョブを実行します。
- [AWS FargateでGitLab CIを自動スケールする](runner_autoscale_aws_fargate/_index.md):GitLab Custom executorでAWS Fargateドライバーを使用して、AWS ECSでジョブを実行します。
- [グラフィカルプロセッシングユニット](gpus.md):GPUを使用してジョブを実行します。
- [initシステム](init.md):GitLab Runnerは、オペレーティングシステムに基づいてinitサービスファイルをインストールします。
- [サポートされているShell](../shells/_index.md):Shellスクリプトジェネレーターを使用して、さまざまなシステムでビルドを実行します。
- [セキュリティに関する考慮事項](../security/_index.md):GitLab Runnerでジョブを実行する際のセキュリティへの潜在的な影響に注意してください。
- [Runnerのモニタリング](../monitoring/_index.md):Runnerの動作をモニタリングします。
- [Dockerキャッシュを自動的にクリーンアップする](../executors/docker.md#clear-the-docker-cache):ディスク容量が少なくなっている場合は、cronジョブを使用して古いコンテナとボリュームをクリーンアップします。
- [プロキシの背後で実行するようにGitLab Runnerを設定する](proxy.md):Linuxプロキシをセットアップし、GitLab Runnerを設定します。このセットアップは、Docker executorと適切に連携します。
- [レート制限されたリクエストを処理する](proxy.md#handling-rate-limited-requests)。
- [GitLab Runner Operatorを設定する](configuring_runner_operator.md)。
