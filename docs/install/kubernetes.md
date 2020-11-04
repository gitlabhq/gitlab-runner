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

- `gitlabUrl` - the GitLab server full URL (e.g., `https://gitlab.example.com`) to register the Runner against.
- `runnerRegistrationToken` - The registration token for adding new Runners to
  GitLab. This must be [retrieved from your GitLab instance](https://docs.gitlab.com/ee/ci/runners/).

Unless you need to specify any additional configuration, you are
ready to [install the Runner](#installing-gitlab-runner-using-the-helm-chart).

### Additional configuration

> [Introduced](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/106) [configuration template](https://docs.gitlab.com/runner/register/#runners-configuration-template-file) in Helm Chart 0.23.0. See See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222.

The rest of the configuration is
[documented in the `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/master/values.yaml) in the chart repository.

Here is a snippet of the important settings:

```yaml
## GitLab Runner Image
##
## By default it's using gitlab/gitlab-runner:alpine-v{VERSION}
## where {VERSION} is taken from Chart.yaml from appVersion field
##
## ref: https://hub.docker.com/r/gitlab/gitlab-runner/tags/
##
## Note: If you change the image to the ubuntu release
##       don't forget to change the securityContext; 
##       these images run on different user IDs.
##
# image: gitlab/gitlab-runner:alpine-v11.6.0

## Specify a imagePullPolicy
## 'Always' if imageTag is 'latest', else set to 'IfNotPresent'
## ref: https://kubernetes.io/docs/concepts/containers/images/#pre-pulled-images
##
imagePullPolicy: IfNotPresent

## The GitLab Server URL (with protocol) that want to register the runner against
## ref: https://docs.gitlab.com/runner/commands/README.html#gitlab-runner-register
##
# gitlabUrl: http://gitlab.your-domain.com/

## The Registration Token for adding new Runners to the GitLab Server. This must
## be retrieved from your GitLab Instance.
## ref: https://docs.gitlab.com/ce/ci/runners/README.html
##
# runnerRegistrationToken: ""

## The Runner Token for adding new Runners to the GitLab Server. This must
## be retrieved from your GitLab Instance. It is token of already registered runner.
## ref: (we don't yet have docs for that, but we want to use existing token)
##
# runnerToken: ""
#
## Unregister all runners before termination
##
## Updating the runner's chart version or configuration will cause the runner container
## to be terminated and created again. This may cause your Gitlab instance to reference
## non-existant runners. Un-registering the runner before termination mitigates this issue.
## ref: https://docs.gitlab.com/runner/commands/README.html#gitlab-runner-unregister
##
# unregisterRunners: true

## When stopping the runner, give it time to wait for its jobs to terminate.
##
## Updating the runner's chart version or configuration will cause the runner container
## to be terminated with a graceful stop request. terminationGracePeriodSeconds
## instructs Kubernetes to wait long enough for the runner pod to terminate gracefully.
## ref: https://docs.gitlab.com/runner/commands/#signals
terminationGracePeriodSeconds: 3600

## Set the certsSecretName in order to pass custom certficates for GitLab Runner to use
## Provide resource name for a Kubernetes Secret Object in the same namespace,
## this is used to populate the /home/gitlab-runner/.gitlab-runner/certs/ directory
## ref: https://docs.gitlab.com/runner/configuration/tls-self-signed.html#supported-options-for-self-signed-certificates
##
# certsSecretName:

## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
concurrent: 10

## Defines in seconds how often to check GitLab for a new builds
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
checkInterval: 30

## Configure GitLab Runner's logging level. Available values are: debug, info, warn, error, fatal, panic
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
# logLevel:

## Configure GitLab Runner's logging format. Available values are: runner, text, json
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
# logFormat:

## Configure GitLab Runner's Sentry DSN.
## ref https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
# sentryDsn:

## A custom bash script that will be executed prior to the invocation
## gitlab-runner process
#
#preEntrypointScript: |
#  echo "hello"

## For RBAC support:
rbac:
  create: false
  ## Define specific rbac permissions.
  # resources: ["pods", "pods/exec", "secrets"]
  # verbs: ["get", "list", "watch", "create", "patch", "delete"]

  ## Run the gitlab-bastion container with the ability to deploy/manage containers of jobs
  ## cluster-wide or only within namespace
  clusterWideAccess: false

  ## Use the following Kubernetes Service Account name if RBAC is disabled in this Helm chart (see rbac.create)
  ##
  # serviceAccountName: default

  ## Specify annotations for Service Accounts, useful for annotations such as eks.amazonaws.com/role-arn
  ##
  ## ref: https://docs.aws.amazon.com/eks/latest/userguide/specify-service-account-role.html
  ##
  # serviceAccountAnnotations: {}

  ## Use podSecurity Policy
  ## ref: https://kubernetes.io/docs/concepts/policy/pod-security-policy/
  podSecurityPolicy:
    enabled: false
    resourceNames:
    - gitlab-runner

  ## Specify one or more imagePullSecrets used for pulling the runner image
  ##
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account
  ##
  # imagePullSecrets: []

## Configure integrated Prometheus metrics exporter
## ref: https://docs.gitlab.com/runner/monitoring/#configuration-of-the-metrics-http-server
metrics:
  enabled: true

## Configuration for the Pods that that the runner launches for each new job
##
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:18.04"
        privileged = false
        cpu_request = "100m"
        memory_request = "128Mi"
        service_cpu_request = "100m"
        service_memory_request = "128Mi"
        helper_cpu_request = "100m"
        helper_memory_request = "128Mi"


  ## Default container image to use for builds when none is specified
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # image: ubuntu:16.04

  ## Specify one or more imagePullSecrets
  ##
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # imagePullSecrets: []

  ## Specify the image pull policy: never, if-not-present, always. The cluster default will be used if not set.
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # imagePullPolicy: ""

  ## Defines number of concurrent requests for new job from GitLab
  ## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runners-section
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # requestConcurrency: 1

  ## Specify whether the runner should be locked to a specific project: true, false. Defaults to true.
  ##
  # locked: true

  ## Specify the tags associated with the runner. Comma-separated list of tags.
  ##
  ## ref: https://docs.gitlab.com/ce/ci/runners/#use-tags-to-limit-the-number-of-jobs-using-the-runner
  ##
  # tags: ""

  ## Specify if jobs without tags should be run.
  ## If not specified, Runner will default to true if no tags were specified. In other case it will
  ## default to false.
  ##
  ## ref: https://docs.gitlab.com/ce/ci/runners/#runner-is-allowed-to-run-untagged-jobs
  ##
  # runUntagged: true

  ## Specify whether the runner should only run protected branches.
  ## Defaults to False.
  ##
  ## ref: https://docs.gitlab.com/ee/ci/runners/#prevent-runners-from-revealing-sensitive-information
  ##
  # protected: true

  ## Run all containers with the privileged flag enabled
  ## This will allow the docker:dind image to run if you need to run Docker
  ## commands. Please read the docs before turning this on:
  ## ref: https://docs.gitlab.com/runner/executors/kubernetes.html#using-dockerdind
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # privileged: false

  ## The name of the secret containing runner-token and runner-registration-token
  # secret: gitlab-runner

  ## Namespace to run Kubernetes jobs in (defaults to the same namespace of this release)
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # namespace:

  ## The amount of time, in seconds, that needs to pass before the runner will
  ## timeout attempting to connect to the container it has just created.
  ## ref: https://docs.gitlab.com/runner/executors/kubernetes.html
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # pollTimeout: 180

  ## Set maximum build log size in kilobytes, by default set to 4096 (4MB)
  ## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runners-section
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # outputLimit: 4096

  ## Distributed runners caching
  ## ref: https://docs.gitlab.com/runner/configuration/autoscale.html#distributed-runners-caching
  ##
  ## If you want to use s3 based distributing caching:
  ## First of all you need to uncomment General settings and S3 settings sections.
  ##
  ## Create a secret 's3access' containing 'accesskey' & 'secretkey'
  ## ref: https://aws.amazon.com/blogs/security/wheres-my-secret-access-key/
  ##
  ## $ kubectl create secret generic s3access \
  ##   --from-literal=accesskey="YourAccessKey" \
  ##   --from-literal=secretkey="YourSecretKey"
  ## ref: https://kubernetes.io/docs/concepts/configuration/secret/
  ##
  ## If you want to use gcs based distributing caching:
  ## First of all you need to uncomment General settings and GCS settings sections.
  ##
  ## Access using credentials file:
  ## Create a secret 'google-application-credentials' containing your application credentials file.
  ## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runnerscachegcs-section
  ## You could configure
  ## $ kubectl create secret generic google-application-credentials \
  ##   --from-file=gcs-application-credentials-file=./path-to-your-google-application-credentials-file.json
  ## ref: https://kubernetes.io/docs/concepts/configuration/secret/
  ##
  ## Access using access-id and private-key:
  ## Create a secret 'gcsaccess' containing 'gcs-access-id' & 'gcs-private-key'.
  ## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runnerscachegcs-section
  ## You could configure
  ## $ kubectl create secret generic gcsaccess \
  ##   --from-literal=gcs-access-id="YourAccessID" \
  ##   --from-literal=gcs-private-key="YourPrivateKey"
  ## ref: https://kubernetes.io/docs/concepts/configuration/secret/
  cache: {}
    ## General settings
    ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
    # cacheType: s3
    # cachePath: "gitlab_runner"
    # cacheShared: true

    ## S3 settings
    ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
    # s3ServerAddress: s3.amazonaws.com
    # s3BucketName:
    # s3BucketLocation:
    # s3CacheInsecure: false

    ## GCS settings
    ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
    # gcsBucketName:

    ## S3 the name of the secret.
    # secretName: s3access
    ## Use this line for access using access-id and private-key
    # secretName: gcsaccess
    ## Use this line for access using google-application-credentials file
    # secretName: google-application-credentials


  ## Build Container specific configuration
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  builds: {}
    # cpuLimit: 200m
    # cpuLimitOverwriteMaxAllowed: 400m
    # memoryLimit: 256Mi
    # memoryLimitOverwriteMaxAllowed: 512Mi
    # cpuRequests: 100m
    # cpuRequestsOverwriteMaxAllowed: 200m
    # memoryRequests: 128Mi
    # memoryRequestsOverwriteMaxAllowed: 256Mi

  ## Service Container specific configuration
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  services: {}
    # cpuLimit: 200m
    # memoryLimit: 256Mi
    # cpuRequests: 100m
    # memoryRequests: 128Mi

  ## Helper Container specific configuration
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  helpers: {}
    # cpuLimit: 200m
    # memoryLimit: 256Mi
    # cpuRequests: 100m
    # memoryRequests: 128Mi
    # image: "gitlab/gitlab-runner-helper:x86_64-${CI_RUNNER_REVISION}"

  ## Helper container security context configuration
  ## Refer to https://docs.gitlab.com/runner/executors/kubernetes.html#using-security-context
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # pod_security_context:
  #   run_as_non_root: true
  #   run_as_user: 100
  #   run_as_group: 100
  #   fs_group: 65533
  #   supplemental_groups: [101, 102]

  ## Service Account to be used for runners
  ##
  # serviceAccountName:

  ## If Gitlab is not reachable through $CI_SERVER_URL
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # cloneUrl:

  ## Specify node labels for CI job pods assignment
  ## ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # nodeSelector: {}

  ## Specify node tolerations for CI job pods assignment
  ## ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # nodeTolerations: {}

  ## Specify pod labels for CI job pods
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # podLabels: {}

  ## Specify annotations for job pods, useful for annotations such as iam.amazonaws.com/role
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # podAnnotations: {}

  ## Configure environment variables that will be injected to the pods that are created while
  ## the build is running. These variables are passed as parameters, i.e. `--env "NAME=VALUE"`,
  ## to `gitlab-runner register` command.
  ##
  ## Note that `envVars` (see below) are only present in the runner pod, not the pods that are
  ## created for each build.
  ##
  ## ref: https://docs.gitlab.com/runner/commands/#gitlab-runner-register
  ##
  ## DEPRECATED: See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222
  # env:
  #   NAME: VALUE


## Configure securitycontext
## ref: http://kubernetes.io/docs/user-guide/security-context/
##
securityContext:
  fsGroup: 65533
  runAsUser: 100
  ## Note: values for the ubuntu image:
  # fsGroup: 999
  # runAsUser: 999

## Configure resource requests and limits
## ref: http://kubernetes.io/docs/user-guide/compute-resources/
##
resources: {}
  # limits:
  #   memory: 256Mi
  #   cpu: 200m
  # requests:
  #   memory: 128Mi
  #   cpu: 100m

## Affinity for pod assignment
## Ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity
##
affinity: {}

## Node labels for pod assignment
## Ref: https://kubernetes.io/docs/user-guide/node-selection/
##
nodeSelector: {}
  # Example: The gitlab runner manager should not run on spot instances so you can assign
  # them to the regular worker nodes only.
  # node-role.kubernetes.io/worker: "true"

## List of node taints to tolerate (requires Kubernetes >= 1.6)
## Ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
##
tolerations: []
  # Example: Regular worker nodes may have a taint, thus you need to tolerate the taint
  # when you assign the gitlab runner manager with nodeSelector or affinity to the nodes.
  # - key: "node-role.kubernetes.io/worker"
  #   operator: "Exists"

## Configure environment variables that will be present when the registration command runs
## This provides further control over the registration process and the config.toml file
## ref: `gitlab-runner register --help`
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html
##
# envVars:
#   - name: RUNNER_EXECUTOR
#     value: kubernetes

## list of hosts and IPs that will be injected into the pod's hosts file
hostAliases: []
  # Example:
  # - ip: "127.0.0.1"
  #   hostnames:
  #   - "foo.local"
  #   - "bar.local"
  # - ip: "10.1.2.3"
  #   hostnames:
  #   - "foo.remote"
  #   - "bar.remote"

## Annotations to be added to manager pod
##
podAnnotations: {}
  # Example:
  # iam.amazonaws.com/role: <my_role_arn>

## Labels to be added to manager pod
##
podLabels: {}
  # Example:
  # owner.team: <my_cool_team>

## HPA support for custom metrics:
## This section enables runners to autoscale based on defined custom metrics.
## In order to use this functionality, Need to enable a custom metrics API server by
## implementing "custom.metrics.k8s.io" using supported third party adapter
## Example: https://github.com/directxman12/k8s-prometheus-adapter
##
#hpa: {}
  # minReplicas: 1
  # maxReplicas: 10
  # metrics:
  # - type: Pods
  #   pods:
  #     metricName: gitlab_runner_jobs
  #     targetAverageValue: 400m
```

### Using configuration template

> [Introduced](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/106) [configuration template](https://docs.gitlab.com/runner/register/#runners-configuration-template-file) in Helm Chart 0.23.0. See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222.

It's now possible to use a [configuration template file](../register/index.md#runners-configuration-template-file)
to configure the runner. The configuration template allows users to configure any field on the Runner,
without having the Helm chart be aware of specific runner configuration options.

Here's a snippet of the default settings [found in the `values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/master/values.yaml) in the chart repository:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:16.04"
```

### Migrating to the new configuration template

> [Introduced](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/106) [configuration template](https://docs.gitlab.com/runner/register/#runners-configuration-template-file) in Helm Chart 0.23.0. See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/222.

Since many of the fields accepted by the `values.yaml` file will be removed with the introduction of version `1.0` of the
Helm Chart, migrating away from them in a timely manner is recommended.

All the configuration options supported by the Kubernetes executor are listed in [the Kubernetes executor docs](../executors/kubernetes.md#the-keywords).
For many of the fields the old naming in `values.yaml` is the same as [the keyword](../executors/kubernetes.md#the-keywords).
For some, a bit of renaming will be needed. As an example, if you have been using `helper CPU limits` before:

```yaml
helpers: {}
    cpuLimit: 200m
``` 

Now, they can be set as `helper_cpu_limit`:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:16.04"
        helper_cpu_limit: "200m"

## helpers: {}
##    cpuLimit: 200m
```

NOTE: **Note:**
Make sure to comment or remove the old configuration values from your `values.yaml` file
to avoid conflicts.

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

### Using an image from a private registry

Using an image from a private registry requires the configuration of imagePullSecrets. For more details on how to create imagePullSecrets [see the documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/).

```yaml
runners:
  ## Specify one or more imagePullSecrets
  ##
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
  ##
  imagePullSecrets:
  - [your-image-pull-secret]
```

Take note of the format. The value is not prefixed by a 'name' tag as is the convention in Kubernetes resources.

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

### Switching to the Ubuntu-based `gitlab-runner` Docker image

By default GitLab Runner's Helm Chart uses the Alpine version of the `gitlab/gitlab-runner` image,
which uses `musl libc`. In some cases, you may want to switch to the Ubuntu-based image, which uses `glibc`.

To do so, update your `values.yaml` file with the following values:

```yaml
# Specify the Ubuntu image. Remember to set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v13.0.0

# Update the security context values to the user ID in the ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

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
