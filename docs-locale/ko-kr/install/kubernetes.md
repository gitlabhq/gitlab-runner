---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab Helm 차트를 사용하여 Kubernetes에 러너를 설치합니다.
title: GitLab Helm 차트
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

러너 Helm 차트는 Kubernetes 클러스터에 러너 인스턴스를 배포하는 공식적인 방법입니다. 이 차트는 러너를 다음과 같이 구성합니다:

- 러너에 대한 [Kubernetes 실행기](../executors/kubernetes/_index.md)를 사용하여 실행합니다.
- 각 새로운 CI/CD 작업에 대해 지정된 네임스페이스에서 새로운 Pod을 프로비저닝합니다.

## Helm 차트를 사용하여 러너 구성 {#configure-gitlab-runner-with-the-helm-chart}

GitLab 러너 구성 변경사항을 `values.yaml`에 저장합니다. 이 파일을 구성하는 데 도움이 되는 정보는 다음을 참조하세요:

- 차트 저장소의 기본값 [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) 구성입니다.
- [Values Files](https://helm.sh/docs/chart_template_guide/values_files/)에 대한 Helm 문서이며, values 파일이 기본값을 재정의하는 방법을 설명합니다.

러너가 제대로 작동하려면 구성 파일에서 이러한 값을 설정해야 합니다:

- `gitlabUrl`: 러너를 등록할 GitLab 서버의 전체 URL(예: `https://gitlab.example.com`)입니다.
- `rbac: { create: true }`: 러너가 작업을 실행할 Pod을 생성하도록 하는 RBAC(역할 기반 액세스 제어) 규칙을 만듭니다.
  - 기존 `serviceAccount`을 사용하려는 경우, `rbac`에서 서비스 계정 이름을 추가합니다:

    ```yaml
    rbac:
      create: false
    serviceAccount:
      create: false
      name: your-service-account
    ```

  - `serviceAccount`이 필요로 하는 최소 권한에 대해 알아보려면 [러너 API 권한 구성](../executors/kubernetes/_index.md#configure-runner-api-permissions)을 참조하세요.
- `runnerToken`: [GitLab UI에서 러너 만들기](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token)할 때 얻은 인증 토큰입니다.
  - 이 토큰을 직접 설정하거나 비밀에 저장합니다.

더 많은 [선택 사항 구성 설정](kubernetes_helm_chart_configuration.md)을 사용할 수 있습니다.

이제 [러너 설치](#install-gitlab-runner-with-the-helm-chart)할 준비가 되었습니다!

## Helm 차트를 사용하여 러너 설치 {#install-gitlab-runner-with-the-helm-chart}

전제 조건:

- GitLab 서버의 API는 클러스터에서 연결할 수 있습니다.
- Kubernetes 1.4 이상(베타 API 활성화).
- `kubectl` CLI는 로컬에 설치되어 있으며 클러스터에 인증되어 있습니다.
- [Helm 클라이언트](https://helm.sh/docs/using_helm/#installing-the-helm-client)가 머신에 로컬로 설치되어 있습니다.
- [`values.yaml`에 필수 값](#configure-gitlab-runner-with-the-helm-chart)을 모두 설정했습니다.

Helm 차트에서 러너를 설치하려면:

1. GitLab Helm 저장소를 추가합니다:

   ```shell
   helm repo add gitlab https://charts.gitlab.io
   ```

1. Helm 2를 사용하는 경우, `helm init`로 Helm을 초기화합니다.
1. 액세스할 수 있는 러너 버전을 확인합니다:

   ```shell
   helm search repo -l gitlab/gitlab-runner
   ```

1. 러너의 최신 버전에 액세스할 수 없는 경우, 다음 명령으로 차트를 업데이트합니다:

   ```shell
   helm repo update gitlab
   ```

1. 러너를 `values.yaml` 파일에서 [구성](#configure-gitlab-runner-with-the-helm-chart)한 후, 필요에 따라 매개변수를 변경하여 다음 명령을 실행합니다:

   ```shell
   # For Helm 2
   helm install --namespace <NAMESPACE> --name gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner

   # For Helm 3
   helm install --namespace <NAMESPACE> gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner
   ```

   - `<NAMESPACE>`: 러너를 설치할 Kubernetes 네임스페이스입니다.
   - `<CONFIG_VALUES_FILE>`: 사용자 지정 구성을 포함하는 values 파일의 경로입니다. 생성하려면 [Helm 차트를 사용하여 러너 구성](#configure-gitlab-runner-with-the-helm-chart)을 참조하세요.
   - 러너 Helm 차트의 특정 버전을 설치하려면 `--version <RUNNER_HELM_CHART_VERSION>`을 `helm install` 명령에 추가합니다. 차트의 모든 버전을 설치할 수 있지만, 더 최신의 `values.yml`는 차트의 이전 버전과 호환되지 않을 수 있습니다.

### 사용 가능한 러너 Helm 차트 버전 확인 {#check-available-gitlab-runner-helm-chart-versions}

Helm 차트와 러너는 동일한 버전 관리를 따르지 않습니다. 두 버전 간의 버전 매핑을 확인하려면 Helm 버전에 대한 명령을 실행합니다:

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

출력의 예:

```plaintext
NAME                  CHART VERSION APP VERSION DESCRIPTION
gitlab/gitlab-runner  0.64.0        16.11.0     GitLab Runner
gitlab/gitlab-runner  0.63.0        16.10.0     GitLab Runner
gitlab/gitlab-runner  0.62.1        16.9.1      GitLab Runner
gitlab/gitlab-runner  0.62.0        16.9.0      GitLab Runner
gitlab/gitlab-runner  0.61.3        16.8.1      GitLab Runner
gitlab/gitlab-runner  0.61.2        16.8.0      GitLab Runner
...
```

## Helm 차트를 사용하여 러너 업그레이드 {#upgrade-gitlab-runner-with-the-helm-chart}

전제 조건:

- 러너 차트를 설치했습니다.
- GitLab에서 러너를 일시 중지했습니다. 이렇게 하면 [완료 시 권한 부족 오류](../faq/_index.md#helm-chart-error--unauthorized)와 같이 작업으로 인해 발생하는 문제를 방지할 수 있습니다.
- 모든 작업이 완료되었는지 확인했습니다.

구성을 변경하거나 차트를 업데이트하려면 `helm upgrade`을 사용하고 필요에 따라 매개변수를 변경합니다:

```shell
helm upgrade --namespace <NAMESPACE> -f <CONFIG_VALUES_FILE> <RELEASE-NAME> gitlab/gitlab-runner
```

- `<NAMESPACE>`: 러너를 설치한 Kubernetes 네임스페이스입니다.
- `<CONFIG_VALUES_FILE>`: 사용자 지정 구성을 포함하는 values 파일의 경로입니다. 생성하려면 [Helm 차트를 사용하여 러너 구성](#configure-gitlab-runner-with-the-helm-chart)을 참조하세요.
- `<RELEASE-NAME>`: 차트를 설치할 때 부여한 이름입니다. 설치 섹션에서 예제는 `gitlab-runner`로 이름을 지었습니다.
- 최신 버전이 아닌 러너 Helm 차트의 특정 버전으로 업데이트하려면 `--version <RUNNER_HELM_CHART_VERSION>`을 `helm upgrade` 명령에 추가합니다.

## Helm 차트를 사용하여 러너 제거 {#uninstall-gitlab-runner-with-the-helm-chart}

러너를 제거하려면:

1. GitLab에서 러너를 일시 중지하고 모든 작업이 완료되었는지 확인합니다. 이렇게 하면 [완료 시 권한 부족 오류](../faq/_index.md#helm-chart-error--unauthorized)와 같은 작업 관련 문제를 방지할 수 있습니다.
1. 이 명령을 실행하고 필요에 따라 수정합니다:

   ```shell
   helm delete --namespace <NAMESPACE> <RELEASE-NAME>
   ```

   - `<NAMESPACE>`은 러너가 설치된 Kubernetes 네임스페이스입니다.
   - `<RELEASE-NAME>`은 차트를 설치할 때 부여한 이름입니다. 이 페이지의 [설치 섹션](#install-gitlab-runner-with-the-helm-chart)에서 `gitlab-runner`으로 이름을 지었습니다.
