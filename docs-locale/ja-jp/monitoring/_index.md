---
stage: Verify
group: Runner Core
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

GitLab RunnerにはネイティブのPrometheusメトリクスが含まれており、`/metrics`パス上の埋め込みHTTPサーバーを使用して公開できます。このサーバーが有効になっている場合、Prometheusモニタリングシステムによりスクレイピングしたり、他のHTTPクライアントでアクセスしたりできます。

公開される情報には以下のものが含まれます:

- Runnerのビジネスロジックメトリクス（現時点で実行中のジョブの数など）
- Go固有のプロセスメトリクス（ガベージコレクションの統計、goroutine、memstatなど）
- 一般的なプロセスメトリクス（メモリ使用量、CPU使用量、ファイル記述子の使用量など）
- ビルドバージョン情報

メトリクスの形式は、Prometheusの[公開形式](https://prometheus.io/docs/instrumenting/exposition_formats/)の仕様に記載されています。

これらのメトリクスは、オペレーターがRunnerをモニタリングしてインサイトを得るための手段として提供されています。たとえば、Runnerホストの負荷平均の増加が、処理されたジョブの増加に関連しているかどうかを確認できます。あるいは、マシンのクラスターを実行しており、インフラストラクチャに変更を加えるために、ビルドの傾向を追跡することがあります。

### Prometheusについて詳しく理解する {#learning-more-about-prometheus}

このHTTPエンドポイントをスクレイピングし、収集されたメトリクスを使用するようにPrometheusサーバーを設定するには、Prometheusの[入門](https://prometheus.io/docs/prometheus/latest/getting_started/)ガイドを参照してください。Prometheusの設定方法の詳細については、[設定](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)セクションを参照してください。アラート設定の詳細については、[アラートルール](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)と[Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/)を参照してください。

## 利用可能なメトリクス {#available-metrics}

利用可能なすべてのメトリクスのリストを確認するには、メトリクスエンドポイントを設定して有効にした後に、メトリクスエンドポイントに対して`curl`を実行します。たとえば、リッスンポート`9252`を使用して設定されているローカルRunnerの場合は次のようになります:

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

次のいずれかの方法を使用して、メトリクスHTTPサーバーを設定します:

- `config.toml`ファイルで`listen_address`グローバル設定オプションを使用します。
- `run`コマンドの`--listen-address`コマンドラインオプションを使用します。
- Helm Chartを使用するRunnerの場合は、`values.yaml`で次の手順に従います:

  1. `metrics`オプションを設定します:

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

       ## Configure a prometheus-operator serviceMonitor to allow automatic detection of
       ## the scraping target. Requires enabling the service resource below.
       ##
       serviceMonitor:
         enabled: true

         ...
     ```

  1. 設定されている`metrics`を取得するように`service`モニターを設定します:

     ```yaml
     ## Configure a service resource to allow scraping metrics by using
     ## prometheus-operator serviceMonitor
     service:
       enabled: true

       ## Provide additional labels for the service
       ##
       labels: {}

       ## Provide additional annotations for the service
       ##
       annotations: {}

       ...
     ```

`config.toml`ファイルにアドレスを追加する場合は、メトリクスHTTPサーバーを起動するために、Runnerプロセスを再起動する必要があります。

どちらの場合も、オプションは`[host]:<port>`形式の文字列を受け入れます。各要素の意味は次のとおりです:

- `host`には、IPアドレスまたはホスト名を使用できます。
- `port`は、有効なTCPポートまたはシンボリックサービス名（`http`など）です。すでに[Prometheusに割り当てられている](https://github.com/prometheus/prometheus/wiki/Default-port-allocations)ポート`9252`を使用する必要があります。

リッスンアドレスにポートが含まれていない場合は、デフォルトで`9252`になります。

アドレスの例:

- `:9252`は、ポート`9252`のすべてのインターフェースでリッスンします。
- `localhost:9252`は、ポート`9252`のループバックインターフェースでリッスンします。
- `[2001:db8::1]:http`は、HTTPポート`80`のIPv6アドレス`[2001:db8::1]`でリッスンします。

少なくともLinux/Unixシステムでは、`1024`より下のポートでリッスンするには、root/管理者権限が必要であることに注意してください。

HTTPサーバーは、選択されている`host:port`で**認証なしで**開きます。メトリクスサーバーをパブリックインターフェースにバインドする場合は、ファイアウォールを使用してアクセス制御を制限するか、認可とアクセス制御のためにHTTPプロキシを追加します。

## GitLab Runner Operatorによって管理されるGitLab Runnerをモニタリングします {#monitor-operator-managed-gitlab-runners}

GitLab Runner Operatorによって管理されるGitLab Runnerは、スタンドアロンのGitLab Runnerインスタンスと同じ埋め込みPrometheusメトリクスサーバーを使用します。メトリクスサーバーは、`listenAddr`が`[::]:9252`に設定されており、ポート`9252`上のすべてのIPv6およびIPv4インターフェースでリッスンするように事前設定されています。

### メトリクスポートを公開する {#expose-metrics-port}

GitLab Runner Operatorによって管理されるGitLab Runnerのモニタリングとメトリクス収集を有効にするには、[Operatorが管理するGitLab Runnerをモニタリングする](#monitor-operator-managed-gitlab-runners)を参照してください。

#### メトリクスポートを設定する {#configure-the-metrics-port}

次のパッチをRunner設定の`podSpec`フィールドに追加します:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: gitlab-runner
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  buildImage: alpine
  podSpec:
    name: "metrics-config"
    patch: |
      {
        "containers": [
          {
            "name": "runner",
            "ports": [
              {
                "name": "metrics",
                "containerPort": 9252,
                "protocol": "TCP"
              }
            ]
          }
        ]
      }
    patchType: "strategic"
```

この設定では:

- `name`: 識別用のカスタム`PodSpec`に名前を割り当てます。
- `patch`: `PodSpec`に適用するJSONパッチを定義し、Runnerコンテナ上のポート`9252`を公開します。
- `patchType`: パッチを適用するために、`strategic`マージ戦略（デフォルト）を使用します。
- `port`: Kubernetesサービスで簡単に識別できるように、`metrics`として名前が付けられています。

#### Prometheusのスクレイピングを設定する {#configure-prometheus-scraping}

Prometheus Operatorを使用する環境の場合は、Runnerポッドからメトリクスを直接スクレイプするための`PodMonitor`リソースを作成します:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: gitlab-runner-metrics
  namespace: kube-prometheus-stack
  labels:
    release: kube-prometheus-stack
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: runner
  namespaceSelector:
    matchNames:
      - gitlab-runner-system
  podMetricsEndpoints:
    - port: metrics
      interval: 10s
      path: /metrics
```

`PodMonitor`構成を適用します:

```shell
kubectl apply -f gitlab-runner-podmonitor.yaml
```

`PodMonitor`構成:

- `selector`: `app.kubernetes.io/component: runner`ラベルが付いたポッドと一致します。
- `namespaceSelector`: スクレイピングを`gitlab-runner-system`ネームスペースに制限します。
- `podMetricsEndpoints`: メトリクスポート、スクレイプ間隔、パスを定義します。

#### Runnerの識別をメトリクスに追加する {#add-runner-identification-to-metrics}

すべてのエクスポートされたメトリクスにRunnerの識別を追加するには、`PodMonitor`にrelabel設定を含めます:

```yaml
podMetricsEndpoints:
  - port: metrics
    interval: 10s
    path: /metrics
    relabelings:
      - sourceLabels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        targetLabel: runner_name
```

relabel設定:

- 各Runnerポッドから`app.kubernetes.io/name`ラベルを抽出します（GitLab Runner Operatorによって自動的に設定されます）。
- そのポッドからのすべてのメトリクスに、`runner_name`ラベルとして追加します。
- 特定のRunnerインスタンスによるフィルターと集計メトリクスを有効にします。

次に示すのは、Runnerの識別情報を含むメトリクスの例です:

```prometheus
gitlab_runner_concurrent{runner_name="my-gitlab-runner"} 10
gitlab_runner_jobs_running_total{runner_name="my-gitlab-runner"} 3
```

#### Prometheusの直接スクレイプ設定 {#direct-prometheus-scrape-configuration}

Prometheus Operatorを使用していない場合は、Prometheusスクレイプ設定でrelabel設定を直接追加できます:

```yaml
scrape_configs:
  - job_name: 'gitlab-runner-operator'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - gitlab-runner-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        target_label: runner_name
    metrics_path: /metrics
    scrape_interval: 10s
```

この設定では:

- Kubernetesサービスディスカバリを使用して、`gitlab-runner-system`ネームスペース内のポッドを検索します。
- `app.kubernetes.io/name`ラベルを抽出し、メトリクスに`runner_name`として追加します。

## Kubernetes以外のexecutorを使用するGitLab Runnerをモニタリングする {#monitor-gitlab-runner-with-executors-other-than-kubernetes}

Kubernetes以外のexecutorを使用するGitLab Runnerデプロイメントの場合、Prometheus設定で外部ラベルを介してRunnerの識別を追加できます。

### 外部ラベルを使用した静的な設定 {#static-configuration-with-external-labels}

GitLab Runnerインスタンスをスクレイプし、識別ラベルを追加するようにPrometheusを設定します:

```yaml
scrape_configs:
  - job_name: 'gitlab-runner'
    static_configs:
      - targets: ['runner1.example.com:9252']
        labels:
          runner_name: 'production-runner-1'
      - targets: ['runner2.example.com:9252']
        labels:
          runner_name: 'staging-runner-1'
    metrics_path: /metrics
    scrape_interval: 30s
```

この設定により、メトリクスにRunnerの識別が追加されます:

```prometheus
gitlab_runner_concurrent{runner_name="production-runner-1"} 10
gitlab_runner_jobs_running_total{runner_name="staging-runner-1"} 3
```

この設定により、次のことが可能になります:

- 特定のRunnerインスタンスでメトリクスをフィルターします。
- Runner固有のダッシュボードとアラートを作成します。
- さまざまなRunnerデプロイメント全体のパフォーマンスを追跡する。

### Operatorが管理するGitLab Runnerで利用可能なメトリクス {#available-metrics-for-operator-managed-gitlab-runners}

GitLab Runner Operatorによって管理されるGitLab Runnerは、スタンドアロンのGitLab Runnerデプロイメントと同じメトリクスを公開します。利用可能なすべてのメトリクスを表示するには、`kubectl`を使用してメトリクスエンドポイントにアクセスします:

```shell
kubectl port-forward pod/<gitlab-runner-pod-name> 9252:9252
curl -s "http://localhost:9252/metrics" | grep -E "# HELP"
```

利用可能なメトリクスの完全なリストについては、[利用可能なメトリクス](#available-metrics)を参照してください。

### Operatorが管理するGitLab Runnerのセキュリティに関する考慮事項 {#security-considerations-for-operator-managed-gitlab-runners}

GitLab Runner Operatorによって管理されるGitLab Runnerのメトリクス収集を設定する場合:

- Kubernetes `NetworkPolicies`を使用して、承認されたモニタリングシステムへのアクセスを制限します。
- 本番環境でのメトリクススクレイピングには、`mutal` TLS暗号化の使用を検討してください。

### Operatorが管理するGitLab Runnerモニタリングのトラブルシューティング {#troubleshooting-operator-managed-gitlab-runner-monitoring}

#### メトリクスエンドポイントにアクセスできません {#metrics-endpoint-not-accessible}

メトリクスエンドポイントにアクセスできない場合:

1. ポッドの仕様にメトリクスポート設定が含まれていることを検証する。
1. Runnerポッドが実行中で正常であることを確認します:

   ```shell
   kubectl get pods -l app.kubernetes.io/component=runner -n gitlab-runner-system
   kubectl describe pod <runner-pod-name> -n gitlab-runner-system
   ```

1. メトリクスエンドポイントへの接続をテストします:

   ```shell
   kubectl port-forward pod/<runner-pod-name> 9252:9252 -n gitlab-runner-system
   curl "http://localhost:9252/metrics"
   ```

#### Prometheusにメトリクスが表示されない {#missing-metrics-in-prometheus}

Prometheusにメトリクスが表示されない場合:

1. `PodMonitor`が正しく設定され、適用されていることを検証する。
1. ネームスペースとラベルセレクターがRunnerポッドと一致することを確認します。
1. スクレイピングエラーのPrometheusログをレビューします。
1. `PodMonitor`がPrometheus Operatorによって検出可能であることを検証します:

   ```shell
   kubectl get podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   kubectl describe podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   ```
