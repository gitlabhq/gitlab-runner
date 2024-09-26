---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Runner Helm Chart

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, Self-managed

The official way of deploying a GitLab Runner instance into your
Kubernetes cluster is by using the `gitlab-runner` Helm chart.

This chart configures GitLab Runner to:

- Run using the [Kubernetes executor](../executors/kubernetes/index.md) for GitLab Runner.
- Provision a new pod in the specified namespace for each new CI/CD job.

## Configuring GitLab Runner using the Helm Chart

Store your GitLab Runner configuration changes in `values.yaml`. For help configuring this file, see:

- The default [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)
  configuration in the chart repository.
- The Helm documentation for [Values Files](https://helm.sh/docs/chart_template_guide/values_files/), which explains
  how your values file overrides the default values.

For GitLab Runner to run properly, you must set these values in your configuration file:

- `gitlabUrl`: The full URL of the GitLab server (like `https://gitlab.example.com`) to register the runner against.
- `rbac: { create: true }`: Create RBAC (role-based access control) rules for the GitLab Runner to create
  pods to run jobs in.
  - Prefer to use an existing `serviceAccount`? You should also set `rbac: { serviceAccountName: "SERVICE_ACCOUNT_NAME" }`.
  - To learn about the minimal permissions the `serviceAccount` requires, see
    [Configure runner API permissions](../executors/kubernetes/index.md#configure-runner-api-permissions).
- `runnerToken`: The authentication token obtained when you
  [create a runner in the GitLab UI](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-an-instance-runner-with-a-runner-authentication-token).
  - Set this token directly or [store it in a secret](#store-registration-tokens-or-runner-tokens-in-secrets).

For more configuration options, see [additional configuration](#additional-configuration).

You're now ready to [install GitLab Runner](#installing-gitlab-runner-using-the-helm-chart)!

## Installing GitLab Runner using the Helm Chart

Prerequisites:

- Your GitLab server's API is reachable from the cluster.
- Kubernetes 1.4 or later, with beta APIs enabled.
- The `kubectl` CLI is installed locally, and authenticated for the cluster.
- The [Helm client](https://helm.sh/docs/using_helm/#installing-the-helm-client) is installed locally on your machine.
- You've set all [required values in `values.yaml`](#configuring-gitlab-runner-using-the-helm-chart).

To install GitLab Runner by using the Helm chart:

1. Add the GitLab Helm repository:

   ```shell
   helm repo add gitlab https://charts.gitlab.io
   ```

1. If you use Helm 2, initialize Helm with `helm init`.
1. Check which GitLab Runner versions you have access to:

   ```shell
   helm search repo -l gitlab/gitlab-runner
   ```

1. If you can't access the latest versions of GitLab Runner, update the chart with this command:

   ```shell
   helm repo update gitlab
   ```

1. After you [configure](#configuring-gitlab-runner-using-the-helm-chart) GitLab Runner in your `values.yaml` file,
   run this command, changing parameters as needed:

   ```shell
   # For Helm 2
   helm install --namespace <NAMESPACE> --name gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner

   # For Helm 3
   helm install --namespace <NAMESPACE> gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner
   ```

   - `<NAMESPACE>`: The Kubernetes namespace where you want to install the GitLab Runner.
   - `<CONFIG_VALUES_FILE>`: The path to values file containing your custom configuration. To create it, see
     [Configuring GitLab Runner using the Helm Chart](#configuring-gitlab-runner-using-the-helm-chart).
   - To install a specific version of the GitLab Runner Helm Chart, add `--version <RUNNER_HELM_CHART_VERSION>`
     to your `helm install` command. You can install any version of the chart, but more recent `values.yml` might
     be incompatible with older versions of the chart.

### Check available GitLab Runner Helm Chart versions

Helm Chart and GitLab Runner do not follow the same versioning. To see version mappings
between the two, run the command for your version of Helm:

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

An example of the output:

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

## Upgrading GitLab Runner using the Helm Chart

Prerequisites:

- You've installed your GitLab Runner Chart.
- You've paused the runner in GitLab. This prevents problems arising with the jobs, such as
  [authorization errors when they complete](../faq/index.md#helm-chart-error--unauthorized).
- You've ensured all jobs have completed.

To change your configuration or update charts, use `helm upgrade`, changing parameters as needed:

```shell
helm upgrade --namespace <NAMESPACE> -f <CONFIG_VALUES_FILE> <RELEASE-NAME> gitlab/gitlab-runner
```

- `<NAMESPACE>`: The Kubernetes namespace where you've installed GitLab Runner.
- `<CONFIG_VALUES_FILE>`: The path to the values file containing your custom configuration. To create it, see
  [Configure GitLab Runner using the Helm Chart](#configuring-gitlab-runner-using-the-helm-chart).
- `<RELEASE-NAME>`: The name you gave the chart when installing it.
  In the installation section, the example named it `gitlab-runner`.
- To update to a specific version of the GitLab Runner Helm Chart, rather than the latest one, add
  `--version <RUNNER_HELM_CHART_VERSION>` to your `helm upgrade` command.

## Uninstalling GitLab Runner using the Helm Chart

To uninstall GitLab Runner:

1. Pause the runner in GitLab, and ensure any jobs have completed. This prevents job-related problems, such as
   [authorization errors on completion](../faq/index.md#helm-chart-error--unauthorized).
1. Run, this command, modifying it as needed:

   ```shell
   helm delete --namespace <NAMESPACE> <RELEASE-NAME>
   ```

   - `<NAMESPACE>` is the Kubernetes namespace where GitLab Runner is installed.
   - `<RELEASE-NAME>` is the name you gave the chart when you installed it.
     In the [installation section](#installing-gitlab-runner-using-the-helm-chart) of this page, we called it `gitlab-runner`.

## Additional configuration

> - [Introduced](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/106) [configuration template](../register/index.md#register-with-a-configuration-template) in Helm Chart 0.23.0. See [deprecation issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222).

You can use a [configuration template file](../register/index.md#register-with-a-configuration-template)
to [configure the behavior of GitLab Runner build pod in Kubernetes](../executors/kubernetes/index.md#configuration-settings).
You can use the configuration template to configure any field on the runner,
without having the Helm chart be aware of specific runner configuration options.

Here's a snippet of the default settings [found in the `values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) in the chart repository:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:22.04"
```

NOTE:
In the `config:` section, the format should be `toml` (`<parameter> = <value>` instead of `<parameter>: <value>`), as we are embedding `config.toml` in `values.yaml`.

The executor-specific configuration [is documented in the `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).

## Using cache with configuration template

To use the cache with your configuration template, set the following variables in `values.yaml`:

- `runners.cache.secretName` with the secret name for your object storage provider (`s3access`, `gcsaccess`, `google-application-credentials`, or `azureaccess`).
- `runners.config` with the other settings for [the cache](../configuration/advanced-configuration.md#the-runnerscache-section). Use `toml` formatting.

### S3

For example, here is an example that configures [S3 with static credentials](https://aws.amazon.com/blogs/security/wheres-my-secret-access-key/):

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

Next, create an `s3access` Kubernetes secret that contains `accesskey` and `secretkey`:

```shell
kubectl create secret generic s3access \
    --from-literal=accesskey="YourAccessKey" \
    --from-literal=secretkey="YourSecretKey"
```

### Google Cloud Storage (GCS)

### Static credentials directly configured

The following example shows how to configure
[GCS with credentials with an access ID and a private key](../configuration/advanced-configuration.md#the-runnerscache-section):

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

Next, create a `gcsaccess` Kubernetes secret that contains `gcs-access-id`
and `gcs-private-key`:

```shell
kubectl create secret generic gcsaccess \
    --from-literal=gcs-access-id="YourAccessID" \
    --from-literal=gcs-private-key="YourPrivateKey"
```

### Static credentials in a JSON file downloaded from GCP

The following example shows how to
[configure GCS with credentials in a JSON file](../configuration/advanced-configuration.md#the-runnerscache-section)
downloaded from Google Cloud Platform:

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

Next, create a Kubernetes secret `google-application-credentials` and
load the JSON file with it:

```shell
kubectl create secret generic google-application-credentials \
    --from-file=gcs-application-credentials-file=./path-to-your-google-application-credentials-file.json
```

### Azure

The following example shows
[how to configure Azure Blob Storage](../configuration/advanced-configuration.md#the-runnerscacheazure-section):

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

Next, create an `azureaccess` Kubernetes secret that contains
`azure-account-name` and `azure-account-key`:

```shell
kubectl create secret generic azureaccess \
    --from-literal=azure-account-name="YourAccountName" \
    --from-literal=azure-account-key="YourAccountKey"
```

Read more about the caching in Helm Chart in [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).

## Enabling RBAC support

If your cluster has RBAC enabled, you can choose to either have the chart create
its own service account or [provide one on your own](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#service-account-permissions).

To have the chart create the service account for you, set `rbac.create` to true:

```yaml
rbac:
  create: true
```

To use an existing service account, use:

```yaml
rbac:
  create: false
  serviceAccountName: your-service-account
```

## Controlling maximum Runner concurrency

A single GitLab Runner deployed on Kubernetes is able to execute multiple jobs
in parallel by automatically starting additional Runner pods. The
[`concurrent` setting](../configuration/advanced-configuration.md#the-global-section)
controls the maximum number of pods allowed at a single time, and defaults to `10`:

```yaml
## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
concurrent: 10
```

## Running Docker-in-Docker containers with GitLab Runner

See [running privileged containers for the runners](#running-privileged-containers-for-the-runners) for how to enable it,
and the [GitLab Runner documentation](../executors/kubernetes/index.md#using-docker-in-builds) on running dind.

## Running privileged containers for the runners

You can tell the runner to run using privileged containers. You might need
this setting enabled if you need to use the Docker executable in your GitLab CI/CD jobs.

This comes with several risks that you can read about in the
[GitLab CI/CD Runner documentation](../executors/kubernetes/index.md#using-docker-in-builds).

You can enable privileged mode in `values.yaml` if:

- You understand the risks.
- Your GitLab Runner instance is registered against a specific project in GitLab that you trust the CI jobs of.

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        # Run all containers with the privileged flag enabled.
        # See https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runnerskubernetes-section for details.
        privileged = true
        ...
```

## Best practices for building containers without privileged mode

Building containers within containers with Docker-in-Docker requires Docker privileged
mode. Google's [Kaniko](https://github.com/GoogleContainerTools/kaniko) is an alternative
that works without privileged mode, and it has been tested on the Kubernetes GitLab Runner.

The <i class="fa fa-youtube-play youtube" aria-hidden="true"></i>
[Least Privilege Container Builds with Kaniko on GitLab](https://www.youtube.com/watch?v=d96ybcELpFs)
video is a walkthrough of the [Kaniko Docker Build](https://gitlab.com/guided-explorations/containers/kaniko-docker-build)
working example project. It makes use of the documentation for
[Building images with Kaniko and GitLab CI/CD](https://docs.gitlab.com/ee/ci/docker/using_kaniko.html).
<!-- Video published on 2020-04-07 -->

For testing, copy the working example project to your own group or instance. More details on what other GitLab CI patterns are demonstrated are available at the project page [Kaniko Docker Build](https://gitlab.com/guided-explorations/containers/kaniko-docker-build).

## Using an image from a private registry

Using an image from a private registry requires the configuration of `imagePullSecrets`. For more details on how to create `imagePullSecrets`, see [Pull an Image from a Private Registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/) in the Kubernetes documentation.

You must create one or more secrets in the Kubernetes namespace used for the CI/CD job.

You can use the following command to create a secret that works with `image_pull_secrets`:

```yaml
kubectl create secret docker-registry <SECRET_NAME> \
  --namespace <NAMESPACE> \
  --docker-server="https://<REGISTRY_SERVER>" \
  --docker-username="<REGISTRY_USERNAME>" \
  --docker-password="<REGISTRY_PASSWORD>"
```

In GitLab Runner Helm Chart v0.53.x and later, the deprecated properties are not supported anymore in the `values.yaml`
and `runners.imagePullSecrets` is one of them. To configure an `image_pull_secret`, users have to set it in
the `config.toml` template provided in `runners.config`:

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

In GitLab Runner Helm Chart v0.52.x and earlier, if you configure `runners.imagePullSecrets`, the container adds
`--kubernetes-image-pull-secrets "<SECRET_NAME>"` to the image entrypoint script. This eliminates the need to configure
the `image_pull_secrets` parameter in the Kubernetes executor `config.toml` settings.

```yaml
runners:
  imagePullSecrets: [your-image-pull-secret]
```

Take note of the format. The value is not prefixed by a `name` tag as is the convention in Kubernetes resources. It requires an _array_ of one or more secret names, even if you use only one registry credential.

## Providing a custom certificate for accessing GitLab

You can provide a [Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/)
to the GitLab Runner Helm Chart, which is used to populate the container's
`/home/gitlab-runner/.gitlab-runner/certs` directory.

Each key name in the Secret is used as a filename in the directory, with the
file content being the value associated with the key:

- The key/filename used should be in the format `<gitlab.hostname>.crt`, for example
  `gitlab.your-domain.com.crt`.
- Concatenate any intermediate certificates together with your server certificate in the same file.
- The hostname used should be the one the certificate is registered for.

If you installed GitLab Helm Chart using the [auto-generated self-signed wildcard certificate](https://docs.gitlab.com/charts/installation/tls.html#option-4-use-auto-generated-self-signed-wildcard-certificate) method, a secret is created for you.

```yaml
## Set the certsSecretName to pass custom certificates for GitLab Runner to use
## Provide resource name for a Kubernetes Secret Object in the same namespace,
## this is used to populate the /home/gitlab-runner/.gitlab-runner/certs/ directory
## ref: https://docs.gitlab.com/runner/configuration/tls-self-signed.html#supported-options-for-self-signed-certificates-targeting-the-gitlab-server
##
certsSecretName: RELEASE-wildcard-tls-chain
```

The GitLab Runner Helm Chart does not create a secret for you. To create
the secret, tell Kubernetes to store the certificate as a secret, and present it
to the GitLab Runner containers as a file. To do this, run the following command:

```shell
kubectl create secret generic <SECRET_NAME> \
  --namespace <NAMESPACE> \
  --from-file=<CERTIFICATE_FILENAME>
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where you want to install the GitLab Runner.
- `<SECRET_NAME>` is the Kubernetes Secret resource name. (For example: `gitlab-domain-cert`.)
- `<CERTIFICATE_FILENAME>` is the filename for the certificate in your current directory to import into the secret.

Specify the filename to use on the target if either:

- The source file `<CERTIFICATE_FILENAME>` is not in the current directory.
- The source file does not follow the format `<gitlab.hostname.crt>`.

```shell
kubectl create secret generic <SECRET_NAME> \
  --namespace <NAMESPACE> \
  --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
```

Where:

- `<TARGET_FILENAME>` is the name of the certificate file as presented to the Runner
  containers. (For example: `gitlab.hostname.crt`.)
- `<CERTIFICATE_FILENAME>` is the filename for the certificate relative to your
  current directory to import into the secret. (For example:
  `cert-directory/my-gitlab-certificate.crt`)

You then need to provide the secret's name to the GitLab Runner chart.
Add the following to your `values.yaml`:

```yaml
## Set the certsSecretName in order to pass custom certificates for GitLab Runner to use
## Provide resource name for a Kubernetes Secret Object in the same namespace,
## this is used to populate the /home/gitlab-runner/.gitlab-runner/certs/ directory
## ref: https://docs.gitlab.com/runner/configuration/tls-self-signed.html#supported-options-for-self-signed-certificates
##
certsSecretName: <SECRET NAME>
```

Where:

- `<SECRET_NAME>` is the Kubernetes Secret resource name, as in the above example, `gitlab-domain-cert`.

For more information on how GitLab Runner uses these certificates, see
[Supported options for self-signed certificates targeting the GitLab server](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates-targeting-the-gitlab-server).

## Set pod labels to CI environment variables keys

You can't use environment variables as pod labels in the `values.yaml` file.
For more information, see [Can't set environment variable key as pod label](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173).
Use [the workaround described in the issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890) as a temporary solution.

## Store registration tokens or runner tokens in secrets

> - Introduced in [16.1](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/merge_requests/405).

To register a runner created in the GitLab UI, you specify the `runnerToken` in `values.yml`. GitLab shows the `runnerToken` briefly in
the UI when you create the runner.

It can be a security risk to store tokens in `values.yml`, especially if you commit these to Git. Instead, you can store the values of these tokens in a
[Kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/), and
then update the `runners.secret` value in `values.yml` with the name of
the secret.

If you have an existing registered runner and want to use that, set the
`runner-token` with the token used to identify that runner. If you want
to have a new runner registered you can set the
`runner-registration-token` with a
[registration token](https://docs.gitlab.com/ee/ci/runners/) ([deprecated](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/102681)).

For example:

1. Create a secret with registration token:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-runner-secret
type: Opaque
data:
  runner-registration-token: "" # need to leave as an empty string for compatibility reasons
  runner-token: "REDACTED"
```

```shell
kubectl apply --namespace <NAMESPACE> -f gitlab-runner-secret.yaml
```

1. Configure the following in `values.yaml`:

```yaml
runners:
  secret: gitlab-runner-secret
```

This example uses the secret `gitlab-runner-secret` and takes the value of
`runner-token` to register the runner.

NOTE:
If your secret management solution doesn't allow you to set empty string for `runner-registration-token`,
you can set it to any string. When `runner-token` is present, the string is ignored.

## Switching to the Ubuntu-based `gitlab-runner` Docker image

By default the GitLab Runner Helm Chart uses the Alpine version of the `gitlab/gitlab-runner` image,
which uses `musl libc`. In some cases, consider switching to the Ubuntu-based image, which uses `glibc`.

To do so, update your `values.yaml` file with the following values:

```yaml
# Specify the Ubuntu image. Remember to set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v16.5.0

# Update the security context values to the user ID in the ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

## Running with non-root user

By default, the GitLab Runner images don't work with non-root users. The [GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421) and [GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433)
images are designed for that scenario. To use them, change the GitLab Runner and GitLab Runner Helper images:

NOTE:
Although `run_as_user` points to the user ID of `nonroot` user (59417), the images work with any user ID.
It's important that this user ID is part of the root group. Being part of the root group doesn't give it any specific privileges.

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

## Using FIPS compliant GitLab Runner

To use a [FIPS compliant GitLab Runner](index.md#fips-compliant-gitlab-runner), change the GitLab Runner image and the Helper image as follows:

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
