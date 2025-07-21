---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: Prometheusメトリクス。
title: GitLab Runnerの使用状況をモニタリングする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

[Prometheus](https://prometheus.io)を使用してGitLab Runnerをモニタリングできます。

## 埋め込みPrometheusメトリクス {#embedded-prometheus-metrics}

GitLab RunnerにはネイティブのPrometheusメトリクスが組み込まれており、`/metrics`パス上の埋め込みHTTPサーバーを使用して公開できます。このサーバーが有効になっている場合、Prometheusモニタリングシステムによりスクレイピングしたり、他のHTTPクライアントでアクセスしたりできます。

公開される情報には以下のものが含まれます。

- Runnerのビジネスロジックメトリクス（現時点で実行中のジョブの数など）
- Go固有のプロセスメトリクス（ガベージコレクションの統計、goroutine、memstatなど）
- 一般的なプロセスメトリクス（メモリ使用量、CPU使用量、ファイル記述子の使用量など）
- ビルドバージョン情報

メトリクスの形式は、Prometheusの[公開形式](https://prometheus.io/docs/instrumenting/exposition_formats/)の仕様に記載されています。

これらのメトリクスは、オペレーターがRunnerをモニタリングしてインサイトを得るための手段として提供されています。たとえば、Runnerホストの負荷平均の増加が、処理されたジョブの増加に関連しているかどうかを確認できます。あるいは、マシンのクラスターを実行しており、インフラストラクチャに変更を加えるために、ビルドの傾向を追跡することがあります。

### Prometheusについて詳しく理解する {#learning-more-about-prometheus}

このHTTPエンドポイントをスクレイピングし、収集されたメトリクスを使用するようにPrometheusサーバーを設定するには、Prometheusの[入門](https://prometheus.io/docs/prometheus/latest/getting_started/)ガイドを参照してください。Prometheusの設定方法の詳細については、[設定](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)セクションを参照してください。アラート設定の詳細については、[アラートルール](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)と[Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/)を参照してください。

## 利用可能なメトリクス {#available-metrics}

利用可能なすべてのメトリクスのリストを確認するには、メトリクスエンドポイントを設定して有効にした後に、メトリクスエンドポイントに対して`curl`を実行します。たとえば、リッスンポート`9252`を使用して設定されているローカルRunnerの場合は次のようになります。

```shell
$ curl -s "http://localhost:9252/metrics" | grep -E "# HELP"

# HELP gitlab_runner_api_request_statuses_total The total number of api requests, partitioned by runner, endpoint and status.
# HELP gitlab_runner_autoscaling_machine_creation_duration_seconds Histogram of machine creation time.
# HELP gitlab_runner_autoscaling_machine_states The current number of machines per state in this provider.
# HELP gitlab_runner_concurrent The current value of concurrent setting
# HELP gitlab_runner_errors_total The number of caught errors.
# HELP gitlab_runner_limit The current value of limit setting
# HELP gitlab_runner_request_concurrency The current number of concurrent requests for a new job
# HELP gitlab_runner_request_concurrency_exceeded_total Count of excess requests above the configured request_concurrency limit
# HELP gitlab_runner_version_info A metric with a constant '1' value labeled by different build stats fields.
...
```

リストには[Go固有のプロセスメトリクス](https://github.com/prometheus/client_golang/blob/v1.19.0/prometheus/go_collector.go)が含まれています。Go固有のプロセスを含まない利用可能なメトリクスのリストについては、[Runnerのモニタリング](../fleet_scaling/_index.md#monitoring-runners)を参照してください。

## `pprof` HTTPエンドポイント {#pprof-http-endpoints}

メトリクスによるGitLab Runnerプロセスの内部状態の情報は貴重ですが、場合によっては、実行中のプロセスをリアルタイムで調べる必要があります。この目的で`pprof` HTTPエンドポイントを導入しました。

`pprof`エンドポイントは、`/debug/pprof/`パス上の埋め込みHTTPサーバーを介して利用できます。

`pprof`の使用方法の詳細については、その[ドキュメント](https://pkg.go.dev/net/http/pprof)を参照してください。

## メトリクスHTTPサーバーの設定 {#configuration-of-the-metrics-http-server}

{{< alert type="note" >}}

メトリクスサーバーは、GitLab Runnerプロセスの内部状態に関するデータをエクスポートするため、一般に公開すべきではありません。

{{< /alert >}}

次のいずれかの方法を使用して、メトリクスHTTPサーバーを設定します。

- `config.toml`ファイルで`listen_address`グローバル設定オプションを使用します。
- `run`コマンドの`--listen-address`コマンドラインオプションを使用します。
- Helm Chartを使用するRunnerの場合は、`values.yaml`で次の手順に従います。

  1. `metrics`オプションを設定します。

     ```yaml
     ## Configure integrated Prometheus metrics exporter
     ##
     ## ref: https://docs.gitlab.com/runner/monitoring/#configuration-of-the-metrics-http-server
     ##
     metrics:
       enabled: true

       ## Define a name for the metrics port
       ##
       portName: metrics

       ## Provide a port number for the integrated Prometheus metrics exporter
       ##
       port: 9252

       ## Configure a prometheus-operator serviceMonitor to allow autodetection of
       ## the scraping target. Requires enabling the service resource below.
       ##
       serviceMonitor:
         enabled: true

         ...
     ```

  1. 設定されている`metrics`を取得するように`service`モニターを設定します。

     ```yaml
     ## Configure a service resource to allow scraping metrics by uisng
     ## prometheus-operator serviceMonitor
     service:
       enabled: true

       ## Provide additonal labels for the service
       ##
       labels: {}

       ## Provide additonal annotations for the service
       ##
       annotations: {}

       ...
     ```

`config.toml`ファイルにアドレスを追加する場合は、メトリクスHTTPサーバーを起動するために、Runnerプロセスを再起動する必要があります。

どちらの場合も、オプションは`[host]:<port>`形式の文字列を受け入れます。各要素の意味は次のとおりです。

- `host`には、IPアドレスまたはホスト名を使用できます。
- `port`は、有効なTCPポートまたはシンボリックサービス名（`http`など）です。すでに[Prometheusに割り当てられている](https://github.com/prometheus/prometheus/wiki/Default-port-allocations)ポート`9252`を使用する必要があります。

リッスンアドレスにポートが含まれていない場合は、デフォルトで`9252`になります。

アドレスの例:

- `:9252`は、ポート`9252`のすべてのインターフェースでリッスンします。
- `localhost:9252`は、ポート`9252`のループバックインターフェースでリッスンします。
- `[2001:db8::1]:http`は、HTTPポート`80`のIPv6アドレス`[2001:db8::1]`でリッスンします。

少なくともLinux/Unixシステムでは、`1024`より下のポートでリッスンするにはルート/管理者権限が必要であることに注意してください。

HTTPサーバーは、選択されている`host:port`で**認証なしで**開きます。メトリクスサーバーをパブリックインターフェースにバインドする場合は、ファイアウォールを使用してアクセスを制限するか、認証およびアクセス制御のためにHTTPプロキシを追加します。
