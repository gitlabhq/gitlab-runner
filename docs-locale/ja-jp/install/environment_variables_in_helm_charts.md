---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner Helmチャートで環境変数を設定する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

環境変数は、アプリケーションがランタイム時の動作を調整するために使用できる情報を含むキー/バリューペアです。これらの変数は、コンテナの環境に挿入されます。これらの変数を使用して、アプリケーションに必要な設定データ、シークレット、またはその他の動的情報を渡すことができます。

GitLab Runner Helmチャートで環境変数を設定するには、次のものを使用します:

- [`runners.config`プロパティ](#use-the-runnersconfig-property)
- [`values.yaml`のプロパティ](#use-valuesyaml-properties)

## `runners.config`プロパティを使用してください。 {#use-the-runnersconfig-property}

`config.toml`ファイルで行うのと同様に、`runners.config`プロパティを使用して環境変数を設定できます:

```yaml
runners:
  config: |
    [[runners]]
      shell = "bash"
      [runners.kubernetes]
        host = ""
        environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true"]
```

この方法で定義された変数は、ジョブPodとGitLab Runner Managerコンテナの両方に適用されます。上記の例では、`FF_USE_ADVANCED_POD_SPEC_CONFIGURATION`機能フラグが環境変数として設定されており、GitLab Runner Managerがその動作を変更するために使用します。

## `values.yaml`プロパティの使用 {#use-valuesyaml-properties}

`values.yaml`の次のプロパティを使用して環境変数を設定することもできます。これらの変数は、GitLab Runner Managerコンテナにのみ影響します。

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
  extraEnvFrom: {}
    CACHE_S3_ACCESS_KEY:
      secretKeyRef:
        name: s3access
        key: accesskey
    CACHE_S3_SECRET_KEY:
      secretKeyRef:
        name: s3access
        key: secretkey
  ```

  `extraEnvFrom`の詳細については、以下を参照してください:

  - [`Distribute Credentials Securely Using Secrets`](https://kubernetes.io/docs/tasks/inject-data-application/distribute-credentials-secure/)
  - [`Use container fields as values for environment variables`](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/#use-container-fields-as-values-for-environment-variables)
