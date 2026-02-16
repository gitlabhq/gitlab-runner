---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configure the GitLab Runner Helm chart
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

You can add optional configuration to your GitLab Runner Helm chart.

## Use the cache with a configuration template

To use the cache with your configuration template, set these variables in `values.yaml`:

- `runners.cache.secretName`: The secret name for your object storage provider.
  Options: `s3access`, `gcsaccess`, `google-application-credentials`, or `azureaccess`.
- `runners.config`: Other settings for [the cache](../configuration/advanced-configuration.md#the-runnerscache-section), in TOML format.

### Amazon S3

To configure [Amazon S3 with static credentials](https://aws.amazon.com/blogs/security/wheres-my-secret-access-key/):

1. Add this example to your `values.yaml`, changing values where needed:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "s3"
           Path = "runner"
           Shared = true
           [runners.cache.s3]
             ServerAddress = "s3.amazonaws.com"
             BucketName = "my_bucket_name"
             BucketLocation = "eu-west-1"
             Insecure = false
             AuthenticationType = "access-key"

     cache:
         secretName: s3access
   ```

1. Create an `s3access` Kubernetes secret that contains `accesskey` and `secretkey`:

   ```shell
   kubectl create secret generic s3access \
       --from-literal=accesskey="YourAccessKey" \
       --from-literal=secretkey="YourSecretKey"
   ```

### Google Cloud Storage (GCS)

Google Cloud Storage can be configured with static credentials in multiple ways.

#### Static credentials directly configured

To configure GCS with credentials
[with an access ID and a private key](../configuration/advanced-configuration.md#the-runnerscache-section):

1. Add this example to your `values.yaml`, changing values where needed:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
       secretName: gcsaccess
   ```

1. Create a `gcsaccess` Kubernetes secret that contains `gcs-access-id` and `gcs-private-key`:

   ```shell
   kubectl create secret generic gcsaccess \
       --from-literal=gcs-access-id="YourAccessID" \
       --from-literal=gcs-private-key="YourPrivateKey"
   ```

#### Static credentials in a JSON file downloaded from GCP

To [configure GCS with credentials in a JSON file](../configuration/advanced-configuration.md#the-runnerscache-section)
downloaded from Google Cloud Platform:

1. Add this example to your `values.yaml`, changing values where needed:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
         secretName: google-application-credentials

   secrets:
     - name: google-application-credentials
   ```

1. Create a Kubernetes secret called `google-application-credentials` and load the JSON file with it. Change the path as needed:

   ```shell
   kubectl create secret generic google-application-credentials \
       --from-file=gcs-application-credentials-file=./PATH-TO-CREDENTIALS-FILE.json
   ```

### Azure

To [configure Azure Blob Storage](../configuration/advanced-configuration.md#the-runnerscacheazure-section):

1. Add this example to your `values.yaml`, changing values where needed:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "azure"
           Path = "runner"
           Shared = true
           [runners.cache.azure]
             ContainerName = "CONTAINER_NAME"
             StorageDomain = "blob.core.windows.net"

     cache:
         secretName: azureaccess
   ```

1. Create an `azureaccess` Kubernetes secret that contains `azure-account-name` and `azure-account-key`:

   ```shell
   kubectl create secret generic azureaccess \
       --from-literal=azure-account-name="YourAccountName" \
       --from-literal=azure-account-key="YourAccountKey"
   ```

To learn more about Helm chart caching, see [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).

### Persistent volume claim

You can use persistent volume claims (PVCs) for caching if none of the object storage options work for you.

To configure your cache to use a PVC:

1. [Create a PVC](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims) in the namespace where job pods will run.

   {{< alert type="note" >}}

   If you want multiple job pods to access the same cache PVC, it must have the `ReadWriteMany` access mode.

   {{< /alert >}}

1. Mount the PVC to the `/cache` directory:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [[runners.kubernetes.volumes.pvc]]
           name = "cache-pvc"
           mount_path = "/cache"
   ```

## Enable RBAC support

If your cluster has RBAC (role-based access controls) enabled, the chart can create
its own service account, or you can
[provide one](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#service-account-permissions).

- To have the chart create the service account for you, set `rbac.create` to true:

  ```yaml
  rbac:
    create: true
  ```

- To use an existing service account, set a `serviceAccount.name`:

  ```yaml
  rbac:
    create: false
  serviceAccount:
    create: false
    name: your-service-account
  ```

## Control maximum runner concurrency

A single runner deployed on Kubernetes can run multiple jobs in parallel by starting additional Runner pods.
To change the maximum number of pods allowed at one time, edit the
[`concurrent` setting](../configuration/advanced-configuration.md#the-global-section). It defaults to `10`:

```yaml
## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
concurrent: 10
```

For more information about this setting, see [the global section](../configuration/advanced-configuration.md#the-global-section)
in the advanced configuration documentation for GitLab Runner.

## Run Docker-in-Docker containers with GitLab Runner

To use Docker-in-Docker containers with GitLab Runner:

- To enable it, see [Use privileged containers for the runners](#use-privileged-containers-for-the-runners).
- For instructions on running Docker-in-Docker, see the
  [GitLab Runner documentation](../executors/kubernetes/_index.md#using-docker-in-builds).

## Use privileged containers for the runners

To use the Docker executable in your GitLab CI/CD jobs, configure the runner to use privileged containers.

Prerequisites:

- You understand the risks, which are described in the
  [GitLab CI/CD Runner documentation](../executors/kubernetes/_index.md#using-docker-in-builds).
- Your GitLab Runner instance is registered against a specific project in GitLab, and you trust its CI/CD jobs.

To enable privileged mode in `values.yaml`, add these lines:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        # Run all containers with the privileged flag enabled.
        privileged = true
        ...
```

For more information, see the advanced configuration information about the
[`[runners.kubernetes]`](../configuration/advanced-configuration.md#the-runnerskubernetes-section) section.

## Use an image from a private registry

To use an image from a private registry, configure `imagePullSecrets`.

1. Create one or more secrets in the Kubernetes namespace used for the CI/CD job. This command creates a secret
   that works with `image_pull_secrets`:

   ```shell
   kubectl create secret docker-registry <SECRET_NAME> \
     --namespace <NAMESPACE> \
     --docker-server="https://<REGISTRY_SERVER>" \
     --docker-username="<REGISTRY_USERNAME>" \
     --docker-password="<REGISTRY_PASSWORD>"
   ```

1. For GitLab Runner Helm chart version 0.53.x and later, in `config.toml`, set `image_pull_secret` from the template
   provided in `runners.config`:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           ## Specify one or more imagePullSecrets
           ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
           ##
           image_pull_secrets = [your-image-pull-secret]
   ```

   For more information, see
   [Pull an image from a private registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
   in the Kubernetes documentation.

1. For GitLab Runner Helm chart version 0.52 and earlier, in `values.yaml`, set a value for `runners.imagePullSecrets`.
   When you set this value, the container adds `--kubernetes-image-pull-secrets "<SECRET_NAME>"` to the image entrypoint script.
   This eliminates the need to configure the `image_pull_secrets` parameter in the Kubernetes executor `config.toml` settings.

   ```yaml
   runners:
     imagePullSecrets: [your-image-pull-secret]
   ```

{{< alert type="note" >}}

The value of `imagePullSecrets` is not prefixed by a `name` tag, as is the convention in Kubernetes resources. This value requires
an array of one or more secret names, even if you use only one registry credential.

{{< /alert >}}

For more details on how to create `imagePullSecrets`, see
[Pull an Image from a Private Registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
in the Kubernetes documentation.

{{< alert type="note" >}}

When a job Pod is being created, GitLab Runner automatically handles image access in two steps:

1. GitLab Runner converts any existing Docker credentials into Kubernetes secrets so they can pull images from registries.
   It also checks that any manually configured imagePullSecrets actually exist in the cluster.
   For more information about statically defined credentials, credentials stores, or credential helpers, see
   [Access an image from a private container registry](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry).
1. GitLab Runner creates the job Pod and attaches both types of credentials to it:
   the `imagePullSecrets` and the converted Docker credentials, in that order.

When Kubernetes needs to pull the container image, it tries the credentials one by one until it finds the one that works.

{{< /alert >}}

## Access GitLab with a custom certificate

To use a custom certificate, provide a [Kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/)
to the GitLab Runner Helm chart. This secret is added to the container's
`/home/gitlab-runner/.gitlab-runner/certs` directory:

1. [Prepare your certificate](#prepare-your-certificate)
1. [Create a Kubernetes secret](#create-a-kubernetes-secret)
1. [Provide the secret to the chart](#provide-the-secret-to-the-chart)

### Prepare your certificate

Each key name in the Kubernetes secret is used as a filename in the directory, with the
file content being the value associated with the key:

- The filename used should be in the format `<gitlab.hostname>.crt`, for example
  `gitlab.your-domain.com.crt`.
- Concatenate any intermediate certificates together with your server certificate in the same file.
- The hostname used should be the one the certificate is registered for.

### Create a Kubernetes secret

If you installed GitLab Helm chart using the
[auto-generated self-signed wildcard certificate](https://docs.gitlab.com/charts/installation/tls/#option-4-use-auto-generated-self-signed-wildcard-certificate) method, a secret was created for you.

If you did not install GitLab Helm chart with the auto-generated self-signed wildcard certificate, create a secret.
These commands store your certificate as a secret in Kubernetes, and present it to the GitLab Runner containers as a file.

- If your certificate is in the current directory, and follows the format `<gitlab.hostname.crt>`,
  modify this command as needed:

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<CERTIFICATE_FILENAME>
  ```

  - `<NAMESPACE>`: The Kubernetes namespace where you want to install the GitLab Runner.
  - `<SECRET_NAME>`: The Kubernetes Secret resource name, like `gitlab-domain-cert`.
  - `<CERTIFICATE_FILENAME>`: The filename for the certificate in your current directory to import into the secret.

- If your certificate is in another directory, or doesn't follow the format `<gitlab.hostname.crt>`, you must
  specify the filename to use as the target:

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
  ```

  - `<TARGET_FILENAME>` is the name of the certificate file as presented to the Runner
    containers, like `gitlab.hostname.crt`.
  - `<CERTIFICATE_FILENAME>` is the filename for the certificate, relative to your
    current directory, to import into the secret. For example:
    `cert-directory/my-gitlab-certificate.crt`.

### Provide the secret to the chart

In `values.yaml`, set `certsSecretName` to the resource name of a Kubernetes secret object in the same namespace.
This enables you to pass your custom certificate for GitLab Runner to use. In the previous example, the resource
name was `gitlab-domain-cert`:

```yaml
certsSecretName: <SECRET NAME>
```

For more information, see the
[supported options for self-signed certificates](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates-targeting-the-gitlab-server)
targeting the GitLab server.

## Set pod labels to CI environment variable keys

You can't use environment variables as pod labels in the `values.yaml` file.
For more information, see [Can't set environment variable key as pod label](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173).
Use [the workaround described in the issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890) as a temporary solution.

## Switch to the Ubuntu-based `gitlab-runner` Docker image

By default, the GitLab Runner Helm chart uses the Alpine version of the `gitlab/gitlab-runner` image,
which uses `musl libc`. You might need to switch to the Ubuntu-based image, which uses `glibc`.

To do this, specify the image your `values.yaml` file with the following values:

```yaml
# Specify the Ubuntu image, and set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v17.3.0

# Update the security context values to the user ID in the Ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

## Run with non-root user

By default, the GitLab Runner images don't work with non-root users. The
[GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421) and
[GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433)
images are designed for that scenario.

To use them, change the GitLab Runner and GitLab Runner Helper images in `values.yaml`:

```yaml
image:
  registry: registry.gitlab.com
  image: gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-ocp
  tag: v16.11.0

securityContext:
    runAsNonRoot: true
    runAsUser: 999

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image = "registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp:x86_64-v16.11.0"
            [runners.kubernetes.pod_security_context]
              run_as_non_root = true
              run_as_user = 59417
```

Although `run_as_user` points to the user ID of `nonroot` user (59417), the images work with any user ID.
It's important that this user ID is part of the root group. Being part of the root group doesn't give it any specific privileges.

## Use a FIPS-compliant GitLab Runner

To use a [FIPS-compliant GitLab Runner](requirements.md#fips-compliant-gitlab-runner), change the GitLab Runner image
and the Helper image in `values.yaml`:

```yaml
image:
  registry: docker.io
  image: gitlab/gitlab-runner
  tag: ubi-fips

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image_flavor = "ubi-fips"
```

## Use a configuration template

To [configure the behavior of GitLab Runner build pod in Kubernetes](../executors/kubernetes/_index.md#configuration-settings),
use a [configuration template file](../register/_index.md#register-with-a-configuration-template).
Configuration templates can configure any field on the runner, without sharing specific runner configuration options
with the Helm chart. For example, these default settings
[found in the `values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) in the `chart` repository:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:22.04"
```

Values in the `config:` section should use TOML (`<parameter> = <value>` instead of `<parameter>: <value>`, as
`config.toml` is embedded in `values.yaml`.

For executor-specific configuration, see [the `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) file.
