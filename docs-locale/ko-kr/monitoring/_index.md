---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Prometheus 메트릭입니다.
title: GitLab 러너 사용 현황 모니터링
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너는 [Prometheus](https://prometheus.io)를 사용하여 모니터링할 수 있습니다.

## Embedded Prometheus 메트릭 {#embedded-prometheus-metrics}

GitLab 러너는 기본 Prometheus 메트릭을 포함하며, `/metrics` 경로의 embedded HTTP 서버를 사용하여 노출할 수 있습니다. 서버는 활성화된 경우 Prometheus 모니터링 시스템에서 스크래핑하거나 다른 HTTP 클라이언트로 액세스할 수 있습니다.

노출된 정보에는 다음이 포함됩니다:

- 러너 비즈니스 로직 메트릭(예: 현재 실행 중인 작업의 수)
- Go 관련 프로세스 메트릭(예: 가비지 컬렉션 통계, goroutine 및 memstats)
- 일반 프로세스 메트릭(메모리 사용량, CPU 사용량, 파일 설명자 사용량 등)
- 빌드 버전 정보

메트릭 형식은 Prometheus' [Exposition formats](https://prometheus.io/docs/instrumenting/exposition_formats/) 명세에 문서화되어 있습니다.

이 메트릭은 운영자가 러너를 모니터링하고 통찰력을 얻을 수 있는 방법으로 의도되었습니다. 예를 들어, 러너 호스트의 로드 평균 증가가 처리된 작업의 증가와 관련이 있는지 알고 싶을 수 있습니다. 또는 머신의 클러스터를 실행 중이고 빌드 추세를 추적하여 인프라에 변경을 가할 수 있기를 원할 수 있습니다.

### Prometheus에 대해 자세히 알아보기 {#learning-more-about-prometheus}

Prometheus 서버를 설정하여 이 HTTP 엔드포인트를 스크래핑하고 수집된 메트릭을 사용하려면 Prometheus의 [getting started](https://prometheus.io/docs/prometheus/latest/getting_started/) 가이드를 참조하세요. Prometheus를 구성하는 방법에 대한 자세한 내용은 [configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) 섹션을 참조하세요. 경고 구성에 대한 자세한 내용은 [alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) 및 [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/)를 참조하세요.

## 사용 가능한 메트릭 {#available-metrics}

사용 가능한 모든 메트릭의 전체 목록을 찾으려면 메트릭 엔드포인트가 구성 및 활성화된 후 `curl`을 사용합니다. 예를 들어, 리스닝 포트 `9252`(으)로 구성된 로컬 러너의 경우:

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

목록에는 [Go-specific process metrics](https://github.com/prometheus/client_golang/blob/v1.19.0/prometheus/go_collector.go)가 포함됩니다. Go 관련 프로세스를 포함하지 않는 사용 가능한 메트릭 목록은 [Monitoring runners](../fleet_scaling/_index.md#monitoring-runners)를 참조하세요.

## `pprof` HTTP 엔드포인트 {#pprof-http-endpoints}

메트릭을 통한 GitLab 러너 프로세스의 내부 상태는 중요하지만, 경우에 따라 실행 중인 프로세스를 실시간으로 검토해야 합니다. 그래서 우리는 `pprof` HTTP 엔드포인트를 도입했습니다.

`pprof` 엔드포인트는 `/debug/pprof/` 경로의 embedded HTTP 서버를 통해 사용 가능합니다.

`pprof` 사용에 대해 자세히 알아보려면 [documentation](https://pkg.go.dev/net/http/pprof)을 참조하세요.

## 메트릭 HTTP 서버의 구성 {#configuration-of-the-metrics-http-server}

> [!note]
> 메트릭 서버는 GitLab 러너 프로세스의 내부 상태에 대한 데이터를 내보내며 공개적으로 사용할 수 없어야 합니다!

다음 방법 중 하나를 사용하여 메트릭 HTTP 서버를 구성합니다:

- `listen_address` 파일에서 `config.toml` 글로벌 구성 옵션을 사용합니다.
- `run` 명령에 대해 `--listen-address` 명령줄 옵션을 사용합니다.
- Helm 차트를 사용하는 러너의 경우, `values.yaml`에서:

  1. `metrics` 옵션을 구성합니다:

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

  1. `service` 모니터를 구성하여 구성된 `metrics`을 검색합니다:

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

주소를 `config.toml` 파일에 추가하면 메트릭 HTTP 서버를 시작하려면 러너 프로세스를 다시 시작해야 합니다.

두 경우 모두 옵션은 `[host]:<port>` 형식의 문자열을 허용합니다. 여기서:

- `host`은 IP 주소 또는 호스트명일 수 있습니다.
- `port`은 유효한 TCP 포트 또는 기호 서비스 이름입니다(예: `http`). 포트 `9252`을 사용해야 하며, 이는 이미 [allocated in Prometheus](https://github.com/prometheus/prometheus/wiki/Default-port-allocations)입니다.

listen 주소에 포트가 없으면 `9252`로 기본값이 설정됩니다.

주소의 예:

- `:9252`은 포트 `9252`의 모든 인터페이스에서 수신합니다.
- `localhost:9252`은 포트 `9252`의 루프백 인터페이스에서 수신합니다.
- `[2001:db8::1]:http`은 IPv6 주소 `[2001:db8::1]`에서 HTTP 포트 `80`에서 수신합니다.

`1024` 아래의 포트에서 수신하려면 - 적어도 Linux/Unix 시스템에서 - root/관리자 권한이 필요합니다.

HTTP 서버는 선택된 `host:port`(으)로 열리며 **without any authorization**입니다. 메트릭 서버를 공개 인터페이스에 바인드하면 방화벽을 사용하여 액세스를 제한하거나 인증 및 액세스 제어를 위해 HTTP 프록시를 추가합니다.

## Operator가 관리하는 GitLab 러너 모니터링 {#monitor-operator-managed-gitlab-runners}

GitLab 러너 Operator가 관리하는 GitLab 러너는 독립 실행형 GitLab 러너 인스턴스와 동일한 embedded Prometheus 메트릭 서버를 사용합니다. 메트릭 서버는 `listenAddr`이 `[::]:9252`로 설정되어 사전 구성되어 있으며, 포트 `9252`의 모든 IPv6 및 IPv4 인터페이스에서 수신합니다.

### 메트릭 포트 노출 {#expose-metrics-port}

GitLab 러너 Operator가 관리하는 GitLab 러너에 대한 모니터링 및 메트릭 수집을 활성화하려면 [Monitor Operator managed GitLab Runners](#monitor-operator-managed-gitlab-runners)를 참조하세요.

#### 메트릭 포트 구성 {#configure-the-metrics-port}

러너 구성의 `podSpec` 필드에 다음 패치를 추가합니다:

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

이 구성:

- `name`: 식별을 위해 사용자 정의 `PodSpec`에 이름을 할당합니다.
- `patch`: `PodSpec`에 적용할 JSON 패치를 정의하고, 러너 컨테이너에 포트 `9252`을 노출합니다.
- `patchType`: `strategic` 병합 전략(기본값)을 사용하여 패치를 적용합니다.
- `port`: Kubernetes 서비스에서 쉽게 식별할 수 있도록 `metrics`로 명명됩니다.

#### Prometheus 스크래핑 구성 {#configure-prometheus-scraping}

Prometheus Operator를 사용하는 환경의 경우, 러너 Pod에서 메트릭을 직접 스크래핑하기 위해 `PodMonitor` 리소스를 생성합니다:

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

`PodMonitor` 구성을 적용합니다:

```shell
kubectl apply -f gitlab-runner-podmonitor.yaml
```

`PodMonitor` 구성:

- `selector`: `app.kubernetes.io/component: runner` 레이블이 있는 Pod를 일치시킵니다.
- `namespaceSelector`: 스크래핑을 `gitlab-runner-system` 네임스페이스로 제한합니다.
- `podMetricsEndpoints`: 메트릭 포트, 스크래핑 간격 및 경로를 정의합니다.

#### 메트릭에 러너 식별 추가 {#add-runner-identification-to-metrics}

내보낸 모든 메트릭에 러너 식별을 추가하려면 `PodMonitor`에 relabel 구성을 포함합니다:

```yaml
podMetricsEndpoints:
  - port: metrics
    interval: 10s
    path: /metrics
    relabelings:
      - sourceLabels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        targetLabel: runner_name
```

relabel 구성:

- 각 러너 Pod에서 `app.kubernetes.io/name` 레이블을 추출합니다(GitLab 러너 Operator에서 자동으로 설정).
- 해당 Pod의 모든 메트릭에 `runner_name` 레이블로 추가합니다.
- 특정 러너 인스턴스별로 메트릭을 필터링 및 집계할 수 있습니다.

다음은 러너 식별이 있는 메트릭의 예입니다:

```prometheus
gitlab_runner_concurrent{runner_name="my-gitlab-runner"} 10
gitlab_runner_jobs_running_total{runner_name="my-gitlab-runner"} 3
```

#### 직접 Prometheus 스크래핑 구성 {#direct-prometheus-scrape-configuration}

Prometheus Operator를 사용하지 않는 경우 Prometheus 스크래프 구성에서 relabel 구성을 직접 추가할 수 있습니다:

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

이 구성:

- Kubernetes 서비스 검색을 사용하여 `gitlab-runner-system` 네임스페이스에서 Pod를 찾습니다.
- `app.kubernetes.io/name` 레이블을 추출하고 메트릭에 `runner_name`로 추가합니다.

## Kubernetes 이외의 실행기를 사용하는 GitLab 러너 모니터링 {#monitor-gitlab-runner-with-executors-other-than-kubernetes}

Kubernetes 이외의 실행기가 있는 GitLab 러너 배포의 경우 Prometheus 구성의 외부 레이블을 통해 러너 식별을 추가할 수 있습니다.

### 외부 레이블이 있는 정적 구성 {#static-configuration-with-external-labels}

GitLab 러너 인스턴스를 스크래핑하고 식별 레이블을 추가하도록 Prometheus를 구성합니다:

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

이 구성은 메트릭에 러너 식별을 추가합니다:

```prometheus
gitlab_runner_concurrent{runner_name="production-runner-1"} 10
gitlab_runner_jobs_running_total{runner_name="staging-runner-1"} 3
```

이 구성을 사용하면 다음을 수행할 수 있습니다:

- 특정 러너 인스턴스별로 메트릭을 필터링합니다.
- 러너별 대시보드 및 경고를 생성합니다.
- 다양한 러너 배포 간에 성능을 추적합니다.

### Operator가 관리하는 GitLab 러너에 사용 가능한 메트릭 {#available-metrics-for-operator-managed-gitlab-runners}

GitLab 러너 Operator가 관리하는 GitLab 러너는 독립 실행형 GitLab 러너 배포와 동일한 메트릭을 노출합니다. 사용 가능한 모든 메트릭을 보려면 `kubectl`을 사용하여 메트릭 엔드포인트에 액세스합니다:

```shell
kubectl port-forward pod/<gitlab-runner-pod-name> 9252:9252
curl -s "http://localhost:9252/metrics" | grep -E "# HELP"
```

사용 가능한 메트릭의 전체 목록은 [Available metrics](#available-metrics)를 참조하세요.

### Operator가 관리하는 GitLab 러너의 보안 고려 사항 {#security-considerations-for-operator-managed-gitlab-runners}

GitLab 러너 Operator가 관리하는 GitLab 러너에 대한 메트릭 수집을 구성할 때:

- Kubernetes `NetworkPolicies`을 사용하여 권한이 있는 모니터링 시스템에 대한 액세스를 제한합니다.
- 프로덕션 환경에서 메트릭 스크래핑을 위해 `mutal` TLS 암호화를 사용하는 것을 고려합니다.

### Operator가 관리하는 GitLab 러너 모니터링 문제 해결 {#troubleshooting-operator-managed-gitlab-runner-monitoring}

#### 메트릭 엔드포인트에 액세스할 수 없음 {#metrics-endpoint-not-accessible}

메트릭 엔드포인트에 액세스할 수 없는 경우:

1. Pod 명세에 메트릭 포트 구성이 포함되어 있는지 확인합니다.
1. 러너 Pod가 실행 중이고 정상 상태인지 확인합니다:

   ```shell
   kubectl get pods -l app.kubernetes.io/component=runner -n gitlab-runner-system
   kubectl describe pod <runner-pod-name> -n gitlab-runner-system
   ```

1. 메트릭 엔드포인트에 대한 연결을 테스트합니다:

   ```shell
   kubectl port-forward pod/<runner-pod-name> 9252:9252 -n gitlab-runner-system
   curl "http://localhost:9252/metrics"
   ```

#### Prometheus에서 메트릭이 누락됨 {#missing-metrics-in-prometheus}

메트릭이 Prometheus에 나타나지 않는 경우:

1. `PodMonitor`이 올바르게 구성되고 적용되었는지 확인합니다.
1. 네임스페이스 및 레이블 선택기가 러너 Pod와 일치하는지 확인합니다.
1. 스크래핑 오류에 대한 Prometheus 로그를 검토합니다.
1. `PodMonitor`이 Prometheus Operator에서 검색 가능한지 확인합니다:

   ```shell
   kubectl get podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   kubectl describe podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   ```
