---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Set environment variables in GitLab Runner Helm chart
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Environment variables are key-value pairs that contain information that applications can use to adjust their behavior at runtime.
These variables are injected into the container's environment. You can use these variables to pass configuration data, secrets, or any
other dynamic information required by the application.

You can set environment variables in GitLab Runner Helm chart by using the:

- [`runners.config` property](#use-the-runnersconfig-property)
- [Properties in `values.yaml`](#use-valuesyaml-properties)

## Use the `runners.config` property

You can configure environment variables through the `runners.config` property, similar to what you would do in the `config.toml` file:

```yaml
runners:
  config: |
    [[runners]]
      shell = "bash"
      [runners.kubernetes]
        host = ""
        environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true"]
```

Variables defined this way are applied to both the job Pod and the GitLab Runner Manager container.
In the example above, the `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` feature flag is set as an environment variable,
which the GitLab Runner Manager uses to modify its behavior.

## Use `values.yaml` properties

You can also set environment variables by using the following properties in `values.yaml`.
These variables only affect the GitLab Runner Manager container.

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

  For more information on `extraEnvFrom`, see:

  - [`Distribute Credentials Securely Using Secrets`](https://kubernetes.io/docs/tasks/inject-data-application/distribute-credentials-secure/)
  - [`Use container fields as values for environment variables`](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/#use-container-fields-as-values-for-environment-variables)
