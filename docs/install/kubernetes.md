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
- For each new job it receives from GitLab CI/CD, provision a new pod within the specified namespace to run it.

## Prerequisites

- Your GitLab server's API is reachable from the cluster.
- Kubernetes 1.4+ with beta APIs enabled.
- The `kubectl` CLI installed locally and authenticated for the cluster.
- The [Helm client](https://helm.sh/docs/using_helm/#installing-the-helm-client) installed locally on your machine.

## Installing GitLab Runner using the Helm Chart

Add the GitLab Helm repository:

```shell
helm repo add gitlab https://charts.gitlab.io
```

If using Helm 2, you must also initialize Helm:

```shell
helm init
```

If you are unable to access to the latest versions of GitLab Runner, you should update the chart. To update the chart, run:

```shell
helm repo update gitlab
```

To view a list of GitLab Runner versions you have access to, run:

```shell
helm search repo -l gitlab/gitlab-runner
```

Once you [have configured](#configuring-gitlab-runner-using-the-helm-chart) GitLab Runner in your `values.yaml` file,
run the following:

```shell
# For Helm 2
helm install --namespace <NAMESPACE> --name gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner

# For Helm 3
helm install --namespace <NAMESPACE> gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where you want to install the GitLab Runner.
- `<CONFIG_VALUES_FILE>` is the path to values file containing your custom configuration. See the
  [Configuring GitLab Runner using the Helm Chart](#configuring-gitlab-runner-using-the-helm-chart) section to create it.

If you want to install a specific version of GitLab Runner Helm Chart, add `--version <RUNNER_HELM_CHART_VERSION>`
to your `helm install` command. You can install any version of the chart
this way, however more recent `values.yml` may not work with an older version of the chart.

## Upgrading GitLab Runner using the Helm Chart

Before upgrading GitLab Runner, pause the runner in GitLab and ensure any jobs have completed.
Pausing the runner prevents problems arising with the jobs, such as
[authorization errors when they complete](../faq/index.md#helm-chart-error--unauthorized).

Once your GitLab Runner Chart is installed, configuration changes and chart updates should be done using `helm upgrade`:

```shell
helm upgrade --namespace <NAMESPACE> -f <CONFIG_VALUES_FILE> <RELEASE-NAME> gitlab/gitlab-runner
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where GitLab Runner is installed.
- `<CONFIG_VALUES_FILE>` is the path to values file containing your custom configuration. See the
  [Configuring GitLab Runner using the Helm Chart](#configuring-gitlab-runner-using-the-helm-chart) section to create it.
- `<RELEASE-NAME>` is the name you gave the chart when installing it.
  In the [Installing GitLab Runner using the Helm Chart](#installing-gitlab-runner-using-the-helm-chart) section, we called it `gitlab-runner`.

If you want to update to a specific version of GitLab Runner Helm Chart instead of the latest one, add `--version <RUNNER_HELM_CHART_VERSION>`
to your `helm upgrade` command.

## Check available GitLab Runner Helm Chart versions

Versions of Helm Chart and GitLab Runner do not follow the same versioning.
Use the command below to get version mappings between Helm Chart and GitLab Runner:

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

Example of the output is shown below:

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

## Configuring GitLab Runner using the Helm Chart

Create a `values.yaml` file for your GitLab Runner configuration. See
[Helm docs](https://helm.sh/docs/chart_template_guide/#values-files)
for information on how your values file will override the defaults.

The default configuration can always be found in the
[`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)
in the chart repository.

### Required configuration

For GitLab Runner to function, your configuration file **must** specify the following:

- `gitlabUrl`: The GitLab server full URL to register the runner against. For example, `https://gitlab.example.com`.
- `rbac: { create: true }`: Create RBAC rules for the GitLab Runner to create pods to run jobs in. If you have an existing `serviceAccount` you prefer to use, you should also set `rbac: { serviceAccountName: "SERVICE_ACCOUNT_NAME" }`. For more information about the minimal permissions required for the `serviceAccount`, see [Configure runner API permissions](../executors/kubernetes/index.md#configure-runner-api-permissions).
- `runnerToken`:
  - The authentication token you obtain when you [create a runner in the GitLab UI](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-an-instance-runner-with-a-runner-authentication-token).
  - Set the token directly or [store it in a secret](#store-registration-tokens-or-runner-tokens-in-secrets).

  \- or -

- `runnerRegistrationToken` ([deprecated](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/102681) in GitLab 15.6):
  - The registration token [retrieved from your GitLab instance](https://docs.gitlab.com/ee/ci/runners/).
  - Set the token directly or [store it in a secret](#store-registration-tokens-or-runner-tokens-in-secrets).
  - Registration tokens have been deprecated and will be removed in GitLab 17.0.

Unless you need to specify any additional configuration, you are
ready to [install GitLab Runner](#installing-gitlab-runner-using-the-helm-chart).

### Additional configuration

> - [Introduced](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/106) [configuration template](../register/index.md#register-with-a-configuration-template) in Helm Chart 0.23.0. See [deprecation issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222).

You can use a [configuration template file](../register/index.md#register-with-a-configuration-template)
to [configure the behavior of GitLab Runner build pod within Kubernetes](../executors/kubernetes/index.md#configuration-settings).
You can use the configuration template to configure any field on the runner,
without having the Helm chart be aware of specific runner configuration options.

Here's a snippet of the default settings [found in the `values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) in the chart repository. It is important to note that, for the `config:` section, the format should be `toml` (`<parameter> = <value>` instead of `<parameter>: <value>`), as we are embedding `config.toml` in `values.yaml`.

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:22.04"
```

The executor-specific configuration [is documented in the `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).

### Using cache with configuration template

To use the cache with your configuration template, set the following variables in `values.yaml`:

- `runners.cache.secretName` with the secret name for your object storage provider (`s3access`, `gcsaccess`, `google-application-credentials`, or `azureaccess`).
- `runners.config` with the other settings for [the cache](../configuration/advanced-configuration.md#the-runnerscache-section). Use `toml` formatting.

#### S3

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

#### Google Cloud Storage (GCS)

#### Static credentials directly configured

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

#### Static credentials in a JSON file downloaded from GCP

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

#### Azure

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

### Enabling RBAC support

If your cluster has RBAC enabled, you can choose to either have the chart create
its own service account or [provide one on your own](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#service-account-permissions).

To have the chart create the service account for you, set `rbac.create` to true:

```yaml
rbac:
  create: true
```

To use an already existing service account, use:

```yaml
rbac:
  create: false
  serviceAccountName: your-service-account
```

### Controlling maximum Runner concurrency

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

### Running Docker-in-Docker containers with GitLab Runner

See [running privileged containers for the runners](#running-privileged-containers-for-the-runners) for how to enable it,
and the [GitLab Runner documentation](../executors/kubernetes/index.md#using-docker-in-builds) on running dind.

### Running privileged containers for the runners

You can tell the GitLab Runner to run using privileged containers. You may need
this enabled if you need to use the Docker executable within your GitLab CI/CD jobs.

This comes with several risks that you can read about in the
[GitLab CI/CD Runner documentation](../executors/kubernetes/index.md#using-docker-in-builds).

If you are okay with the risks, and your GitLab Runner instance is registered
against a specific project in GitLab that you trust the CI jobs of, you can
enable privileged mode in `values.yaml`:

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

### Best practices for building containers without privileged mode

Building containers within containers with Docker-in-Docker requires Docker privileged
mode. Google's [Kaniko](https://github.com/GoogleContainerTools/kaniko) is an alternative
that works without privileged mode, and it has been tested on the Kubernetes GitLab Runner.

The [Least Privilege Container Builds with Kaniko on GitLab](https://www.youtube.com/watch?v=d96ybcELpFs)
video is a walkthrough of the [Kaniko Docker Build](https://gitlab.com/guided-explorations/containers/kaniko-docker-build)
working example project. It makes use of the documentation for
[Building images with Kaniko and GitLab CI/CD](https://docs.gitlab.com/ee/ci/docker/using_kaniko.html).

The working example project can be copied to your own group or instance for testing. More details on what other GitLab CI patterns are demonstrated are available at the project page [Kaniko Docker Build](https://gitlab.com/guided-explorations/containers/kaniko-docker-build).

### Using an image from a private registry

Using an image from a private registry requires the configuration of imagePullSecrets. For more details on how to create imagePullSecrets [see the documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/).

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
the `config.toml` template provided in `runners.config`

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

Take note of the format. The value is not prefixed by a `name` tag as is the convention in Kubernetes resources. An array of one or more secret names is required, regardless of whether or not you're using multiple registry credentials.

### Providing a custom certificate for accessing GitLab

You can provide a [Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/)
to the GitLab Runner Helm Chart, which will be used to populate the container's
`/home/gitlab-runner/.gitlab-runner/certs` directory.

Each key name in the Secret will be used as a filename in the directory, with the
file content being the value associated with the key:

- The key/filename used should be in the format `<gitlab.hostname>.crt`, for example
  `gitlab.your-domain.com.crt`.
- Any intermediate certificates need to be concatenated to your server certificate in the same file.
- The hostname used should be the one the certificate is registered for.

If you installed GitLab Helm Chart using the [auto-generated self-signed wildcard certificate](https://docs.gitlab.com/charts/installation/tls.html#option-4-use-auto-generated-self-signed-wildcard-certificate) method a secret is created for you.

```yaml
## Set the certsSecretName to pass custom certificates for GitLab Runner to use
## Provide resource name for a Kubernetes Secret Object in the same namespace,
## this is used to populate the /home/gitlab-runner/.gitlab-runner/certs/ directory
## ref: https://docs.gitlab.com/runner/configuration/tls-self-signed.html#supported-options-for-self-signed-certificates-targeting-the-gitlab-server
##
certsSecretName: RELEASE-wildcard-tls-chain
```

The GitLab Runner Helm Chart does not create a secret for you. In order to create
the secret, you tell Kubernetes to store the certificate as a secret and present it
to the GitLab Runner containers as a file. To do this, run the following command:

```shell
kubectl create secret generic <SECRET_NAME> \
  --namespace <NAMESPACE> \
  --from-file=<CERTIFICATE_FILENAME>
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where you want to install the GitLab Runner.
- `<SECRET_NAME>` is the Kubernetes Secret resource name. (For example: `gitlab-domain-cert`.)
- `<CERTIFICATE_FILENAME>` is the filename for the certificate in your current directory that will be imported into the secret.

If the source file `<CERTIFICATE_FILENAME>` is not in the current directory or
does not follow the format `<gitlab.hostname.crt>` then it will be necessary to
specify the filename to use on the target:

```shell
kubectl create secret generic <SECRET_NAME> \
  --namespace <NAMESPACE> \
  --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
```

Where:

- `<TARGET_FILENAME>` is the name of the certificate file as presented to the Runner
  containers. (For example: `gitlab.hostname.crt`.)
- `<CERTIFICATE_FILENAME>` is the filename for the certificate relative to your
  current directory that will be imported into the secret. (For example:
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

More information on how GitLab Runner uses these certificates can be found in the
[Runner Documentation](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates-targeting-the-gitlab-server).

### Set pod labels to CI environment variables keys

At the moment it is not possible to use environment variables as pod labels within the `values.yaml` file.
We are working on it in this issue: [Can't set environment variable key as pod label](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173).
Use [the workaround described in the issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890) as a temporary solution.

### Store registration tokens or runner tokens in secrets

> - Introduced in [16.1](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/merge_requests/405).

To register a runner that was created in the GitLab UI, you specify the `runnerToken` in `values.yml`. The `runnerToken` is displayed briefly in
the UI when you create the runner.

It can be a security risk to store tokens in `values.yml`, especially if you commit these to `git`. Instead, you can store the values of these tokens inside of a
[Kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/), and
then update the `runners.secret` value in `values.yml` with the name of
the secret.

If you have an existing registered runner and want to use that, set the
`runner-token` with the token used to identify that runner. If you want
to have a new runner registered you can set the
`runner-registration-token` with a
[registration token](https://docs.gitlab.com/ee/ci/runners/) ([deprecated](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/102681)).

For example:

1. Create a secret with registration token

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
you can set it to any string - it will be ignored when `runner-token` is present.

### Switching to the Ubuntu-based `gitlab-runner` Docker image

By default the GitLab Runner Helm Chart uses the Alpine version of the `gitlab/gitlab-runner` image,
which uses `musl libc`. In some cases, you may want to switch to the Ubuntu-based image, which uses `glibc`.

To do so, update your `values.yaml` file with the following values:

```yaml
# Specify the Ubuntu image. Remember to set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v16.5.0

# Update the security context values to the user ID in the ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

### Running with non-root user

By default, the GitLab Runner images will not work with non-root users. The [GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421) and [GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433)
images are designed for that scenario. To use them change the GitLab Runner and GitLab Runner Helper images:

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

### Using FIPS compliant GitLab Runner

To use a [FIPS compliant GitLab Runner](index.md#fips-compliant-gitlab-runner) change the GitLab Runner image and the Helper image as follows:

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

## Uninstalling GitLab Runner using the Helm Chart

Before uninstalling GitLab Runner, pause the runner in GitLab and ensure any jobs have completed.
Pausing the runner prevents problems arising with the jobs, such as
[authorization errors when they complete](../faq/index.md#helm-chart-error--unauthorized).

To uninstall the GitLab Runner Chart, run the following:

```shell
helm delete --namespace <NAMESPACE> <RELEASE-NAME>
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where GitLab Runner is installed.
- `<RELEASE-NAME>` is the name you gave the chart when installing it.
  In the [Installing GitLab Runner using the Helm Chart](#installing-gitlab-runner-using-the-helm-chart) section, we called it `gitlab-runner`.

## Troubleshooting a Kubernetes installation

### `ERROR: Job failed (system failure): secrets is forbidden`

If you see the following error:

```plaintext
Using Kubernetes executor with image alpine ...
ERROR: Job failed (system failure): secrets is forbidden: User "system:serviceaccount:gitlab:default" cannot create resource "secrets" in API group "" in the namespace "gitlab"
```

[Enable RBAC support](#enabling-rbac-support) to correct the error.

### `Unable to mount volumes for pod`

If you see mount volume failures for a required secret, ensure that you've followed
[Store registration tokens or runner tokens in secrets](#store-registration-tokens-or-runner-tokens-in-secrets).

### Slow artifact uploads to Google Cloud Storage

<!-- See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28393#note_722733798 -->

Artifact uploads to Google Cloud Storage can experience reduced performance due to the runner helper pod becoming CPU bound. This will appear in the form of a slow bandwidth rate.

This can be mitigated by increasing the Helper pod CPU Limit:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        helper_cpu_limit = "250m"
```

### `PANIC: creating directory: mkdir /nonexistent: permission denied`

To resolve this error, [switch to the Ubuntu-based GitLab Runner Docker image](#switching-to-the-ubuntu-based-gitlab-runner-docker-image).

### Error: `invalid header field for "Private-Token"`

You might get the following error if the `runner-token` value in `gitlab-runner-secret`
is base64-encoded with a newline character at the end:

```plaintext
couldn't execute POST against "https:/gitlab.example.com/api/v4/runners/verify": net/http: invalid header field for "Private-Token"
```

To resolve this issue, ensure a newline is not appended to the token value (for example, `echo -n glrt-A5sFGybkt0pY8AdVLnx4 | base64`).

### Runner configuration is reserved

You might get the following error in the pod logs after installing the GitLab Runner Helm chart:

```plaintext
FATAL: Runner configuration other than name and executor configuration is reserved (specifically --locked, --access-level, --run-untagged, --maximum-timeout, --paused, --tag-list, and --maintenance-note) and cannot be specified when registering with a runner authentication token. This configuration is specified on the GitLab server. Please try again without specifying any of those arguments
```

It happens if you use an authentication token and [provide a token through a secret](#store-registration-tokens-or-runner-tokens-in-secrets).
To fix this error, review your values YAML file and make sure that you are not using any deprecated values. For more information about which values are deprecated, see [Installing GitLab Runner with Helm chart](https://docs.gitlab.com/ee/ci/runners/new_creation_workflow.html#installing-gitlab-runner-with-helm-chart).
