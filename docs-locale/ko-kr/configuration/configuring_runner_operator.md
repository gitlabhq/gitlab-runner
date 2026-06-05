---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: OpenShift에서 GitLab 러너 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

이 문서는 OpenShift에서 GitLab 러너를 구성하는 방법을 설명합니다.

## GitLab 러너 연산자에 속성 전달 {#passing-properties-to-gitlab-runner-operator}

`Runner`를 만들 때 해당 `spec`에서 속성을 설정하여 구성할 수 있습니다. 예를 들어 러너가 등록된 GitLab URL이나 등록 토큰을 포함하는 시크릿의 이름을 지정할 수 있습니다:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret # Name of the secret containing the Runner token
```

사용 가능한 모든 속성에 대해 [연산자 속성](#operator-properties)을 읽으세요.

## 연산자 속성 {#operator-properties}

다음 속성을 연산자에 전달할 수 있습니다.

일부 속성은 최신 버전의 연산자에서만 사용할 수 있습니다.

| 설정            | 연산자 | 설명 |
|--------------------|----------|-------------|
| `gitlabUrl`        | 모두      | 예를 들어 `https://gitlab.example.com`같은 GitLab 인스턴스의 정규화된 도메인 이름입니다. |
| `token`            | 모두      | `Secret`의 이름으로, 러너를 등록하는 데 사용되는 `runner-registration-token` 키를 포함합니다. |
| `tags`             | 모두      | 러너에 적용될 쉼표로 구분된 태그 목록입니다. |
| `concurrent`       | 모두      | 동시에 실행할 수 있는 작업 수를 제한합니다. 최대 수는 정의된 모든 러너입니다. 0은 무제한을 의미하지 않습니다. 기본값은 `10`입니다. |
| `interval`         | 모두      | 새 작업을 확인하는 간격(초)을 정의합니다. 기본값은 `30`입니다. |
| `locked`           | 1.8      | 러너를 프로젝트에 잠글지 여부를 정의합니다. 기본값은 `false`입니다. |
| `runUntagged`      | 1.8      | 태그 없는 작업을 실행할지 여부를 정의합니다. 태그가 지정되지 않은 경우 기본값은 `true`입니다. 그렇지 않으면 `false`입니다. |
| `protected`        | 1.8      | 러너가 보호된 브랜치에서만 작업을 실행할지 여부를 정의합니다. 기본값은 `false`입니다. |
| `cloneURL`         | 모두      | GitLab 인스턴스의 URL을 덮어씁니다. 러너가 GitLab URL에 연결할 수 없는 경우에만 사용됩니다. |
| `env`              | 모두      | `ConfigMap`의 이름으로, 러너 포드에 환경 변수로 주입되는 키-값 쌍을 포함합니다. |
| `runnerImage`      | 1.7      | 기본 GitLab 러너 이미지를 덮어씁니다. 기본값은 연산자가 번들로 제공된 러너 이미지입니다. |
| `helperImage`      | 모두      | 기본 GitLab 러너 헬퍼 이미지를 덮어씁니다. |
| `buildImage`       | 모두      | 지정된 것이 없을 때 빌드에 사용할 기본 Docker 이미지입니다. |
| `cacheType`        | 모두      | 러너 아티팩트에 사용되는 캐시 유형입니다. `gcs`, `s3`, `azure` 중 하나입니다. |
| `cachePath`        | 모두      | 파일 시스템의 캐시 경로를 정의합니다. |
| `cacheShared`      | 모두      | 러너 간 캐시 공유를 활성화합니다. |
| `s3`               | 모두      | S3 캐시를 설정하는 데 사용되는 옵션입니다. [캐시 속성](#cache-properties)을 참조하세요. |
| `gcs`              | 모두      | `gcs` 캐시를 설정하는 데 사용되는 옵션입니다. [캐시 속성](#cache-properties)을 참조하세요. |
| `azure`            | 모두      | Azure 캐시를 설정하는 데 사용되는 옵션입니다. [캐시 속성](#cache-properties)을 참조하세요. |
| `ca`               | 모두      | 사용자 지정 CA(인증서 기관) 인증서를 포함하는 TLS 시크릿의 이름입니다. |
| `serviceaccount`   | 모두      | 러너 포드를 실행하는 데 사용할 서비스 계정을 재정의하는 데 사용합니다. |
| `config`           | 모두      | 사용자 지정 `ConfigMap`과 [구성 템플릿](../register/_index.md#register-with-a-configuration-template)을 제공하는 데 사용합니다. |
| `shutdownTimeout`  | 1.34     | [강제 종료 작업](../commands/_index.md#signals)이 시간 초과되고 프로세스가 종료될 때까지의 시간(초)입니다. 기본값은 `30`입니다. `0` 이하로 설정하면 기본값이 사용됩니다. |
| `logLevel`         | 1.34     | 로그 수준을 정의합니다. 옵션은 `debug`, `info`, `warn`, `error`, `fatal`, `panic`입니다. |
| `logFormat`        | 1.34     | 로그 형식을 지정합니다. 옵션은 `runner`, `text`, `json`입니다. 기본값은 `runner`이며, 색상 지정을 위한 ANSI 이스케이프 코드를 포함합니다. |
| `listenAddr`       | 1.34     | Prometheus 메트릭 HTTP 서버가 수신해야 할 주소(`<host>:<port>`)를 정의합니다. 구성에 대한 정보는 [GitLab 러너 연산자 모니터링](../monitoring/_index.md#monitor-operator-managed-gitlab-runners)을 참조하세요. |
| `sentryDsn`        | 1.34     | Sentry로의 모든 시스템 수준 오류 추적을 활성화합니다. |
| `connectionMaxAge` | 1.34     | GitLab 서버에 대한 TLS keepalive 연결이 재연결되기 전에 열려 있어야 하는 최대 기간입니다. 기본값은 15분의 `15m`입니다. `0` 이하로 설정하면 연결이 가능한 한 오래 유지됩니다. |
| `podSpec`          | 1.23     | GitLab 러너 포드(템플릿)에 적용할 패치 목록입니다. 자세한 내용은 [러너 포드 템플릿 패칭](#patching-the-runner-pod-template)을 참조하세요. |
| `deploymentSpec`   | 1.40     | GitLab 러너 배포에 적용할 패치 목록입니다. 자세한 내용은 [러너 배포 템플릿 패칭](#patching-the-runner-deployment-template)을 참조하세요. |

## 캐시 속성 {#cache-properties}

### S3 캐시 {#s3-cache}

| 설정       | 연산자 | 설명 |
|---------------|----------|-------------|
| `server`      | 모두      | S3 서버 주소입니다. |
| `credentials` | 모두      | `Secret`의 이름으로, 객체 스토리지에 액세스하는 데 사용되는 `accesskey` 및 `secretkey` 속성을 포함합니다. |
| `bucket`      | 모두      | 캐시가 저장되는 버킷의 이름입니다. |
| `location`    | 모두      | 캐시가 저장되는 S3 지역의 이름입니다. |
| `insecure`    | 모두      | 안전하지 않은 연결 또는 `HTTP`를 사용합니다. |

### `gcs` 캐시 {#gcs-cache}

| 설정           | 연산자 | 설명 |
|-------------------|----------|-------------|
| `credentials`     | 모두      | `Secret`의 이름으로, 객체 스토리지에 액세스하는 데 사용되는 `access-id` 및 `private-key` 속성을 포함합니다. |
| `bucket`          | 모두      | 캐시가 저장되는 버킷의 이름입니다. |
| `credentialsFile` | 모두      | `gcs` 자격증 파일인 `keys.json`를 사용합니다. |

### Azure 캐시 {#azure-cache}

| 설정         | 연산자 | 설명 |
|-----------------|----------|-------------|
| `credentials`   | 모두      | `Secret`의 이름으로, 객체 스토리지에 액세스하는 데 사용되는 `accountName` 및 `privateKey` 속성을 포함합니다. |
| `container`     | 모두      | 캐시가 저장되는 Azure 컨테이너의 이름입니다. |
| `storageDomain` | 모두      | Azure Blob Storage의 도메인 이름입니다. |

## 프록시 환경 구성 {#configure-a-proxy-environment}

프록시 환경을 만들려면:

1. `custom-env.yaml` 파일을 편집합니다. 예를 들어:

   ```yaml
   apiVersion: v1
   data:
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
   ```

1. OpenShift를 업데이트하여 변경 사항을 적용합니다.

   ```shell
   oc apply -f custom-env.yaml
   ```

1. [`gitlab-runner.yml`](../install/operator.md#install-gitlab-runner) 파일을 업데이트합니다.

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret # Name of the secret containing the Runner token
     env: custom-env
   ```

프록시가 Kubernetes API에 연결할 수 없으면 CI/CD 작업에 오류가 표시될 수 있습니다:

```shell
ERROR: Job failed (system failure): prepare environment: setting up credentials: Post https://172.21.0.1:443/api/v1/namespaces/<KUBERNETES_NAMESPACE>/secrets: net/http: TLS handshake timeout. Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

이 오류를 해결하려면 `custom-env.yaml` 파일의 `NO_PROXY` 구성에 Kubernetes API의 IP 주소를 추가하세요:

```yaml
   apiVersion: v1
   data:
     NO_PROXY: 172.21.0.1
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
```

다음 명령을 실행하여 Kubernetes API의 IP 주소를 확인할 수 있습니다:

```shell
oc get services --namespace default --field-selector='metadata.name=kubernetes' | grep -v NAME | awk '{print $3}'
```

## `config.toml`을 구성 템플릿으로 사용자 지정 {#customize-configtoml-with-a-configuration-template}

[구성 템플릿](../register/_index.md#register-with-a-configuration-template)을 사용하여 러너의 `config.toml` 파일을 사용자 지정할 수 있습니다.

1. 사용자 지정 구성 템플릿 파일을 만듭니다. 예를 들어 러너에 `EmptyDir` 볼륨을 마운트하고 `cpu_limit`를 설정하도록 지시해 봅시다. `custom-config.toml` 파일을 만듭니다:

   ```toml
   [[runners]]
     [runners.kubernetes]
       cpu_limit = "500m"
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "empty-dir"
           mount_path = "/path/to/empty_dir"
           medium = "Memory"
   ```

1. `custom-config.toml` 파일에서 `custom-config-toml`라는 `ConfigMap`을 만듭니다:

   ```shell
    oc create configmap custom-config-toml --from-file config.toml=custom-config.toml
   ```

1. `Runner`의 `config` 속성을 설정합니다:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     config: custom-config-toml
   ```

[알려진 문제](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/issues/229)로 인해 구성 템플릿 대신 환경 변수를 사용하여 다음 설정을 수정해야 합니다:

| 설정                          | 환경 변수         | 기본값 |
|----------------------------------|------------------------------|---------------|
| `runners.request_concurrency`    | `RUNNER_REQUEST_CONCURRENCY` | `1`           |
| `runners.output_limit`           | `RUNNER_OUTPUT_LIMIT`        | `4096`        |
| `kubernetes.runner.poll_timeout` | `KUBERNETES_POLL_TIMEOUT`    | `180`         |

## 사용자 지정 TLS 인증서 구성 {#configure-a-custom-tls-cert}

1. 사용자 지정 TLS 인증서를 설정하려면 키 `tls.crt`를 사용하여 시크릿을 만듭니다. 이 예제에서 파일의 이름은 `custom-tls-ca-secret.yaml`입니다:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
       name: custom-tls-ca
   type: Opaque
   stringData:
       tls.crt: |
           -----BEGIN CERTIFICATE-----
           MIIEczCCA1ugAwIBAgIBADANBgkqhkiG9w0BAQQFAD..AkGA1UEBhMCR0Ix
           .....
           7vQMfXdGsRrXNGRGnX+vWDZ3/zWI0joDtCkNnqEpVn..HoX
           -----END CERTIFICATE-----
   ```

1. 시크릿을 만듭니다:

   ```shell
   oc apply -f custom-tls-ca-secret.yaml
   ```

1. `runner.yaml`에서 `ca` 키를 우리 시크릿의 이름과 동일하게 설정합니다:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     ca: custom-tls-ca
   ```

## 러너 포드의 CPU 및 메모리 크기 구성 {#configure-the-cpu-and-memory-size-of-runner-pods}

사용자 지정 `config.toml` 파일에서 [CPU 제한](../executors/kubernetes/_index.md#cpu-requests-and-limits) 및 [메모리 제한](../executors/kubernetes/_index.md#memory-requests-and-limits) 을 설정하려면 [이 항목](#customize-configtoml-with-a-configuration-template)의 지침을 따릅니다.

## 클러스터 리소스에 따른 러너당 작업 동시성 구성 {#configure-job-concurrency-per-runner-based-on-cluster-resources}

`Runner` 리소스의 `concurrent` 속성을 설정합니다:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  concurrent: 2
```

작업 동시성은 프로젝트의 요구 사항에 따라 결정됩니다.

1. CI 작업을 실행하는 데 필요한 컴퓨팅 및 메모리 리소스를 결정하는 것부터 시작하세요.
1. 클러스터의 리소스가 주어졌을 때 해당 작업이 실행될 수 있는 횟수를 계산합니다.

높은 동시성 값을 설정하면 Kubernetes 실행기가 가능한 한 빨리 작업을 처리합니다. 그러나 Kubernetes 클러스터의 스케줄러 용량이 작업이 스케줄되는 시기를 결정합니다.

## GitLab 러너 관리자를 위한 서비스 계정 {#service-account-for-the-gitlab-runner-manager}

새로운 설치의 경우 GitLab 러너는 이러한 RBAC 역할 바인딩 리소스가 없으면 러너 관리자 포드에 대해 `gitlab-runner-app-sa`라는 Kubernetes `ServiceAccount`를 만듭니다:

- `gitlab-runner-app-rolebinding`
- `gitlab-runner-rolebinding`

역할 바인딩 중 하나가 있으면 GitLab은 역할 바인딩에 정의된 `subjects` 및 `roleRef`에서 역할 및 서비스 계정을 확인합니다.

두 역할 바인딩이 모두 있으면 `gitlab-runner-app-rolebinding`이 `gitlab-runner-rolebinding`보다 우선합니다.

## 문제 해결 {#troubleshooting}

### 루트 vs 루트 아님 {#root-vs-non-root}

GitLab 러너 연산자 및 GitLab 러너 포드는 루트가 아닌 사용자로 실행됩니다. 결과적으로 작업에서 사용되는 빌드 이미지는 성공적으로 완료될 수 있도록 루트가 아닌 사용자로 실행되어야 합니다. 이는 작업이 최소 권한으로 성공적으로 실행될 수 있도록 보장합니다.

이를 작동시키려면 CI/CD 작업에 사용되는 빌드 이미지가 다음을 수행하는지 확인하세요:

- 루트가 아닌 사용자로 실행
- 제한된 파일 시스템에 쓰지 않음

OpenShift 클러스터의 대부분 컨테이너 파일 시스템은 다음을 제외하고 읽기 전용입니다:

- 마운트된 볼륨
- `/var/tmp`
- `/tmp`
- 루트 파일 시스템에 `tmpfs`로 마운트된 기타 볼륨

#### `HOME` 환경 변수 재정의 {#overriding-the-home-environment-variable}

사용자 지정 빌드 이미지를 만들거나 [환경 변수를 재정의](#configure-a-proxy-environment)하는 경우 `HOME` 환경 변수가 읽기 전용인 `/`로 설정되지 않았는지 확인하세요. 특히 작업이 홈 디렉터리에 파일을 써야 하는 경우입니다. `/home` 아래에 디렉터리를 만들 수 있습니다. 예를 들어 `/home/ci` 및 `Dockerfile`에서 `ENV HOME=/home/ci`을 설정합니다.

러너 포드의 경우 [`HOME`가 `/home/gitlab-runner`로 설정될 것으로 예상됩니다](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L14). 이 변수가 변경되면 새 위치는 [적절한 권한](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L38)을 가져야 합니다. 이러한 지침은 [Red Hat 컨테이너 플랫폼 문서](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/images/creating-images#images-create-guide-openshift_create-images)에도 문서화되어 있습니다.

### `locked` 변수 재정의 {#overriding-locked-variable}

러너 토큰을 등록할 때 `locked` 변수를 `true`로 설정하면 오류 `Runner configuration other than name, description, and exector is reserved and cannot be specified`이 나타납니다.

```yaml
  locked: true # REQUIRED
  tags: ""
  runUntagged: false
  protected: false
  maximumTimeout: 0
```

자세한 내용은 [이슈 472](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/472#note_1483346437)를 참조하세요.

#### 보안 컨텍스트 제약 주의 {#watch-out-for-security-context-constraints}

기본적으로 새 OpenShift 프로젝트에 설치할 때 GitLab 러너 연산자는 루트가 아닌 사용자로 실행됩니다. `default` 프로젝트와 같은 일부 프로젝트는 모든 서비스 계정에 `anyuid` 액세스 권한이 있는 예외입니다. 이 경우 이미지의 사용자는 `root`입니다. `whoami`를 컨테이너 셸 내에서 실행하여 확인할 수 있습니다(예: 작업). 보안 컨텍스트 제약에 대해 자세히 알아보려면 [Red Hat 컨테이너 플랫폼 문서](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/authentication_and_authorization/managing-pod-security-policies)를 읽으세요.

#### `anyuid` 보안 컨텍스트 제약으로 실행 {#run-as-anyuid-security-context-constraints}

> [!warning]
> 루트로 작업을 실행하거나 루트 파일 시스템에 쓰면 시스템이 보안 위험에 노출될 수 있습니다.

CI/CD 작업을 루트 사용자로 실행하거나 루트 파일 시스템에 쓰려면 `gitlab-runner-app-sa` 서비스 계정에서 `anyuid` 보안 컨텍스트 제약을 설정합니다. GitLab 러너 컨테이너는 이 서비스 계정을 사용합니다.

OpenShift 4.3.8 이전:

```shell
oc adm policy add-scc-to-user anyuid -z gitlab-runner-app-sa -n <runner_namespace>

# Check that the anyiud SCC is set:
oc get scc anyuid -o yaml
```

OpenShift 4.3.8 이후:

```shell
oc create -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scc-anyuid
  namespace: <runner_namespace>
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - anyuid
  resources:
  - securitycontextconstraints
  verbs:
  - use
EOF

oc create -f - <<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: sa-to-scc-anyuid
  namespace: <runner_namespace>
subjects:
  - kind: ServiceAccount
    name: gitlab-runner-app-sa
roleRef:
  kind: Role
  name: scc-anyuid
  apiGroup: rbac.authorization.k8s.io
EOF
```

#### 헬퍼 컨테이너와 빌드 컨테이너 사용자 ID 및 그룹 ID 일치 {#matching-helper-container-and-build-container-user-id-and-group-id}

GitLab 러너 연산자 배포는 `registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp`를 기본 헬퍼 이미지로 사용합니다. 이 이미지는 보안 컨텍스트에 의해 명시적으로 수정되지 않은 한 `1001:1001`의 사용자 ID 및 그룹 ID로 실행됩니다.

빌드 컨테이너의 사용자 ID가 헬퍼 이미지의 사용자 ID와 다르면 빌드 중에 권한 관련 오류가 발생할 수 있습니다. 다음은 일반적인 오류 메시지입니다:

```shell
fatal: detected dubious ownership in repository at '/builds/gitlab-org/gitlab-runner'
```

이 오류는 브랜치 리포지토리가 사용자 ID `1001`(헬퍼 컨테이너)로 복제되었지만 빌드 컨테이너의 다른 사용자 ID가 이에 액세스하려고 함을 나타냅니다.

솔루션: 빌드 컨테이너의 보안 컨텍스트를 헬퍼 컨테이너의 사용자 ID 및 그룹 ID와 일치하도록 구성합니다:

```toml
[runners.kubernetes.build_container_security_context]
run_as_user = 1001
run_as_group = 1001
```

추가 정보:

- 이러한 설정은 브랜치 리포지토리를 복제하는 컨테이너와 빌드하는 컨테이너 간의 일관된 파일 소유권을 보장합니다.
- 헬퍼 이미지를 다른 사용자 ID 또는 그룹 ID로 사용자 지정한 경우 이러한 값을 적절히 조정합니다.
- OpenShift 배포의 경우 이러한 보안 컨텍스트 설정이 클러스터의 보안 컨텍스트 제약(SCC)을 준수하는지 확인하세요.

#### SETFCAP 구성 {#configure-setfcap}

Red Hat OpenShift 컨테이너 플랫폼(RHOCP) 4.11 이상을 사용하는 경우 다음 오류 메시지가 나타날 수 있습니다:

```shell
error reading allowed ID mappings:error reading subuid mappings for user
```

일부 작업(예: `buildah`)은 올바르게 실행하기 위해 `SETFCAP` 기능을 부여받아야 합니다. 이 문제를 해결하려면:

1. GitLab 러너가 사용 중인 보안 컨텍스트 제약에 SETFCAP 기능을 추가합니다(`gitlab-scc`을 GitLab 러너 포드에 할당된 보안 컨텍스트 제약으로 바꿉니다):

   ```shell
   oc patch scc gitlab-scc --type merge -p '{"allowedCapabilities":["SETFCAP"]}'
   ```

1. `config.toml`을 업데이트하고 `kubernetes` 섹션 아래에 `SETFCAP` 기능을 추가합니다:

   ```yaml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.pod_security_context]
       [runners.kubernetes.build_container_security_context]
       [runners.kubernetes.build_container_security_context.capabilities]
         add = ["SETFCAP"]
   ```

1. GitLab 러너가 배포된 네임스페이스에서 이 `config.toml`를 사용하여 `ConfigMap`을 만듭니다:

   ```shell
   oc create configmap custom-config-toml --from-file config.toml=config.toml
   ```

1. 수정하려는 러너를 수정하고 최근에 생성된 `ConfigMap`를 가리키는 `config:` 매개변수를 추가합니다(my-runner를 올바른 러너 포드 이름으로 바꿉니다).

   ```shell
   oc patch runner my-runner --type merge -p '{"spec": {"config": "custom-config-toml"}}'
   ```

자세한 내용은 [Red Hat 문서](https://access.redhat.com/solutions/7016013)를 참조하세요.

### FIPS 호환 GitLab 러너 사용 {#using-fips-compliant-gitlab-runner}

> [!note]
> 연산자의 경우 헬퍼 이미지만 변경할 수 있습니다. GitLab 러너 이미지는 아직 변경할 수 없습니다. [이슈 28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814)가 이 기능을 추적합니다.

[FIPS 호환 GitLab 러너 헬퍼](../install/requirements.md#fips-compliant-gitlab-runner)를 사용하려면 다음과 같이 헬퍼 이미지를 변경합니다:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  helperImage: gitlab/gitlab-runner-helper:ubi-fips
  concurrent: 2
```

#### 자체 서명 인증서를 사용하여 GitLab 러너 등록 {#register-gitlab-runner-by-using-a-self-signed-certificate}

GitLab Self-Managed에서 자체 서명 인증서를 사용하려면 프라이빗 인증서에 서명하는 데 사용한 CA 인증서를 포함하는 시크릿을 만듭니다.

시크릿의 이름은 러너 사양 섹션에서 CA로 제공됩니다:

```yaml
KIND:     Runner
VERSION:  apps.gitlab.com/v1beta2

FIELD:    ca <string>

DESCRIPTION:
     Name of tls secret containing the custom certificate authority (CA)
     certificates
```

시크릿은 다음 명령을 사용하여 만들 수 있습니다:

```shell
oc create secret generic mySecret --from-file=tls.crt=myCert.pem -o yaml
```

#### IP 주소를 가리키는 외부 URL로 GitLab 러너 등록 {#register-gitlab-runner-with-an-external-url-that-points-to-an-ip-address}

러너가 자체 서명 인증서를 호스트 이름과 일치시킬 수 없으면 오류 메시지가 나타날 수 있습니다. 이 문제는 GitLab Self-Managed를 호스트 이름 대신 IP 주소(예: `###.##.##.##`)를 사용하도록 구성할 때 발생합니다:

```shell
[31;1mERROR: Registering runner... failed               [0;m  [31;1mrunner[0;m=A5abcdEF [31;1mstatus[0;m=couldn't execute POST against https://###.##.##.##/api/v4/runners:
Post https://###.##.##.##/api/v4/runners: x509: cannot validate certificate for ###.##.##.## because it doesn't contain any IP SANs
[31;1mPANIC: Failed to register the runner. You may be having network problems.[0;m
```

이 문제를 해결하려면:

1. GitLab Self-Managed 서버에서 `openssl`을 수정하여 IP 주소를 `subjectAltName` 매개변수에 추가합니다:

   ```shell
   # vim /etc/pki/tls/openssl.cnf

   [ v3_ca ]
   subjectAltName=IP:169.57.64.36 <---- Add this line. 169.57.64.36 is your GitLab server IP.
   ```

1. 그런 다음 아래 명령으로 자체 서명 CA를 다시 생성합니다:

   ```shell
   # cd /etc/gitlab/ssl
   # openssl req -x509 -nodes -days 3650 -newkey rsa:4096 -keyout /etc/gitlab/ssl/169.57.64.36.key -out /etc/gitlab/ssl/169.57.64.36.crt
   # openssl dhparam -out /etc/gitlab/ssl/dhparam.pem 4096
   # gitlab-ctl restart
   ```

1. 이 새 인증서를 사용하여 새 시크릿을 생성합니다.

## 패치 구조 {#patch-structure}

각 사양 패치는 다음 속성으로 구성됩니다:

| 설정     | 설명 |
|-------------|-------------|
| `name`      | 사용자 지정 사양 패치의 이름입니다. |
| `patchFile` | 생성되기 전에 최종 사양에 적용할 변경 사항을 정의하는 파일의 경로입니다. 파일은 JSON 또는 YAML 파일이어야 합니다. |
| `patch`     | 생성되기 전에 최종 사양에 적용할 변경 사항을 설명하는 JSON 또는 YAML 형식 문자열입니다. |
| `patchType` | 지정된 변경 사항을 사양에 적용하는 데 사용되는 전략입니다. 허용되는 값은 `merge`, `json`, `strategic`(기본값)입니다. |

같은 사양 구성에서 `patchFile` 및 `patch`를 모두 설정할 수 없습니다.

## 러너 포드 템플릿 패칭 {#patching-the-runner-pod-template}

[포드 사양](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) 패칭을 사용하면 연산자가 생성한 Kubernetes 배포에 패치를 적용하여 GitLab 러너가 배포되는 방식을 사용자 지정할 수 있습니다. 패치는 포드 템플릿의 사양(`deployment.spec.template.spec`)에 적용됩니다.

다음과 같은 포드 수준 설정을 제어할 수 있습니다:

- 리소스 요청 및 제한
- 보안 컨텍스트
- 볼륨 마운트 및 볼륨
- 환경 변수
- 노드 선택기 및 선호도 규칙
- 허용 범위
- 호스트 이름 및 DNS 구성

## 러너 배포 템플릿 패칭 {#patching-the-runner-deployment-template}

[배포 사양](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#Deployment) 패칭을 사용하면 연산자가 생성한 Kubernetes 배포에 패치를 적용하여 GitLab 러너가 배포되는 방식을 사용자 지정할 수 있습니다. 패치는 배포 사양(`deployment.spec`)에 적용됩니다.

다음과 같은 배포 수준 설정을 제어할 수 있습니다:

- 복제본 수
- 배포 전략(RollingUpdate, Recreate)
- 개정 기록 제한
- 진행 기한(초)
- 레이블 및 주석

## 패치 순서 {#patch-order}

배포 사양 패치는 포드 사양 패치 이전에 적용됩니다. 이는 배포 및 포드 사양이 동일한 필드를 수정하는 경우 포드 사양이 우선한다는 의미입니다.

## 예제 {#examples}

### 포드 사양 패칭 예제 {#pod-specification-patching-example}

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  podSpec:
    - name: "set-hostname"
      patch: |
        hostname: "custom-hostname"
      patchType: "merge"
    - name: "add-resource-requests"
      patch: |
        containers:
        - name: build
          resources:
            requests:
              cpu: "500m"
              memory: "256Mi"
      patchType: "strategic"
```

### 배포 사양 패칭 예제 {#deployment-specification-patching-example}

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  deploymentSpec:
    - name: "set-replicas"
      patch: |
        replicas: 3
      patchType: "strategic"
    - name: "configure-strategy"
      patch: |
        strategy:
          type: RollingUpdate
          rollingUpdate:
            maxUnavailable: 25%
            maxSurge: 50%
      patchType: "strategic"
    - name: "set-revision-history"
      patch: |
        [{"op": "add", "path": "/revisionHistoryLimit", "value": 10}]
      patchType: "json"
```

## 모범 사례 {#best-practices}

- 프로덕션 배포에 적용하기 전에 프로덕션이 아닌 환경에서 패치를 테스트합니다.
- 개별 포드 설정보다는 배포 동작에 영향을 미치는 설정에 배포 수준 패치를 사용합니다.
- 포드 사양 패치가 충돌하는 필드의 배포 사양 패치를 재정의함을 기억합니다.
