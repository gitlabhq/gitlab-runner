---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너 Helm 차트 문제 해결
---

## 오류: `Job failed (system failure): secrets is forbidden` {#error-job-failed-system-failure-secrets-is-forbidden}

다음 오류가 표시되면 [RBAC 지원 활성화](kubernetes_helm_chart_configuration.md#enable-rbac-support)를 통해 오류를 해결하세요:

```plaintext
Using Kubernetes executor with image alpine ...
ERROR: Job failed (system failure): secrets is forbidden: User "system:serviceaccount:gitlab:default"
cannot create resource "secrets" in API group "" in the namespace "gitlab"
```

## 오류: `Unable to mount volumes for pod` {#error-unable-to-mount-volumes-for-pod}

볼륨 마운트 실패가 표시되고 필수 시크릿이 필요한 경우, 등록 토큰 또는 러너 토큰이 시크릿에 저장되어 있는지 확인하세요.

## Google Cloud Storage로 느린 아티팩트 업로드 {#slow-artifact-uploads-to-google-cloud-storage}

Google Cloud Storage로의 아티팩트 업로드는 러너 헬퍼 Pod가 CPU 바운드 상태가 되어 성능 저하(더 느린 대역폭 속도)가 발생할 수 있습니다. 이 문제를 해결하려면 Helper Pod CPU 제한을 증가하세요:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        helper_cpu_limit = "250m"
```

자세한 내용은 [이슈 28393](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28393#note_722733798)을 참조하세요.

## 오류: `PANIC: creating directory: mkdir /nonexistent: permission denied` {#error-panic-creating-directory-mkdir-nonexistent-permission-denied}

이 오류를 해결하려면 [Ubuntu 기반 GitLab Runner Docker 이미지](kubernetes_helm_chart_configuration.md#switch-to-the-ubuntu-based-gitlab-runner-docker-image)로 전환하세요.

## 오류: `invalid header field for "Private-Token"` {#error-invalid-header-field-for-private-token}

`runner-token` 값이 `gitlab-runner-secret`에서 줄바꿈 문자(`\n`)를 포함하여 base64로 인코딩된 경우 이 오류가 표시될 수 있습니다:

```plaintext
couldn't execute POST against "https:/gitlab.example.com/api/v4/runners/verify":
net/http: invalid header field for "Private-Token"
```

이 문제를 해결하려면 줄바꿈(`\n`)이 토큰 값에 추가되지 않도록 하세요. 예: `echo -n <gitlab-runner-token> | base64`.

## 오류: `FATAL: Runner configuration is reserved` {#error-fatal-runner-configuration-is-reserved}

GitLab Runner Helm 차트를 설치한 후 Pod 로그에서 다음 오류가 표시될 수 있습니다:

```plaintext
FATAL: Runner configuration other than name and executor configuration is reserved
(specifically --locked, --access-level, --run-untagged, --maximum-timeout, --paused, --tag-list, and --maintenance-note)
and cannot be specified when registering with a runner authentication token. This configuration is specified
on the GitLab server. Please try again without specifying any of those arguments
```

이 오류는 인증 토큰을 사용하고 시크릿을 통해 토큰을 제공할 때 발생합니다. 이 문제를 해결하려면 values YAML 파일을 검토하고 더 이상 사용되지 않는 값을 사용하지 않고 있는지 확인하세요. 더 이상 사용되지 않는 값에 대한 자세한 내용은 [Helm 차트를 사용하여 GitLab Runner 설치](https://docs.gitlab.com/ci/runners/new_creation_workflow/#installing-gitlab-runner-with-helm-chart)를 참조하세요.
