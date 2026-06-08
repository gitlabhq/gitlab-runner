---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab Operator를 사용하여 Kubernetes에서 GitLab 러너를 설치합니다.
title: GitLab 러너 Operator 설치
---

## Red Hat OpenShift에 설치 {#install-on-red-hat-openshift}

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Red Hat OpenShift v4 이상에서 [GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator)를 사용하여 러너를 설치할 수 있습니다. OpenShift 웹 콘솔에서 OperatorHub의 안정적인 채널을 사용합니다. 설치 후, 새로 배포된 GitLab Runner 인스턴스를 사용하여 GitLab CI/CD 작업을 실행할 수 있습니다. 각 CI/CD 작업은 별도의 Pod에서 실행됩니다.

### 필수 요구 사항 {#prerequisites}

- 관리자 권한이 있는 OpenShift 4.x 클러스터
- GitLab Runner 등록 토큰

### OpenShift Operator 설치 {#install-the-openshift-operator}

먼저 OpenShift Operator를 설치해야 합니다.

1. OpenShift UI를 열고 관리자 권한이 있는 사용자로 로그인합니다.
1. 왼쪽 창에서 **Operators**를 선택한 다음 **OperatorHub**를 선택합니다.
1. 주 창의 **All Items** 아래에서 `GitLab Runner` 키워드를 검색합니다.

   ![GitLab Operator](img/openshift_allitems_v13_3.png)

1. 설치하려면 GitLab Runner Operator를 선택합니다.
1. GitLab Runner Operator 요약 페이지에서 **설치**를 선택합니다.
1. Operator 설치 페이지에서:
   1. **Update Channel** 아래에서 **stable**을 선택합니다.
   1. **Installed Namespace** 아래에서 원하는 네임스페이스를 선택한 다음 **설치**를 선택합니다.

   ![GitLab Operator Install Page](img/openshift_installoperator_v13_3.png)

설치된 Operators 페이지에서 GitLab Operator가 준비되면 상태가 **성공함**으로 변경됩니다.

![GitLab Operator Install Status](img/openshift_success_v13_3.png)

## Kubernetes에 설치 {#install-on-kubernetes}

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Kubernetes v1.21 이상에서 [GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator) 를 사용하여 러너를 설치합니다. [OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator)의 안정적인 채널을 사용합니다. 설치 후, 새로 배포된 GitLab Runner 인스턴스를 사용하여 GitLab CI/CD 작업을 실행할 수 있습니다. 각 CI/CD 작업은 별도의 Pod에서 실행됩니다.

### 필수 요구 사항 {#prerequisites-1}

- Kubernetes v1.21 이상
- Cert manager v1.7.1

### Kubernetes Operator 설치 {#install-the-kubernetes-operator}

[OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator)의 지침을 따릅니다.

1. 필수 구성 요소를 설치합니다.
1. 오른쪽 상단에서 **설치**를 선택하고 지침을 따라 `olm` 및 Operator를 설치합니다.

#### GitLab Runner 설치 {#install-gitlab-runner}

1. 러너 인증 토큰을 획득합니다. 다음 중 하나를 수행할 수 있습니다:
   - [instance](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token) , [group](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-group-runner-with-a-runner-authentication-token) , 또는 [project](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token) runner를 생성합니다.
   - `config.toml` 파일에서 러너 인증 토큰을 찾습니다. 러너 인증 토큰의 접두사는 `glrt-`입니다.
1. GitLab Runner 토큰으로 비밀 파일을 만듭니다:

   ```shell
   cat > gitlab-runner-secret.yml << EOF
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-secret
   type: Opaque
   # Only one of the following fields can be set. The Operator fails to register the runner if both are provided.
   # NOTE: runner-registration-token is deprecated and will be removed in GitLab 18.0. You should use runner-token instead.
   stringData:
     runner-token: REPLACE_ME # your project runner token
     # runner-registration-token: "" # your project runner secret
   EOF
   ```

1. 클러스터에서 `secret`을 만듭니다. 다음 명령을 실행합니다:

   ```shell
   kubectl apply -f gitlab-runner-secret.yml
   ```

1. CRD(Custom Resource Definition) 파일을 만들고 다음 구성을 포함합니다.

   ```shell
   cat > gitlab-runner.yml << EOF
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: gitlab-runner
   spec:
     gitlabUrl: https://gitlab.example.com
     buildImage: alpine
     token: gitlab-runner-secret
   EOF
   ```

1. 이제 `CRD` 파일을 다음 명령을 실행하여 적용합니다:

   ```shell
   kubectl apply -f gitlab-runner.yml
   ```

1. 다음 명령을 실행하여 GitLab Runner가 설치되었는지 확인합니다:

   ```shell
   kubectl get runner
   NAME             AGE
   gitlab-runner    5m
   ```

1. 러너 Pod도 표시되어야 합니다:

   ```shell
   kubectl get pods
   NAME                             READY   STATUS    RESTARTS   AGE
   gitlab-runner-bf9894bdb-wplxn    1/1     Running   0          5m
   ```

#### OpenShift용 GitLab Runner Operator의 다른 버전 설치 {#install-other-versions-of-gitlab-runner-operator-for-openshift}

Red Hat OperatorHub에서 사용 가능한 GitLab Runner Operator 버전을 사용하지 않으려면 다른 버전을 설치할 수 있습니다.

공식 사용 가능한 Operator 버전을 확인하려면 [`gitlab-runner-operator` 네임스페이스의 태그를 봅니다](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/tags). Operator가 실행 중인 GitLab Runner의 버전을 확인하려면 관심 있는 커밋 또는 태그의 `APP_VERSION` 파일 내용을 봅니다. 예를 들어 <https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/blob/1-17-stable/APP_VERSION>입니다.

특정 버전을 설치하려면 `catalogsource.yaml` 파일을 만들고 `<VERSION>`를 태그 또는 특정 커밋으로 바꿉니다:

> [!note]
> 특정 커밋에 대한 이미지를 사용할 때 태그 형식은 `v0.0.1-<COMMIT>`입니다. 예: `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:v0.0.1-f5a798af`.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: gitlab-runner-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:<VERSION>
  displayName: GitLab Runner Operators
  publisher: GitLab Community
```

`CatalogSource`을 만듭니다:

```shell
oc apply -f catalogsource.yaml
```

1분 후 새 러너가 OpenShift 클러스터의 OperatorHub 섹션에 표시됩니다.

## 오프라인 환경의 Kubernetes 클러스터에 GitLab Runner Operator 설치 {#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments}

전제 조건:

- 설치 프로세스에 필요한 이미지에 액세스할 수 있습니다.

설치 중에 컨테이너 이미지를 끌어오려면 GitLab Runner Operator가 외부 네트워크의 공개 인터넷에 연결되어야 합니다. 오프라인 환경에 설치된 Kubernetes 클러스터가 있는 경우 로컬 이미지 레지스트리 또는 패키지 리포지토리를 사용하여 설치 중에 이미지 또는 패키지를 끌어옵니다.

로컬 리포지토리는 다음 이미지를 제공해야 합니다:

| 이미지                                                 | 기본값 |
|-------------------------------------------------------|---------------|
| **GitLab Runner Operator** 이미지                      | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator:vGITLAB_RUNNER_OPERATOR_VERSION` |
| **GitLab Runner** 및 **GitLab Runner Helper** 이미지 | 이러한 이미지는 GitLab Runner UBI 이미지 레지스트리에서 다운로드되며 Runner Custom Resources를 설치할 때 사용됩니다. 사용되는 버전은 요구 사항에 따라 다릅니다. |
| **RBAC Proxy** 이미지                                  | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/openshift4/ose-kube-rbac-proxy:v4.13.0` |

1. 연결이 끊긴 네트워크 환경에서 로컬 리포지토리 또는 레지스트리를 설정하여 다운로드된 소프트웨어 패키지 및 컨테이너 이미지를 호스팅합니다. 다음을 사용할 수 있습니다:

   - 컨테이너 이미지용 Docker 레지스트리입니다.
   - Kubernetes 바이너리 및 종속성을 위한 로컬 패키지 레지스트리입니다.

1. GitLab Runner Operator v1.23.2 이상에서는 `operator.k8s.yaml` 파일의 최신 버전을 다운로드합니다:

   ```shell
   curl -O "https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-
   operator/-/releases/vGITLAB_RUNNER_OPERATOR_VERSION/downloads/operator.k8s.yaml"
   ```

1. `operator.k8s.yaml` 파일에서 다음 URL을 업데이트합니다:

   - `GitLab Runner Operator image`
   - `RBAC Proxy image`

1. `operator.k8s.yaml` 파일의 업데이트된 버전을 설치합니다:

   ```shell
   kubectl apply -f PATH_TO_UPDATED_OPERATOR_K8S_YAML
   GITLAB_RUNNER_OPERATOR_VERSION = 1.23.2+
   ```

## Operator 제거 {#uninstall-operator}

### Red Hat OpenShift에서 제거 {#uninstall-on-red-hat-openshift}

1. Runner `CRD`를 삭제합니다:

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. `secret`를 삭제합니다:

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Red Hat 설명서에서 [Deleting Operators from a cluster using the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.7/html/operators/administrator-tasks#olm-deleting-operators-from-a-cluster-using-web-console_olm-deleting-operators-from-a-cluster)의 지침을 따릅니다.

### Kubernetes에서 제거 {#uninstall-on-kubernetes}

1. Runner `CRD`를 삭제합니다:

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. `secret`를 삭제합니다:

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Operator 구독을 삭제합니다:

   ```shell
   kubectl delete subscription my-gitlab-runner-operator -n operators
   ```

1. 설치된 `CSV`의 버전을 확인합니다:

   ```shell
   kubectl get clusterserviceversion -n operators
   NAME                            DISPLAY         VERSION   REPLACES   PHASE
   gitlab-runner-operator.v1.7.0   GitLab Runner   1.7.0                Succeeded
   ```

1. `CSV`를 삭제합니다:

   ```shell
   kubectl delete clusterserviceversion gitlab-runner-operator.v1.7.0 -n operators
   ```

#### 구성 {#configuration}

OpenShift에서 GitLab Runner를 구성하려면 [Configuring GitLab Runner on OpenShift](../configuration/configuring_runner_operator.md) 페이지를 참조합니다.

#### 모니터링 {#monitoring}

GitLab Runner Operator 배포에 대한 모니터링 및 메트릭 수집을 활성화하려면 [Monitor GitLab Runner Operator](../monitoring/_index.md#monitor-operator-managed-gitlab-runners)를 참조합니다.
