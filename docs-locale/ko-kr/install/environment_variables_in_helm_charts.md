---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner Helm 차트에서 환경 변수 설정
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

환경 변수는 애플리케이션이 런타임에 동작을 조정하는 데 사용할 수 있는 정보를 포함하는 키-값 쌍입니다. 이러한 변수는 컨테이너의 환경에 주입됩니다. 이러한 변수를 사용하여 구성 데이터, 시크릿 또는 애플리케이션에 필요한 기타 동적 정보를 전달할 수 있습니다.

다음을 사용하여 GitLab Runner Helm 차트에서 환경 변수를 설정할 수 있습니다:

- [`runners.config` 속성](#use-the-runnersconfig-property)
- [`values.yaml`의 속성](#use-valuesyaml-properties)

## `runners.config` 속성 사용 {#use-the-runnersconfig-property}

`runners.config` 속성을 통해 환경 변수를 구성할 수 있으며, 이는 `config.toml` 파일에서 수행하는 것과 유사합니다:

```yaml
runners:
  config: |
    [[runners]]
      shell = "bash"
      [runners.kubernetes]
        host = ""
        environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true"]
```

이렇게 정의된 변수는 작업 Pod과 GitLab Runner Manager 컨테이너 모두에 적용됩니다. 위의 예에서 `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` 기능 플래그는 환경 변수로 설정되며, GitLab Runner Manager가 동작을 수정하는 데 사용합니다.

## `values.yaml` 속성 사용 {#use-valuesyaml-properties}

`values.yaml`에서 다음 속성을 사용하여 환경 변수를 설정할 수 있습니다. 이러한 변수는 GitLab Runner Manager 컨테이너에만 영향을 미칩니다.

- `envVars`

  ```yaml
  envVars:
    - name: RUNNER_EXECUTOR
      value: kubernetes
  ```

- `extraEnv`

  ```yaml
  extraEnv:
    CACHE_S3_SERVER_ADDRESS: s3.amazonaws.com
    CACHE_S3_BUCKET_NAME: runners-cache
    CACHE_S3_BUCKET_LOCATION: us-east-1
    CACHE_SHARED: true
  ```

- `extraEnvFrom`

  ```yaml
  extraEnvFrom:
    CACHE_S3_ACCESS_KEY:
      secretKeyRef:
        name: s3access
        key: accesskey
    CACHE_S3_SECRET_KEY:
      secretKeyRef:
        name: s3access
        key: secretkey
  ```

  `extraEnvFrom`에 대한 자세한 정보를 보려면:

  - [`Distribute Credentials Securely Using Secrets`](https://kubernetes.io/docs/tasks/inject-data-application/distribute-credentials-secure/)
  - [`Use container fields as values for environment variables`](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/#use-container-fields-as-values-for-environment-variables)
