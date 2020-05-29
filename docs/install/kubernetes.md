# GitLab Runner Helm Chart

NOTE: **Note:**
This chart has been tested on Google Kubernetes Engine and Azure Container Service.
Other Kubernetes installations may work as well, if not please
[open an issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues).

The official way of deploying a GitLab Runner instance into your
Kubernetes cluster is by using the `gitlab-runner` Helm chart.

This chart configures the Runner to:

- Run using the GitLab Runner [Kubernetes executor](../executors/kubernetes.md).
- For each new job it receives from GitLab CI/CD, it will provision a
  new pod within the specified namespace to run it.

## Prerequisites

- Your GitLab Server's API is reachable from the cluster.
- Kubernetes 1.4+ with Beta APIs enabled.
- The `kubectl` CLI installed locally and authenticated for the cluster.
- The [Helm client](https://helm.sh/docs/using_helm/#installing-the-helm-client) installed locally on your machine.

## Configuring GitLab Runner using the Helm Chart

Create a `values.yaml` file for your GitLab Runner configuration. See
[Helm docs](https://helm.sh/docs/chart_template_guide/#values-files)
for information on how your values file will override the defaults.

The default configuration can always be found in the
[`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/master/values.yaml)
in the chart repository.

### Required configuration

In order for GitLab Runner to function, your configuration file **must** specify the following:

- `gitlabUrl` - the GitLab server full URL (e.g., `https://example.gitlab.com`) to register the Runner against.
- `runnerRegistrationToken` - The registration token for adding new Runners to
  GitLab. This must be [retrieved from your GitLab instance](https://docs.gitlab.com/ee/ci/runners/).

Unless you need to specify any additional configuration, you are
ready to [install the Runner](#installing-gitlab-runner-using-the-helm-chart).

### Additional configuration

The rest of the configuration is
[documented in the `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/master/values.yaml) in the chart repository.

Here is a snippet of the important settings:

```yaml
## The GitLab Server URL (with protocol) that want to register the runner against
## ref: https://docs.gitlab.com/runner/commands/README.html#gitlab-runner-register
##
gitlabUrl: https://gitlab.example.com/

## The registration token for adding new Runners to the GitLab server. This must
## be retrieved from your GitLab instance.
## ref: https://docs.gitlab.com/ee/ci/runners/
##
runnerRegistrationToken: ""

## Set the certsSecretName in order to pass custom certificates for GitLab Runner to use
## Provide resource name for a Kubernetes Secret Object in the same namespace,
## this is used to populate the /etc/gitlab-runner/certs directory
## ref: https://docs.gitlab.com/runner/configuration/tls-self-signed.html#supported-options-for-self-signed-certificates
##
#certsSecretName:

## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
concurrent: 10

## Defines in seconds how often to check GitLab for a new builds
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
checkInterval: 30

## For RBAC support:
rbac:
  create: false

  ## Run the gitlab-bastion container with the ability to deploy/manage containers of jobs
  ## cluster-wide or only within namespace
  clusterWideAccess: false

  ## If RBAC is disabled in this Helm chart, use the following Kubernetes Service Account name.
  ##
  # serviceAccountName: default

## Configuration for the Pods that the runner launches for each new job
##
runners:
  ## Default container image to use for builds when none is specified
  ##
  image: ubuntu:18.04

  ## Run all containers with the privileged flag enabled
  ## This will allow the docker:stable-dind image to run if you need to run Docker
  ## commands. Please read the docs before turning this on:
  ## ref: https://docs.gitlab.com/runner/executors/kubernetes.html#using-docker-dind
  ##
  privileged: false

  ## Namespace to run Kubernetes jobs in (defaults to 'default')
  ##
  # namespace:

  ## Build Container specific configuration
  ##
  builds:
    # cpuLimit: 200m
    # memoryLimit: 256Mi
    cpuRequests: 100m
    memoryRequests: 128Mi

  ## Service Container specific configuration
  ##
  services:
    # cpuLimit: 200m
    # memoryLimit: 256Mi
    cpuRequests: 100m
    memoryRequests: 128Mi

  ## Helper Container specific configuration
  ##
  helpers:
    # cpuLimit: 200m
    # memoryLimit: 256Mi
    cpuRequests: 100m
    memoryRequests: 128Mi
```

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

### Running Docker-in-Docker containers with GitLab Runners

See [Running Privileged Containers for the Runners](#running-privileged-containers-for-the-runners) for how to enable it,
and the [GitLab Runner documentation](../executors/kubernetes.md#using-docker-in-your-builds) on running dind.

### Running privileged containers for the Runners

You can tell the GitLab Runner to run using privileged containers. You may need
this enabled if you need to use the Docker executable within your GitLab CI/CD jobs.

This comes with several risks that you can read about in the
[GitLab CI/CD Runner documentation](../executors/kubernetes.md#using-docker-in-your-builds).

If you are okay with the risks, and your GitLab Runner instance is registered
against a specific project in GitLab that you trust the CI jobs of, you can
enable privileged mode in `values.yaml`:

```yaml
runners:
  ## Run all containers with the privileged flag enabled
  ## This will allow the docker:stable-dind image to run if you need to run Docker
  ## commands. Please read the docs before turning this on:
  ## ref: https://docs.gitlab.com/runner/executors/kubernetes.html#using-docker-dind
  ##
  privileged: true
```

### Best practices for building containers without privileged mode

Building containers within containers with Docker-in-Docker requires Docker privileged
mode. Google's [Kaniko](https://github.com/GoogleContainerTools/kaniko) is an alternative
that works without privileged mode, and it has been tested on the GitLab Kubernetes Runner.

The [Least Privilege Container Builds with Kaniko on GitLab](https://www.youtube.com/watch?v=d96ybcELpFs)
video is a walkthrough of the [Kaniko Docker Build](https://gitlab.com/guided-explorations/containers/kaniko-docker-build)
working example project. It makes use of the documentation for
[Building images with Kaniko and GitLab CI/CD](https://docs.gitlab.com/ee/ci/docker/using_kaniko.html).

The working example project can be copied to your own group or instance for testing. More details on what other GitLab CI patterns are demonstrated are available at the project page [Kaniko Docker Build](https://gitlab.com/guided-explorations/containers/kaniko-docker-build).

### Providing a custom certificate for accessing GitLab

You can provide a [Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/)
to the GitLab Runner Helm Chart, which will be used to populate the container's
`/etc/gitlab-runner/certs` directory.

Each key name in the Secret will be used as a filename in the directory, with the
file content being the value associated with the key:

- The key/file name used should be in the format `<gitlab-hostname>.crt`, for example
  `gitlab.your-domain.com.crt`.
- Any intermediate certificates need to be concatenated to your server certificate in the same file.
- The hostname used should be the one the certificate is registered for.

The GitLab Runner Helm Chart does not create a secret for you. In order to create
the secret, you tell Kubernetes to store the certificate as a secret and present it
to the Runner containers as a file. To do this, run the following command:

```shell
kubectl
  --namespace <NAMESPACE>
  create secret generic <SECRET_NAME>
  --from-file=<CERTIFICATE_FILENAME>
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where you want to install the GitLab Runner.
- `<SECRET_NAME>` is the Kubernetes Secret resource name. (For example: `gitlab-domain-cert`.)
- `<CERTIFICATE_FILENAME>` is the filename for the certificate in your current directory that will be imported into the secret.

If the source file `<CERTIFICATE_FILENAME>` is not in the current directory or
does not follow the format `<gitlab-hostname.crt>` then it will be necessary to
specify the filename to use on the target:

```shell
kubectl
  --namespace <NAMESPACE>
  create secret generic <SECRET_NAME>
  --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
```

Where:

- `<TARGET_FILENAME>` is the name of the certificate file as presented to the Runner
  containers. (For example: `gitlab-hostname.crt`.)
- `<CERTIFICATE_FILENAME>` is the filename for the certificate relative to your
  current directory that will be imported into the secret. (For example:
  `cert-directory/my-gitlab-certificate.crt`)

You then need to provide the secret's name to the GitLab Runner chart.
Add the following to your `values.yaml`:

```yaml
## Set the certsSecretName in order to pass custom certificates for GitLab Runner to use
## Provide resource name for a Kubernetes Secret Object in the same namespace,
## this is used to populate the /etc/gitlab-runner/certs directory
## ref: https://docs.gitlab.com/runner/configuration/tls-self-signed.html#supported-options-for-self-signed-certificates
##
certsSecretName: <SECRET NAME>
```

Where:

- `<SECRET_NAME>` is the Kubernetes Secret resource name, as in the above example, `gitlab-domain-cert`.

More information on how GitLab Runner uses these certificates can be found in the
[Runner Documentation](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates).

### Set pod labels to CI environment variables keys

At the moment it is not possible to use environment variables as pod labels within the `values.yaml` file.
We are working on it in this issue: [Can't set environment variable key as pod label](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173).
Use [the workaround described in the issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890) as a temporary solution.

### Store registration tokens or Runner tokens in secrets

To register a new GitLab Runner, you can specify
`runnerRegistrationToken` in `values.yml`. To register an existing
Runner, you can use `runnerToken`. It can be a security risk to store
tokens in `values.yml`, especially if you commit these to `git`.

Instead, you can store the values of these tokens inside of a
[Kubernetes
secret](https://kubernetes.io/docs/concepts/configuration/secret/), and
then update the `runners.secret` value in `values.yml` with the name of
the secret.

If you have an existing registered Runner and want to use that, set the
`runner-token` with the token used to identify that Runner. If you want
to have a new Runner registered you can set the
`runner-registration-token` with the [registration token that you would
like](https://docs.gitlab.com/ee/ci/runners/).

For example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-runner-secret
type: Opaque
data:
  runner-registration-token: "NlZrN1pzb3NxUXlmcmVBeFhUWnIK" #base64 encoded registration token
  runner-token: ""
```

```yaml
runners:
  secret: gitlab-runner-secret
```

This example uses the secret `gitlab-runner-secret` and takes the value of
`runner-registration-token` to register the new GitLab Runner.

## Check available GitLab Runner Helm Chart versions

Versions of Helm Chart and GitLab Runner application do not follow the same versioning.
Use the command below to get version mappings between Helm Chart and GitLab Runner:

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

Example of the output is shown below:

```plaintext
NAME                    CHART VERSION   APP VERSION DESCRIPTION
...
gitlab/gitlab-runner    0.14.0          12.8.0      GitLab Runner
gitlab/gitlab-runner    0.13.1          12.7.1      GitLab Runner
gitlab/gitlab-runner    0.13.0          12.7.0      GitLab Runner
gitlab/gitlab-runner    0.12.0          12.6.0      GitLab Runner
gitlab/gitlab-runner    0.11.0          12.5.0      GitLab Runner
gitlab/gitlab-runner    0.10.1          12.4.1      GitLab Runner
gitlab/gitlab-runner    0.10.0          12.4.0      GitLab Runner
...
```

## Installing GitLab Runner using the Helm Chart

Add the GitLab Helm repository:

```shell
helm repo add gitlab https://charts.gitlab.io
```

If using Helm 2, you must also initialize Helm:

```shell
helm init
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
to your `helm install` command.

## Updating GitLab Runner using the Helm Chart

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

## Uninstalling GitLab Runner using the Helm Chart

To uninstall the GitLab Runner Chart, run the following:

```shell
helm delete --namespace <NAMESPACE> <RELEASE-NAME>
```

Where:

- `<NAMESPACE>` is the Kubernetes namespace where GitLab Runner is installed.
- `<RELEASE-NAME>` is the name you gave the chart when installing it.
  In the [Installing GitLab Runner using the Helm Chart](#installing-gitlab-runner-using-the-helm-chart) section, we called it `gitlab-runner`.
