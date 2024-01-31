---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Kubernetes executor **(FREE ALL)**

Use the Kubernetes executor to use Kubernetes clusters for your builds. The executor calls the Kubernetes
cluster API and creates a pod for each GitLab CI job.

## Workflow

The Kubernetes executor divides the build into multiple steps:

1. **Prepare**: Create the Pod against the Kubernetes Cluster.
   This creates the containers required for the build and services to run.
1. **Pre-build**: Clone, restore cache, and download artifacts from previous
   stages. This step runs on a special container as part of the pod.
1. **Build**: User build.
1. **Post-build**: Create cache, upload artifacts to GitLab. This step also uses
   the special container as part of the pod.

### How the runner creates Kubernetes pods

The following diagram shows the interaction between a GitLab instance and a runner hosted on a Kubernetes cluster. The runner calls the Kubernetes API to create pods on the cluster.

The pod consists of the following containers for each `service` defined in the `.gitlab-ci.yml` or `config.toml` files:

- A build container defined as `build`.
- A helper container defined as `helper`.
- A services containers defined as `svc-X`, where `X` is `[0-9]+`.

Services and containers run in the same Kubernetes
pod and share the same localhost address. The following restrictions apply:

- In GitLab Runner 12.8 and Kubernetes 1.7 and later, the services are accessible through their DNS names. If you
use an older version, you must use `localhost`.
- You cannot use several services that use the same port. For example, you cannot have two
  `mysql` services at the same time.

```mermaid
sequenceDiagram
    participant G as GitLab instance
    participant R as Runner on Kubernetes cluster
    participant Kube as Kubernetes API
    participant P as POD
    R->>+G: Get a CI job.
        loop
        G-->R: ;
    end
    Note over R,G: POST /api/v4/jobs/request
    G->>+R: CI job data.
    R-->>-Kube: Create a POD to run the CI job.
    Note over R,Kube: POST to Kube API
    P->>+P: Execute job.
    Note over P: CI build job = Prepare + Pre-build + Build + Post-build
    P->>+G: Job logs
```

The interaction in the diagram is valid for any Kubernetes cluster. For example, turnkey
solutions hosted on the major public cloud providers, or self-managed Kubernetes installations.

## Connect to the Kubernetes API

Use the following options to connect to the Kubernetes API. The user account provided must have
permission to create, list, and attach to Pods in the specified namespace.

| Option      | Description                                                                         |
|-------------|-------------------------------------------------------------------------------------|
| `host`      | Optional Kubernetes apiserver host URL (auto-discovery attempted if not specified). |
| `cert_file` | Optional Kubernetes apiserver user auth certificate.                                |
| `key_file`  | Optional Kubernetes apiserver user auth private key.                                |
| `ca_file`   | Optional Kubernetes apiserver ca certificate.                                       |

If you're running GitLab Runner in the Kubernetes cluster, you should omit
all of these fields so that the GitLab Runner auto-discovers the Kubernetes API.

If you're running GitLab Runner externally to the Cluster, then you must set each
of these settings and ensure that GitLab Runner has access to the Kubernetes API
on the cluster.

## Configuration settings

Use the following settings in the `config.toml` file to configure the Kubernetes executor.

### CPU requests and limits

| Setting | Description |
|---------|-------------|
| `cpu_limit` | The CPU allocation given to build containers. |
| `cpu_limit_overwrite_max_allowed` | The maximum amount that the CPU allocation can be written to for build containers. When empty, it disables the cpu limit overwrite feature. |
| `cpu_request` | The CPU allocation requested for build containers. |
| `cpu_request_overwrite_max_allowed` | The maximum amount that the CPU allocation request can be written to for build containers. When empty, it disables the cpu request overwrite feature. |
| `helper_cpu_limit` | The CPU allocation given to build helper containers. |
| `helper_cpu_limit_overwrite_max_allowed` | The maximum amount that the CPU allocation can be written to for helper containers. When empty, it disables the cpu limit overwrite feature. |
| `helper_cpu_request` | The CPU allocation requested for build helper containers. |
| `helper_cpu_request_overwrite_max_allowed` | The maximum amount that the CPU allocation request can be written to for helper containers. When empty, it disables the cpu request overwrite feature. |
| `service_cpu_limit` | The CPU allocation given to build service containers. |
| `service_cpu_limit_overwrite_max_allowed` | The maximum amount that the CPU allocation can be written to for service containers. When empty, it disables the cpu limit overwrite feature. |
| `service_cpu_request` | The CPU allocation requested for build service containers. |
| `service_cpu_request_overwrite_max_allowed` | The maximum amount that the CPU allocation request can be written to for service containers. When empty, it disables the cpu request overwrite feature. |

### Memory requests and limits

| Setting | Description |
|---------|-------------|
| `memory_limit` | The amount of memory allocated to build containers. |
| `memory_limit_overwrite_max_allowed` | The maximum amount that the memory allocation can be written to for build containers. When empty, it disables the memory limit overwrite feature. |
| `memory_request` | The amount of memory requested from build containers. |
| `memory_request_overwrite_max_allowed` | The maximum amount that the memory allocation request can be written to for build containers. When empty, it disables the memory request overwrite feature. |
| `helper_memory_limit` | The amount of memory allocated to build helper containers. |
| `helper_memory_limit_overwrite_max_allowed` | The maximum amount that the memory allocation can be written to for helper containers. When empty, it disables the memory limit overwrite feature. |
| `helper_memory_request` | The amount of memory requested for build helper containers. |
| `helper_memory_request_overwrite_max_allowed` | The maximum amount that the memory allocation request can be written to for helper containers. When empty, it disables the memory request overwrite feature. |
| `service_memory_limit` | The amount of memory allocated to build service containers. |
| `service_memory_limit_overwrite_max_allowed` | The maximum amount that the memory allocation can be written to for service containers. When empty, it disables the memory limit overwrite feature. |
| `service_memory_request` | The amount of memory requested for build service containers. |
| `service_memory_request_overwrite_max_allowed` | The maximum amount that the memory allocation request can be written to for service containers. When empty, it disables the memory request overwrite feature. |

### Storage requests and limits

| Setting | Description |
|---------|-------------|
| `ephemeral_storage_limit` | The ephemeral storage limit for build containers. |
| `ephemeral_storage_limit_overwrite_max_allowed` | The maximum amount that the ephemeral storage limit for build containers can be overwritten. When empty, it disables the ephemeral storage limit overwrite feature. |
| `ephemeral_storage_request` | The ephemeral storage request given to build containers. |
| `ephemeral_storage_request_overwrite_max_allowed` | The maximum amount that the ephemeral storage request can be overwritten by for build containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `helper_ephemeral_storage_limit` | The ephemeral storage limit given to helper containers. |
| `helper_ephemeral_storage_limit_overwrite_max_allowed` | The maximum amount that the ephemeral storage limit can be overwritten by for helper containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `helper_ephemeral_storage_request` | The ephemeral storage request given to helper containers. |
| `helper_ephemeral_storage_request_overwrite_max_allowed` | The maximum amount that the ephemeral storage request can be overwritten by for helper containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `service_ephemeral_storage_limit` | The ephemeral storage limit given to service containers. |
| `service_ephemeral_storage_limit_overwrite_max_allowed` | The maximum amount that the ephemeral storage limit can be overwritten by for service containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `service_ephemeral_storage_request` | The ephemeral storage request given to service containers. |
| `service_ephemeral_storage_request_overwrite_max_allowed` | The maximum amount that the ephemeral storage request can be overwritten by for service containers. When empty, it disables the ephemeral storage request overwrite feature. |

### Other `config.toml` settings

| Setting | Description |
|---------|-------------|
| `affinity` | Specify affinity rules that determine which node runs the build. Read more about [using affinity](#define-a-list-of-node-affinities). |
| `allow_privilege_escalation` | Run all containers with the `allowPrivilegeEscalation` flag enabled. When empty, it does not define the `allowPrivilegeEscalation` flag in the container `SecurityContext` and allows Kubernetes to use the default [privilege escalation](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) behavior. |
| `allowed_images` | Wildcard list of images that can be specified in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`). [View details](#restrict-docker-images-and-services). |
| `allowed_pull_policies` | List of pull policies that can be specified in the `.gitlab-ci.yml` file or the `config.toml` file. |
| `allowed_services` | Wildcard list of services that can be specified in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`). [View details](#restrict-docker-images-and-services). |
| `bearer_token` | Default bearer token used to launch build pods. |
| `bearer_token_overwrite_allowed` | Boolean to allow projects to specify a bearer token that will be used to create the build pod. |
| `build_container_security_context` | Sets a container security context for the build container. [Read more about security context](#set-a-security-policy-for-the-pod). |
| `cap_add` | Specify Linux capabilities that should be added to the job pod containers. [Read more about capabilities configuration in Kubernetes executor](#specify-container-capabilities). |
| `cap_drop` | Specify Linux capabilities that should be dropped from the job pod containers. [Read more about capabilities configuration in Kubernetes executor](#specify-container-capabilities). |
| `cleanup_grace_period_seconds` | When a job completes, the duration in seconds that the pod has to terminate gracefully. After this period, the processes are forcibly halted with a kill signal. Ignored if `terminationGracePeriodSeconds` is specified. |
| `dns_policy` | Specify the DNS policy that should be used when constructing the pod: `none`, `default`, `cluster-first`, `cluster-first-with-host-net`. The Kubernetes default (`cluster-first`) is used if not set. |
| `dns_config` | Specify the DNS configuration that should be used when constructing the pod. [Read more about using pod's DNS config](#configure-pod-dns-settings). |
| `helper_container_security_context` | Sets a container security context for the helper container. [Read more about security context](#set-a-security-policy-for-the-pod). |
| `helper_image` | (Advanced) [Override the default helper image](../configuration/advanced-configuration.md#helper-image) used to clone repos and upload artifacts. |
| `helper_image_flavor` | Sets the helper image flavor (`alpine`, `alpine3.16`, `alpine3.17`, `alpine3.18`, `alpine3.19`, or `ubuntu`). Defaults to `alpine`. Using `alpine` is the same as `alpine3.18`. |
| `host_aliases` | List of additional host name aliases that will be added to all containers. [Read more about using extra host aliases](#add-extra-host-aliases). |
| `image_pull_secrets` | An array of items containing the Kubernetes `docker-registry` secret names used to authenticate Docker image pulling from private registries. |
| `init_permissions_container_security_context` | Sets a container security context for the init-permissions container. [Read more about security context](#set-a-security-policy-for-the-pod). |
| `namespace` | Namespace in which to run Kubernetes Pods. |
| `namespace_overwrite_allowed` | Regular expression to validate the contents of the namespace overwrite environment variable (documented below). When empty, it disables the namespace overwrite feature. |
| `node_selector` | A `table` of `key=value` pairs in the format of `string=string` (`string:string` in the case of environment variables). Setting this limits the creation of pods to Kubernetes nodes matching all the `key=value` pairs. [Read more about using node selectors](#specify-the-node-to-execute-builds). |
| `node_tolerations` | A `table` of `"key=value" = "Effect"` pairs in the format of `string=string:string`. Setting this allows pods to schedule to nodes with all or a subset of tolerated taints. Only one toleration can be supplied through environment variable configuration. The `key`, `value`, and `effect` match with the corresponding field names in Kubernetes pod toleration configuration. |
| `pod_annotations` | A `table` of `key=value` pairs in the format of `string=string`. This is the list of annotations to be added to each build pod created by the Runner. The value of these can include environment variables for expansion. Pod annotations can be overwritten in each build. |
| `pod_annotations_overwrite_allowed` | Regular expression to validate the contents of the pod annotations overwrite environment variable. When empty, it disables the pod annotations overwrite feature. |
| `pod_labels` | A `table` of `key=value` pairs in the format of `string=string`. This is the list of labels to be added to each build pod created by the runner. The value of these can include environment variables for expansion. Pod labels can be overwritten in each build by using `pod_labels_overwrite_allowed`. |
| `pod_labels_overwrite_allowed` | Regular expression to validate the contents of the pod labels overwrite environment variable. When empty, it disables the pod labels overwrite feature. |
| `pod_security_context` | Configured through the configuration file, this sets a pod security context for the build pod. [Read more about security context](#set-a-security-policy-for-the-pod). |
| `pod_termination_grace_period_seconds` | Pod-level setting which determines the duration in seconds which the pod has to terminate gracefully. After this, the processes are forcibly halted with a kill signal. Ignored if `terminationGracePeriodSeconds` is specified. |
| `poll_interval` | How frequently, in seconds, the runner will poll the Kubernetes pod it has just created to check its status (default = 3). |
| `poll_timeout` | The amount of time, in seconds, that needs to pass before the runner will time out attempting to connect to the container it has just created. Useful for queueing more builds that the cluster can handle at a time (default = 180). |
| `cleanup_resources_timeout` | The total amount of time for Kubernetes resources to be cleaned up after the job completes. Supported syntax: `1h30m`, `300s`, `10m`. Default is 5 minutes (`5m`). |
| `priority_class_name` | Specify the Priority Class to be set to the pod. The default one is used if not set. |
| `privileged` | Run containers with the privileged flag. |
| `pull_policy` | Specify the image pull policy: `never`, `if-not-present`, `always`. If not set, the cluster's image [default pull policy](https://kubernetes.io/docs/concepts/containers/images/#updating-images) is used. For more information and instructions on how to set multiple pull policies, see [using pull policies](#set-a-pull-policy). See also [`if-not-present`, `never` security considerations](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy). You can also [restrict pull policies](#restrict-docker-pull-policies). |
| `resource_availability_check_max_attempts` | The maximum number of attempts to check if a resource (service account and/or pull secret) set is available before giving up. There is 5 seconds interval between each attempt. [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27664) in GitLab 15.0. [Read more about resources check during prepare step](#resources-check-during-prepare-step). |
| `runtime_class_name` | A Runtime class to use for all created pods. If the feature is unsupported by the cluster, jobs exit or fail. |
| `service_container_security_context` | Sets a container security context for the service containers. [Read more about security context](#set-a-security-policy-for-the-pod). |
| `scheduler_name` | Scheduler to use for scheduling build pods. |
| `service_account` | Default service account job/executor pods use to talk to Kubernetes API. |
| `service_account_overwrite_allowed` | Regular expression to validate the contents of the service account overwrite environment variable. When empty, it disables the service account overwrite feature. |
| `services` | [Since GitLab Runner 12.5](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4470), list of [services](https://docs.gitlab.com/ee/ci/services/) attached to the build container using the [sidecar pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/sidecar). Read more about [using services](#define-a-list-of-services). |
| `terminationGracePeriodSeconds` | Duration after the processes running in the pod are sent a termination signal and the time when the processes are forcibly halted with a kill signal. [Deprecated in favour of `cleanup_grace_period_seconds` and `pod_termination_grace_period_seconds`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28165). |
| `volumes` | Configured through the configuration file, the list of volumes that is mounted in the build container. [Read more about using volumes](#configure-volume-types). |
| `pod_spec` | This setting is an Experiment. Overwrites the pod specification generated by the runner manager with a list of configurations set on the pod used to run the CI Job. All the properties listed `Kubernetes Pod Specification` can be set. For more information, see [Overwrite generated pod specifications (Experiment)](#overwrite-generated-pod-specifications). |
| `try_limit` | The maximum number of attempts to communicate with Kubernetes API. The retry interval between each attempt is based on a backoff algorithm starting at 500 ms. |

### Overwrite generated pod specifications **(Beta)**

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3114) in GitLab Runner 15.10.

This feature is a [Beta](https://docs.gitlab.com/ee/policy/experiment-beta-support.html#beta). We strongly recommend that you use
this feature on a test Kubernetes cluster before you use it on a production cluster. To use this feature, you must
enable the `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` [feature flag](../configuration/feature-flags.md).

To add feedback for improvements before the feature is made generally available, use [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29659).

To modify the `PodSpec` generated by the runner manager, use the `pod_spec` setting in the `config.toml` file.

The `pod_spec` setting:

- Overwrites and completes fields for the generated pod specification.
- Overwrites configuration values that might have been set in your `config.toml` under `[runners.kubernetes]`.

You can configure multiple `pod_spec` settings.

| Setting | Description |
|---------|-------------|
| `name` | Name given to the custom `pod_spec`. |
| `patch_path` | Path to the file that defines the changes to apply to the final `PodSpec` object before it is generated. The file must be a JSON or YAML file. |
| `patch` | A JSON or YAML format string that describes the changes which must be applied to the final `PodSpec` object before it is generated. |
| `patch_type` | The strategy the runner uses to apply the specified changes to the `PodSpec` object generated by GitLab Runner. The accepted values are `merge`, `json`, and `strategic`. |

You cannot set the `patch_path` and `patch` in the same `pod_spec` configuration, otherwise an error occurs.

Example of multiple `pod_spec` configurations in the `config.toml`:

```toml
[[runners]]
  [runners.kubernetes]
    [[runners.kubernetes.pod_spec]]
      name = "hostname"
      patch = '''
        hostname: "custom-pod-hostname"
      '''
      patch_type = "merge"
    [[runners.kubernetes.pod_spec]]
      name = "subdomain"
      patch = '''
        subdomain: "subdomain"
      '''
      patch_type = "strategic"
    [[runners.kubernetes.pod_spec]]
      name = "terminationGracePeriodSeconds"
      patch = '''
        [{"op": "replace", "path": "/terminationGracePeriodSeconds", "value": 60}]
      '''
      patch_type = "json"
```

#### Merge patch strategy

The `merge` patch strategy applies [a key-value replacement](https://datatracker.ietf.org/doc/html/rfc7386) on the existing `PodSpec`.
If you use this strategy, the `pod_spec` configuration in the `config.toml` **overwrites** the values in the final `PodSpec`
object before it is generated. Because the values are completely overwritten, you should use this patch strategy with caution.

Example of a `pod_spec` configuration with the `merge` patch strategy:

```toml
concurrent = 1
check_interval = 1
log_level = "debug"
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = ""
  url = "https://gitlab.example.com"
  id = 0
  token = "__REDACTED__"
  token_obtained_at = 0001-01-01T00:00:00Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "kubernetes"
  shell = "bash"
  environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true", "CUSTOM_VAR=value"]
  [runners.kubernetes]
    image = "alpine"
    ...
    [[runners.kubernetes.pod_spec]]
      name = "build envvars"
      patch = '''
        containers:
        - env:
          - name: env1
            value: "value1"
          - name: env2
            value: "value2"
          name: build
      '''
      patch_type = "merge"
```

With this configuration, the final `PodSpec` has only one container called `build` with two environment variables `env1` and `env2`. The example above make the related CI Job failed as:

- The `helper` container specification is removed.
- The `build` container specification lost all necessary configuration set by GitLab Runner.

To prevent the job from failing, in this example, the `pod_spec` must contain the untouched properties generated by GitLab Runner.

#### JSON patch strategy

The `json` patch strategy uses the [JSON Patch specification](https://datatracker.ietf.org/doc/html/rfc6902)
to give control over the `PodSpec` objects and arrays to update. You cannot use this strategy on `array` properties.

Example of a `pod_spec` configuration with the `json` patch strategy. In this configuration,
a new `key: value pair` is added to the existing `nodeSelector`. The existing values are not overwritten.

```toml
concurrent = 1
check_interval = 1
log_level = "debug"
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = ""
  url = "https://gitlab.example.com"
  id = 0
  token = "__REDACTED__"
  token_obtained_at = 0001-01-01T00:00:00Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "kubernetes"
  shell = "bash"
  environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true", "CUSTOM_VAR=value"]
  [runners.kubernetes]
    image = "alpine"
    ...
    [[runners.kubernetes.pod_spec]]
      name = "val1 node"
      patch = '''
        { "op": "add", "path": "/nodeSelector", "value": { key1: "val1" } }
      '''
      patch_type = "json"
```

#### Strategic patch strategy

This `strategic` patch strategy uses the existing `patchStrategy` applied to each field of the `PodSpec` object.

Example of a `pod_spec` configuration with the `strategic` patch strategy. In this configuration,
a `resource request` is set to on the build container.

```toml
concurrent = 1
check_interval = 1
log_level = "debug"
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = ""
  url = "https://gitlab.example.com"
  id = 0
  token = "__REDACTED__"
  token_obtained_at = 0001-01-01T00:00:00Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "kubernetes"
  shell = "bash"
  environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true", "CUSTOM_VAR=value"]
  [runners.kubernetes]
    image = "alpine"
    ...
    [[runners.kubernetes.pod_spec]]
      name = "cpu request 500m"
      patch = '''
        containers:
        - name: build
          resources:
            requests:
              cpu: "500m"
      '''
      patch_type = "strategic"
```

With this configuration, a `resource request` is set to on the build container.

#### Best practices

- Test the added `pod_spec` in a test environment before deployment in a production environment.
- Make sure that the `pod_spec` configuration does not negatively impact the GitLab Runner generated specification.
- Do not use the `merge` patch strategy for complex pod specification updates.
- Where possible, use the `config.toml` when the configuration is available. For example, the following configuration replaces the first environment variables set by GitLab Runner by the one set in the custom `pod_spec` instead of adding the environment variable set to the existing list.

```toml
concurrent = 1
check_interval = 1
log_level = "debug"
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = ""
  url = "https://gitlab.example.com"
  id = 0
  token = "__REDACTED__"
  token_obtained_at = 0001-01-01T00:00:00Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "kubernetes"
  shell = "bash"
  environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true", "CUSTOM_VAR=value"]
  [runners.kubernetes]
    image = "alpine"
    ...
    [[runners.kubernetes.pod_spec]]
      name = "build envvars"
      patch = '''
        containers:
        - env:
          - name: env1
            value: "value1"
          name: build
      '''
      patch_type = "strategic"
```

#### Create a PVC for each build job by modifying the Pod Spec

To create a [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) for each build job make sure to check out how to enable
the [Pod Spec functionality](#overwrite-generated-pod-specifications).

Kubernetes allows to create an ephemeral [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) attached to the lifecycle of a Pod.
This will work if [dynamic provisioning](https://kubernetes.io/docs/concepts/storage/dynamic-provisioning/) is enabled on your Kubernetes cluster allowing
each `PVC` to request a new [Volume](https://kubernetes.io/docs/concepts/storage/volumes/), the volume too will be tied to the lifetime of the Pod.

After [dynamic provisioning](https://kubernetes.io/docs/concepts/storage/dynamic-provisioning/) is enabled, the `config.toml` can be modified as follows to create an
ephemeral `PVC`:

```toml
[[runners.kubernetes.pod_spec]]
  name = "ephemeral-pvc"
  patch = '''
    containers:
    - name: build
      volumeMounts:
      - name: builds
        mountPath: /builds
    - name: helper
      volumeMounts:
      - name: builds
        mountPath: /builds
    volumes:
    - name: builds
      ephemeral:
        volumeClaimTemplate:
          spec:
            storageClassName: <The Storage Class that will dynamically provision a Volume>
            accessModes: [ ReadWriteOnce ]
            resources:
              requests:
                storage: 1Gi
  '''
```

### Pod lifecycle

A [pod's lifecycle](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle)
can be affected by:

- Setting the `pod_termination_grace_period_seconds` property in the `TOML` configuration file.
  The process running on the pod can run for the given duration after the `TERM` signal is sent.
  A kill signal is sent if the Pod is not successfully terminated after this period of time.
- Enabling the [`FF_USE_POD_ACTIVE_DEADLINE_SECONDS` feature flag](../configuration/feature-flags.md).
  When enabled and the job times out, the pod running the CI/CD job is marked as
  failed and all associated containers are killed. To have the job time out on GitLab first,
  `activeDeadlineSeconds` is set to `configured timeout + 1 second`.

NOTE:
If both the `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` feature flag is enabled and the
`pod_termination_grace_period_seconds` is set with a non zero value, the CI job pod is not
immediately terminated when the job times out. The pod `terminationGracePeriods`
ensures the pod is terminated only when it expired.

### Default Annotations for job Pods

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3845) in GitLab Runner 15.9.

The following annotations are added by default on the Pod running the jobs:

| Key                                | Description                                                       |
|------------------------------------|-------------------------------------------------------------------|
| `job.runner.gitlab.com/id`         | The ID of the job, unique across all jobs in the GitLab instance. |
| `job.runner.gitlab.com/url`        | The URL for the job details.                                      |
| `job.runner.gitlab.com/sha`        | The commit revision the project is built for.                     |
| `job.runner.gitlab.com/before_sha` | The previous latest commit present on a branch or tag.            |
| `job.runner.gitlab.com/ref`        | The branch or tag name for which the project is built.            |
| `job.runner.gitlab.com/name`       | The name of the job.                                              |
| `project.runner.gitlab.com/id`     | The project ID of the job.                                        |

To overwrite default annotations, use the `pod_annotations` in the GitLab Runner configuration.
You can also overwrite annotations for each CI/CD job in the [`.gitlab-ci.yml` file](#overwrite-pod-annotations).

### Configure the executor service account

To configure the executor service account, you can set the `KUBERNETES_SERVICE_ACCOUNT` environment variable or use the `--kubernetes-service-account` flag.

### Overwrite the Kubernetes namespace

Prerequisites:

- In the `values.yml` file for GitLab Runner Helm charts, `rbac.clusterWideAccess` is set to `true`.
- The runner has [permissions](#configure-runner-api-permissions) configured in the core API group.

You can overwrite Kubernetes namespaces to designate a namespace for CI purposes, and deploy a custom
set of pods to it. The pods spawned by the runner are in the overwritten namespace to
enable access between containers during the CI stages.

To overwrite the Kubernetes namespace for each CI/CD job, set the `KUBERNETES_NAMESPACE_OVERWRITE`
variable in the `.gitlab-ci.yml` file.

``` yaml
variables:
  KUBERNETES_NAMESPACE_OVERWRITE: ci-${CI_COMMIT_REF_SLUG}
```

NOTE:
This variable does not create a namespace on your cluster. Ensure that the namespace exists before you run the job.

To use only designated namespaces during CI runs, in the `config.toml` file, define a regular expression for `namespace_overwrite_allowed`:

```toml
[runners.kubernetes]
    ...
    namespace_overwrite_allowed = "ci-.*"
```

### Configure runner API permissions

To configure permissions for the core API group, update the `values.yml` file for GitLab Runner Helm charts.

You can either:

- Set `rbac.create` to `true`.
- Specify a service account `rbac.serviceAccountName: <service_account_name>` with the following
  permissions in the `values.yml` file.

 <!-- k8s_api_permissions_list_start -->

| Resource | Verb (Optional Feature Flags) |
|----------|-------------------------------|
| events | list (`FF_RETRIEVE_POD_WARNING_EVENTS=true`), watch (`FF_PRINT_POD_EVENTS=true`) |
| pods | attach (`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false`), create, delete, exec, get |
| pods/logs | get (`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false`), list (`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false`) |
| secrets | create, delete, get, update |
| serviceAccounts | get |
| services | create, get |

<!-- k8s_api_permissions_list_end -->

- _The `serviceAccount` permission is needed only:_

  - _For GitLab 15.0 and 15.1._
  - _For GitLab 15.0.1, 15.1.1, and 15.2 when `resource_availability_check_max_attempts` is set to a value higher than 0._

- _As of GitLab Runner 15.8 the `configmaps` permission is no longer needed._

- _The `event` permission is needed only:_

  - _For GitLab 16.2.1 and later._

### Overwrite the Kubernetes default service account

To overwrite the Kubernetes service account for each CI/CD job in the `.gitlab-ci.yml` file,
set the variable `KUBERNETES_SERVICE_ACCOUNT_OVERWRITE`.

You can use this variable to specify a service account attached to the namespace, which you may need
for complex RBAC configurations.

``` yaml
variables:
  KUBERNETES_SERVICE_ACCOUNT_OVERWRITE: ci-service-account
```

To ensure only designated service accounts are used during CI runs, define a regular expression
for either:

- The `service_account_overwrite_allowed` setting.
- The `KUBERNETES_SERVICE_ACCOUNT_OVERWRITE_ALLOWED` environment variable.

If you don't set either, the overwrite is disabled.

### Set the bearer token for Kubernetes API calls

To set the bearer token for API calls to create pods, use the `KUBERNETES_BEARER_TOKEN`
variable. This allows project owners to use project secret variables to specify a bearer token.

When specifying the bearer token, you must
set the `Host` configuration setting.

``` yaml
variables:
  KUBERNETES_BEARER_TOKEN: thebearertokenfromanothernamespace
```

### Overwrite the node selector

To overwrite the node selector:

1. In the `config.toml` or Helm `values.yaml` file, enable overwriting of the node selector:

   ```toml
   runners:
    ...
    config: |
      [[runners]]
        [runners.kubernetes]
          node_selector_overwrite_allowed = ".*"
   ```

1. In the `.gitlab-ci.yml` file, define the variable to overwrite the node selector:

   ```yaml
   variables:
     KUBERNETES_NODE_SELECTOR_* = ''
   ```

In the following example, to overwrite the Kubernetes node architecture,
the settings are configured in the `config.toml` and `.gitlab-ci.yml`:

::Tabs

:::TabTitle `config.toml`

```toml
concurrent = 1
check_interval = 1
log_level = "debug"
shutdown_timeout = 0

listen_address = ':9252'

[session_server]
  session_timeout = 1800

[[runners]]
  name = ""
  url = "https://gitlab.com/"
  id = 0
  token = "__REDACTED__"
  token_obtained_at = "0001-01-01T00:00:00Z"
  token_expires_at = "0001-01-01T00:00:00Z"
  executor = "kubernetes"
  shell = "bash"
  [runners.kubernetes]
    host = ""
    bearer_token_overwrite_allowed = false
    image = "alpine"
    namespace = ""
    namespace_overwrite_allowed = ""
    pod_labels_overwrite_allowed = ""
    service_account_overwrite_allowed = ""
    pod_annotations_overwrite_allowed = ""
    node_selector_overwrite_allowed = "kubernetes.io/arch=.*" # <--- allows overwrite of the architecture
```

:::TabTitle `.gitlab-ci.yml`

```yaml
  job:
    image: IMAGE_NAME
    variables:
      KUBERNETES_NODE_SELECTOR_ARCH: 'kubernetes.io/arch=amd64' # <--- select the right architecture
```

::EndTabs

### Overwrite pod labels

To overwrite Kubernetes pod labels for each CI/CD job:

1. In the `.config.yaml` file, define a regular expression for `pod_labels_overwrite_allowed`.
1. In the `.gitlab-ci.yml` file, set the `KUBERNETES_POD_LABELS_*` variables with values of
   `key=value`. The pod labels are overwritten to the `key=value`. You can apply multiple values:

    ```yaml
    variables:
      KUBERNETES_POD_LABELS_1: "Key1=Val1"
      KUBERNETES_POD_LABELS_2: "Key2=Val2"
      KUBERNETES_POD_LABELS_3: "Key3=Val3"
    ```

### Overwrite pod annotations

To overwrite Kubernetes pod annotations for each CI/CD job:

1. In the `.config.yaml` file, define a regular expression for `pod_annotations_overwrite_allowed`.
1. In the `.gitlab-ci.yml` file, set the `KUBERNETES_POD_ANNOTATIONS_*` variables and use `key=value` for the value.
   Pod annotations are overwritten to the `key=value`. You can specify multiple annotations:

   ```yaml
   variables:
     KUBERNETES_POD_ANNOTATIONS_1: "Key1=Val1"
     KUBERNETES_POD_ANNOTATIONS_2: "Key2=Val2"
     KUBERNETES_POD_ANNOTATIONS_3: "Key3=Val3"
   ```

In the example below, the `pod_annotations` and the `pod_annotations_overwrite_allowed` are set.
This configuration allows overwrite of any of the `pod_annotations` configured in the `config.toml`.

```toml
[[runners]]
  # usual configuration
  executor = "kubernetes"
  [runners.kubernetes]
    image = "alpine"
    pod_annotations_overwrite_allowed = "*"
    [runners.kubernetes.pod_annotations]
      "Key1" = "Val1"
      "Key2" = "Val2"
      "Key3" = "Val3"
      "Key4" = "Val4"
```

### Overwrite container resources

You can overwrite Kubernetes CPU and memory allocations for each CI/CD
job. You can apply settings for requests and limits for the build, helper, and service containers.

To overwrite container resources, use the following variables in the `.gitlab-ci.yml` file.

The values for the variables are restricted to the [maximum overwrite](#configuration-settings)
setting for that resource. If the maximum overwrite has not been set for a resource, the variable is not used.

``` yaml
 variables:
   KUBERNETES_CPU_REQUEST: "3"
   KUBERNETES_CPU_LIMIT: "5"
   KUBERNETES_MEMORY_REQUEST: "2Gi"
   KUBERNETES_MEMORY_LIMIT: "4Gi"
   KUBERNETES_EPHEMERAL_STORAGE_REQUEST: "512Mi"
   KUBERNETES_EPHEMERAL_STORAGE_LIMIT: "1Gi"

   KUBERNETES_HELPER_CPU_REQUEST: "3"
   KUBERNETES_HELPER_CPU_LIMIT: "5"
   KUBERNETES_HELPER_MEMORY_REQUEST: "2Gi"
   KUBERNETES_HELPER_MEMORY_LIMIT: "4Gi"
   KUBERNETES_HELPER_EPHEMERAL_STORAGE_REQUEST: "512Mi"
   KUBERNETES_HELPER_EPHEMERAL_STORAGE_LIMIT: "1Gi"

   KUBERNETES_SERVICE_CPU_REQUEST: "3"
   KUBERNETES_SERVICE_CPU_LIMIT: "5"
   KUBERNETES_SERVICE_MEMORY_REQUEST: "2Gi"
   KUBERNETES_SERVICE_MEMORY_LIMIT: "4Gi"
   KUBERNETES_SERVICE_EPHEMERAL_STORAGE_REQUEST: "512Mi"
   KUBERNETES_SERVICE_EPHEMERAL_STORAGE_LIMIT: "1Gi"
```

### Configuration example

The following sample shows an example configuration of the `config.toml` file
for the Kubernetes executor.

```toml
concurrent = 4

[[runners]]
  name = "myRunner"
  url = "https://gitlab.com/ci"
  token = "......"
  executor = "kubernetes"
  [runners.kubernetes]
    host = "https://45.67.34.123:4892"
    cert_file = "/etc/ssl/kubernetes/api.crt"
    key_file = "/etc/ssl/kubernetes/api.key"
    ca_file = "/etc/ssl/kubernetes/ca.crt"
    namespace = "gitlab"
    namespace_overwrite_allowed = "ci-.*"
    bearer_token_overwrite_allowed = true
    privileged = true
    cpu_limit = "1"
    memory_limit = "1Gi"
    service_cpu_limit = "1"
    service_memory_limit = "1Gi"
    helper_cpu_limit = "500m"
    helper_memory_limit = "100Mi"
    poll_interval = 5
    poll_timeout = 3600
    dns_policy = "cluster-first"
    priority_class_name = "priority-1"
    [runners.kubernetes.node_selector]
      gitlab = "true"
    [runners.kubernetes.node_tolerations]
      "node-role.kubernetes.io/master" = "NoSchedule"
      "custom.toleration=value" = "NoSchedule"
      "empty.value=" = "PreferNoSchedule"
      "onlyKey" = ""
```

## Restrict access to job variables

When using Kubernetes executor, users with access to the Kubernetes cluster can read variables used in the job. By default, job variables are stored in:

- Pod's environment section

To restrict access to job variable data, you should use role-based access control (RBAC) so that only GitLab administrators have access to the namespace used by the GitLab Runner.

If you need other users to access the GitLab Runner namespace, set the following `verbs` to restrict the type of access users have in the GitLab Runner namespace:

- For `pods` and `configmaps`:
  - `get`
  - `watch`
  - `list`
- For `pods/exec` and `pods/attach`, use `create`.

Example RBAC definition for authorized users:

```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: gitlab-runner-authorized-users
rules:
- apiGroups: [""]
  resources: ["configmaps", "pods"]
  verbs: ["get", "watch", "list"]
- apiGroups: [""]
  resources: ["pods/exec", "pods/attach"]
  verbs: ["create"]
```

## Resources check during prepare step

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27664) in GitLab 15.0.
> - [Updated](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29101) in GitLab 15.2.

Prerequisites:

- `image_pull_secrets` or `service_account` is set.
- `resource_availability_check_max_attempts` is set to a number greater than zero.
- Kubernetes `serviceAccount` used with the `get` and `list` permissions.

GitLab Runner checks if the new service accounts or secrets are available with a 5-second interval between each try.

- In GitLab 15.0 and 15.1, you cannot disable this feature and it defaults to `5` when a negative value is set.
- In GitLab 15.0.1, 15.1.1, 15.2 and later, this feature is disabled by default. To enable this feature, set `resource_availability_check_max_attempts` to any value other than `0`.
The value you set defines the amount of times the runner checks for service accounts or secrets.

## Using the cache with the Kubernetes executor

When the cache is used with the Kubernetes executor, a volume called `/cache` is mounted on the pod. During job
execution, if cached data is needed, the runner checks if cached data is available. Cached data is available if
a compressed file is available on the cache volume.

To set the cache volume, use the [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section) setting in the `config.toml` file.

- If available, the compressed file is extracted into the build folder and can then be used in the job.
- If not available, the cached data is downloaded from the configured storage and saved into the `cache dir` as a compressed file.
  The compressed file is then extracted into the `build` folder.

## Configure volume types

You can mount the following volume types:

- `hostPath`
- `persistentVolumeClaim`
- `configMap`
- `secret`
- `emptyDir`
- `csi`

Example of a configuration with multiple volume types:

```toml
concurrent = 4

[[runners]]
  # usual configuration
  executor = "kubernetes"
  [runners.kubernetes]
    [[runners.kubernetes.volumes.host_path]]
      name = "hostpath-1"
      mount_path = "/path/to/mount/point"
      read_only = true
      host_path = "/path/on/host"
    [[runners.kubernetes.volumes.host_path]]
      name = "hostpath-2"
      mount_path = "/path/to/mount/point_2"
      read_only = true
    [[runners.kubernetes.volumes.pvc]]
      name = "pvc-1"
      mount_path = "/path/to/mount/point1"
    [[runners.kubernetes.volumes.config_map]]
      name = "config-map-1"
      mount_path = "/path/to/directory"
      [runners.kubernetes.volumes.config_map.items]
        "key_1" = "relative/path/to/key_1_file"
        "key_2" = "key_2"
    [[runners.kubernetes.volumes.secret]]
      name = "secrets"
      mount_path = "/path/to/directory1"
      read_only = true
      [runners.kubernetes.volumes.secret.items]
        "secret_1" = "relative/path/to/secret_1_file"
    [[runners.kubernetes.volumes.empty_dir]]
      name = "empty-dir"
      mount_path = "/path/to/empty_dir"
      medium = "Memory"
    [[runners.kubernetes.volumes.csi]]
      name = "csi-volume"
      mount_path = "/path/to/csi/volume"
      driver = "my-csi-driver"
      [runners.kubernetes.volumes.csi.volume_attributes]
        size = "2Gi"
```

### `hostPath` volume

Configure the [`hostPath` volume](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath) to instruct Kubernetes to mount
a specified host path in the container.

Use the following options in the `config.toml` file:

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes    | The name of the volume. |
| `mount_path` | string    | Yes    | Path inside of the container where the volume is mounted. |
| `sub_path`   | string    | No     | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) in the volume instead of the root. |
| `host_path`  | string    | No     | Host path to mount as the volume. If not specified, then this is set to the same path as `mount_path`. |
| `read_only`  | boolean   | No     | Sets the volume in read-only mode (defaults to false). |

### `persistentVolumeClaim` volume

Configure the [`persistentVolumeClaim` volume](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim) to
instruct Kubernetes to use a `persistentVolumeClaim` defined in a Kubernetes cluster and mount it in the container.

Use the following options in the `config.toml` file:

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | The name of the volume and at the same time the name of `PersistentVolumeClaim` that should be used. Supports variables. For more information, see [Persistent per-concurrency build volumes](#persistent-per-concurrency-build-volumes). |
| `mount_path` | string    | Yes      | Path in the container where the volume is mounted. |
| `read_only`  | boolean   | No       | Sets the volume to read-only mode (defaults to false). |
| `sub_path`   | string    | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) in the volume instead of the root. |

### `configMap` volume

Configure the `configMap` volume to instruct Kubernetes to use a [`configMap`](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/)
defined in a Kubernetes cluster and mount it in the container.

Use the following options in the `config.toml`:

| Option       | Type                | Required | Description                                                                                                               |
|--------------|---------------------|----------|---------------------------------------------------------------------------------------------------------------------------|
| `name`       | string              | Yes      | The name of the volume and at the same time the name of _configMap_ that should be used.                                  |
| `mount_path` | string              | Yes      | Path in the container where the volume is mounted.                                                                        |
| `read_only`  | boolean             | No       | Sets the volume to read-only mode (defaults to false).                                                                    |
| `sub_path`   | string              | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) in the volume instead of the root. |
| `items`      | `map[string]string` | no       | Key-to-path mapping for keys from the _configMap_ that should be used.                                                    |

Each key from the `configMap` is changed into a file and stored in the mount path. By default:

- All keys are included.
- The `configMap` key is used as the file name.
- The value is stored in the file contents.

To change the default key and value storage, use the `items` option . If you use the `items` option, **only specified keys**
are added to the volumes and all other keys are skipped.

NOTE:
If you use a key that doesn't exist, the job fails on the pod creation stage.

### `secret` volume

Configure a [`secret` volume](https://kubernetes.io/docs/concepts/storage/volumes/#secret) to instruct Kubernetes to use
a `secret` defined in a Kubernetes cluster and mount it in the container.

Use the following options in the `config.toml` file:

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | The name of the volume and at the same time the name of _secret_ that should be used. |
| `mount_path` | string    | Yes      | Path inside of container where the volume should be mounted. |
| `read_only`  | boolean   | No       | Sets the volume in read-only mode (defaults to false). |
| `sub_path`   | string    | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `items`      | `map[string]string` | No   | Key-to-path mapping for keys from the _configMap_ that should be used. |

Each key from selected `secret` is changed into a file stored in the selected mount path. By default:

- All keys are included.
- The `configMap` key is used as the file name.
- The value is stored in the file contents.

To change default key and value storage, use the `items` option. If you use the `items` option, **only specified keys**
are added to the volumes and all other keys are skipped.

NOTE:
If you use a key that doesn't exist, the job fails on the pod creation stage.

### `emptyDir` volume

Configure an [`emptyDir` volume](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)
to instruct Kubernetes to mount an empty directory in the container.

Use the following options in the `config.toml` file:

| Option       | Type    | Required | Description |
|--------------|---------|----------|-------------|
| `name`       | string  | Yes      | The name of the volume. |
| `mount_path` | string  | Yes      | Path inside of container where the volume should be mounted. |
| `sub_path`   | string  | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) in the volume instead of the root. |
| `medium`     | string  | No       | "Memory" provides a `tmpfs`, otherwise it defaults to the node disk storage (defaults to ""). |
| `size_limit` | string  | No       | The total amount of local storage required for the `emptyDir` volume. For more information, see [http://kubernetes.io/docs/user-guide/volumes#emptydir](http://kubernetes.io/docs/user-guide/volumes#emptydir). |

### `csi` volume

Configure a [Container Storage Interface (`csi`) volume](https://kubernetes.io/docs/concepts/storage/volumes/#csi) to instruct
Kubernetes to use a custom `csi` driver to mount an arbitrary storage system in the container.

Use the following options in the `config.toml`:

| Option              | Type                | Required | Description |
|---------------------|---------------------|----------|-------------|
| `name`              | string              | Yes      | The name of the volume. |
| `mount_path`        | string              | Yes      | Path inside of container where the volume should be mounted. |
| `driver`            | string              | Yes      | A string value that specifies the name of the volume driver to use. |
| `fs_type`           | string              | No       | A string value that specifies the name of the file system type (Ex. "ext4", "xfs", "ntfs".). |
| `volume_attributes` | `map[string]string` | No       | Key-value pair mapping for attributes of the CSI volume. |
| `sub_path`          | string              | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `read_only`         | boolean             | No       | Sets the volume in read-only mode (defaults to false). |

### Mount volumes on service containers

Volumes defined for the build container are also automatically mounted for all services containers. You can use this functionality as an alternative to [`services_tmpfs`](docker.md#mount-a-directory-in-ram) (available only to Docker executor), to mount database storage in RAM to speed up tests.

Example configuration in the `config.toml` file:

```toml
[[runners]]
  # usual configuration
  executor = "kubernetes"
  [runners.kubernetes]
    [[runners.kubernetes.volumes.empty_dir]]
      name = "mysql-tmpfs"
      mount_path = "/var/lib/mysql"
      medium = "Memory"
```

### Custom volume mount

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2862) in GitLab Runner 13.12.

To store the builds directory for the job, define custom volume mounts to the
configured `builds_dir` (`/builds` by default).
If you use [PVC volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/),
based on the
[access mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes),
you might be limited to running jobs on one node.

Example configuration in the `config.toml` file:

```toml
concurrent = 4

[[runners]]
  # usual configuration
  executor = "kubernetes"
  builds_dir = "/builds"
  [runners.kubernetes]
    [[runners.kubernetes.volumes.empty_dir]]
      name = "repo"
      mount_path = "/builds"
      medium = "Memory"
```

### Persistent per-concurrency build volumes

> Support for variable injection to `pvc.name` [introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4256) in GitLab 16.3.

The build directories in Kubernetes CI jobs are ephemeral by default.
If you want to persist your Git clone across jobs (to make `GIT_STRATEGY=fetch` work),
you must mount a persistent volume claim for your build folder.
Because multiple jobs can run concurrently, you must either
use a `ReadWriteMany` volume, or have one volume for each potential
concurrent job on the same runner. The latter is likely to be more performant.
Here is an example of such a configuration:

```toml
concurrent = 4

[[runners]]
  executor = "kubernetes"
  builds_dir = "/mnt/builds"
  [runners.kubernetes]
    [[runners.kubernetes.volumes.pvc]]
      # CI_CONCURRENT_ID identifies parallel jobs of the same runner.
      name = "build-pvc-$CI_CONCURRENT_ID"
      mount_path = "/mnt/builds"
```

In this example, create the persistent volume claims named
`build-pvc-0` to `build-pvc-3` yourself.
Create as many as the runner's `concurrent` setting dictates.

## Set a security policy for the pod

Configure the [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
in the `config.toml` to set a security policy for the build pod.

Use the following options:

| Option                | Type       | Required | Description                                                                                                      |
|-----------------------|------------|----------|------------------------------------------------------------------------------------------------------------------|
| `fs_group`            | `int`      | No       | A special supplemental group that applies to all containers in a pod.                                            |
| `run_as_group`        | `int`      | No       | The GID to run the entry point of the container process.                                                         |
| `run_as_non_root`     | boolean    | No       | Indicates that the container must run as a non-root user.                                                        |
| `run_as_user`         | `int`      | No       | The UID to run the entry point of the container process.                                                         |
| `supplemental_groups` | `int` list | No       | A list of groups applied to the first process run in each container, in addition to the container's primary GID. |
| `selinux_type`        | `string`   | No       | The SELinux type label that applies to all containers in a pod.                                                  |

Example of a pod security context in the `config.toml`:

```toml
concurrent = %(concurrent)s
check_interval = 30
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"
    [runners.kubernetes]
      helper_image = "gitlab-registry.example.com/helper:latest"
      [runners.kubernetes.pod_security_context]
        run_as_non_root = true
        run_as_user = 59417
        run_as_group = 59417
        fs_group = 59417
```

### Use a helper image

After you set the security policy, the [helper image](../configuration/advanced-configuration.md#helper-image) must conform to the policy.
The image does not receive privileges from the root group, so you must ensure that the user ID is part of the root group.

NOTE:
If you only need the `nonroot` environment, you can use the [GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421)
and [GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433)
OpenShift (OCP) images instead of a helper image.

The following example creates a user and group called `nonroot` and sets the helper image to run as that user.

```Dockerfile
ARG tag
FROM registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp:${tag}
USER root
RUN groupadd -g 59417 nonroot && \
    useradd -u 59417 nonroot -g nonroot
WORKDIR /home/nonroot
USER 59417:59417
```

## Set a security policy for the container

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3116) in GitLab Runner 14.5.

Configure the [container security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
in the `config.toml` executor to set a container security policy for the build, helper, or service pods.

Use the following options:

| Option                | Type        | Required | Description |
|-----------------------|-------------|----------|-------------|
| `run_as_group`        | int         | No       | The GID to run the entry point of the container process. |
| `run_as_non_root`     | boolean     | No       | Indicates that the container must run as a non-root user. |
| `run_as_user`         | int         | No       | The UID to run the entry point of the container process. |
| `capabilities.add`    | string list | No       | The capabilities to add when running the container. |
| `capabilities.drop`   | string list | No       | The capabilities to drop when running the container. |
| `selinux_type`        | string      | No       | The SELinux type label that is associated with the container process. |

In the following example in the `config.toml`, the security context configuration:

- Sets a pod security context.
- Overrides `run_as_user` and `run_as_group` for the build and helper containers.
- Specifies that all service containers inherit `run_as_user` and `run_as_group` from the pod security context.

```toml
concurrent = 4
check_interval = 30
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"
    [runners.kubernetes]
      helper_image = "gitlab-registry.example.com/helper:latest"
      [runners.kubernetes.pod_security_context]
        run_as_non_root = true
        run_as_user = 59417
        run_as_group = 59417
        fs_group = 59417
      [runners.kubernetes.init_permissions_container_security_context]
        run_as_user = 1000
        run_as_group = 1000
      [runners.kubernetes.build_container_security_context]
        run_as_user = 65534
        run_as_group = 65534
        [runners.kubernetes.build_container_security_context.capabilities]
          add = ["NET_ADMIN"]
      [runners.kubernetes.helper_container_security_context]
        run_as_user = 1000
        run_as_group = 1000
      [runners.kubernetes.service_container_security_context]
        run_as_user = 1000
        run_as_group = 1000
```

## Define a list of services

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4470) in GitLab Runner 12.5.
> - [Introduced support for `alias`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4829) in GitLab Runner 12.9
> - [Introduced support for `command` and `entrypoint`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173) in GitLab Runner 13.6.

Define a list of [services](https://docs.gitlab.com/ee/ci/services/) in the `config.toml`.

```toml
concurrent = 1
check_interval = 30
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"
    [runners.kubernetes]
      helper_image = "gitlab-registy.example.com/helper:latest"
      [[runners.kubernetes.services]]
        name = "postgres:12-alpine"
        alias = "db1"
      [[runners.kubernetes.services]]
        name = "registry.example.com/svc1"
        alias = "svc1"
        entrypoint = ["entrypoint.sh"]
        command = ["executable","param1","param2"]
```

## Set a pull policy

> Support for multiple pull policies [introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2807) in GitLab 13.11.

Use the `pull_policy` parameter in the `config.toml` file to specify a single or multiple pull policies.
The policy controls how an image is fetched and updated, and applies to the build image, helper image, and any services.

To determine which policy to use, see
[the Kubernetes documentation about pull policies](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy).

For a single pull policy:

```toml
[runners.kubernetes]
  pull_policy = "never"
```

For multiple pull policies:

```toml
[runners.kubernetes]
  # use multiple pull policies
  pull_policy = ["always", "if-not-present"]
```

When you define multiple policies, each policy is attempted until the image is obtained successfully.
For example, when you use `[ always, if-not-present ]`, the policy `if-not-present` is used if the `always` policy fails due to a temporary registry problem.

To retry a failed pull:

```toml
[runners.kubernetes]
  pull_policy = ["always", "always"]
```

The GitLab naming convention is different to the Kubernetes one.

| Runner pull policy | Kubernetes pull policy | Description |
|--------------------|------------------------|-------------|
| _blank_ | _blank_ | Uses the default policy, as specified by Kubernetes. |
| `if-not-present` | `IfNotPresent` | The image is pulled only if it is not already present on the node that executes the job. There are [security considerations](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy) you should be aware of. |
| `always` | `Always` | The image is pulled every time the job is executed. |
| `never` | `Never` | The image is never pulled and requires the node to already have it. |

## Specify the node to execute builds

Use the `node_selector` option to specify which node in a Kubernetes cluster to execute builds on.
It is a table of `key=value` pairs in the format of `string=string` (`string:string` in the case of environment variables).

The runner uses the information provided to determine the OS and architecture for the build. This ensures that
the correct [helper image](../configuration/advanced-configuration.md#helper-image) is used. By default, the OS and
architecture is assumed to be `linux/amd64`.

You can use specific labels to schedule nodes with different operating systems and architectures.

### Example for `linux/arm64`

```toml
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"

    [runners.kubernetes.node_selector]
      "kubernetes.io/arch" = "arm64"
      "kubernetes.io/os" = "linux"
```

### Example for `windows/amd64`

Kubernetes for Windows has certain [limitations](https://kubernetes.io/docs/concepts/windows/intro/#windows-os-version-support),
so if process isolation is used, you must also provide the specific windows build version with the
[`node.kubernetes.io/windows-build`](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesiowindows-build) label.

```toml
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"

    # The FF_USE_POWERSHELL_PATH_RESOLVER feature flag has to be enabled for PowerShell
    # to resolve paths for Windows correctly when Runner is operating in a Linux environment
    # but targeting Windows nodes.
    environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=true"]

    [runners.kubernetes.node_selector]
      "kubernetes.io/arch" = "amd64"
      "kubernetes.io/os" = "windows"
      "node.kubernetes.io/windows-build" = "10.0.20348"
```

## Define a list of node affinities

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2324) in GitLab Runner 13.4.

Define a list of [node affinities](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) to add to a pod specification at build time.

Example configuration in the `config.toml`:

```toml
concurrent = 1
[[runners]]
  name = "myRunner"
  url = "gitlab.example.com"
  executor = "kubernetes"
  [runners.kubernetes]
    [runners.kubernetes.affinity]
      [runners.kubernetes.affinity.node_affinity]
        [[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution]]
          weight = 100
          [runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference]
            [[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_expressions]]
              key = "cpu_speed"
              operator = "In"
              values = ["fast"]
            [[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_expressions]]
              key = "mem_speed"
              operator = "In"
              values = ["fast"]
        [[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution]]
          weight = 50
          [runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference]
            [[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_expressions]]
              key = "core_count"
              operator = "In"
              values = ["high", "32"]
            [[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_fields]]
              key = "cpu_type"
              operator = "In"
              values = ["arm64"]
      [runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution]
        [[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution.node_selector_terms]]
          [[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution.node_selector_terms.match_expressions]]
            key = "kubernetes.io/e2e-az-name"
            operator = "In"
            values = [
              "e2e-az1",
              "e2e-az2"
            ]
```

## Define nodes where pods are scheduled

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2324) in GitLab Runner 14.3.

Use pod affinity and anti-affinity to constrain the nodes
[your pod is eligible](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity)
to be scheduled on, based on labels on other pods.

Example configuration in the `config.toml`:

```toml
concurrent = 1
[[runners]]
  name = "myRunner"
  url = "gitlab.example.com"
  executor = "kubernetes"
  [runners.kubernetes]
    [runners.kubernetes.affinity]
      [runners.kubernetes.affinity.pod_affinity]
        [[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution]]
          topology_key = "failure-domain.beta.kubernetes.io/zone"
          namespaces = ["namespace_1", "namespace_2"]
          [runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution.label_selector]
            [[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution.label_selector.match_expressions]]
              key = "security"
              operator = "In"
              values = ["S1"]
        [[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution]]
        weight = 100
        [runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term]
          topology_key = "failure-domain.beta.kubernetes.io/zone"
          [runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector]
            [[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector.match_expressions]]
              key = "security_2"
              operator = "In"
              values = ["S2"]
      [runners.kubernetes.affinity.pod_anti_affinity]
        [[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution]]
          topology_key = "failure-domain.beta.kubernetes.io/zone"
          namespaces = ["namespace_1", "namespace_2"]
          [runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.label_selector]
            [[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.label_selector.match_expressions]]
              key = "security"
              operator = "In"
              values = ["S1"]
          [runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.namespace_selector]
            [[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.namespace_selector.match_expressions]]
              key = "security"
              operator = "In"
              values = ["S1"]
        [[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution]]
        weight = 100
        [runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term]
          topology_key = "failure-domain.beta.kubernetes.io/zone"
          [runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector]
            [[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector.match_expressions]]
              key = "security_2"
              operator = "In"
              values = ["S2"]
          [runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.namespace_selector]
            [[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.namespace_selector.match_expressions]]
              key = "security_2"
              operator = "In"
              values = ["S2"]
```

## Add extra host aliases

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2818) in GitLab Runner 13.7.

This feature is available in Kubernetes 1.7 and higher.

Configure a [host aliases](https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/) to
instruct Kubernetes to add entries to `/etc/hosts` file in the container.

Use the following options:

| Option       | Type          | Required | Description |
|--------------|---------------|----------|-------------|
| `IP`         | string        | Yes      | The IP address you want to attach hosts to. |
| `Hostnames`  | `string` list | Yes      | A list of host name aliases that will be attached to the IP. |

Example configuration in the `config.toml` file:

```toml
concurrent = 4

[[runners]]
  # usual configuration
  executor = "kubernetes"
  [runners.kubernetes]
    [[runners.kubernetes.host_aliases]]
      ip = "127.0.0.1"
      hostnames = ["web1", "web2"]
    [[runners.kubernetes.host_aliases]]
      ip = "192.168.1.1"
      hostnames = ["web14", "web15"]
```

## Configure a container lifecycle hook

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3630) in GitLab Runner 14.2.

Use [container lifecycle hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/) to run
code configured for a handler when the corresponding lifecycle hook is executed.

You can configure two types of hooks: `PreStop` and `PostStart`. Each of them allows only one type of handler to be set.

Example configuration in the `config.toml` file:

```toml
[[runners]]
  name = "kubernetes"
  url = "https://gitlab.example.com/"
  executor = "kubernetes"
  token = "yrnZW46BrtBFqM7xDzE7dddd"
  [runners.kubernetes]
    image = "alpine:3.11"
    privileged = true
    namespace = "default"
    [runners.kubernetes.container_lifecycle.post_start.exec]
      command = ["touch", "/builds/postStart.txt"]
    [runners.kubernetes.container_lifecycle.pre_stop.http_get]
      port = 8080
      host = "localhost"
      path = "/test"
      [[runners.kubernetes.container_lifecycle.pre_stop.http_get.http_headers]]
        name = "header_name_1"
        value = "header_value_1"
      [[runners.kubernetes.container_lifecycle.pre_stop.http_get.http_headers]]
        name = "header_name_2"
        value = "header_value_2"
```

Use the following settings to configure each lifecycle hook:

| Option       | Type                           | Required | Description |
|--------------|--------------------------------|----------|-------------|
| `exec`       | `KubernetesLifecycleExecAction`| No       | `Exec` specifies the action to take. |
| `http_get`   | `KubernetesLifecycleHTTPGet`   | No       | `HTTPGet` specifies the http request to perform. |
| `tcp_socket` | `KubernetesLifecycleTcpSocket` | No       | `TCPsocket` specifies an action involving a TCP port. |

### KubernetesLifecycleExecAction

| Option       | Type          | Required | Description |
|--------------|---------------|----------|-------------|
| `command`    | `string` list | Yes      | The command line to execute inside the container. |

### KubernetesLifecycleHTTPGet

| Option         | Type                                     | Required | Description |
|----------------|------------------------------------------|----------|-------------|
| `port`         | `int`                                    | Yes      | The number of the port to access on the container. |
| `host`         | string                                   | No       | The host name to connect to, defaults to the pod IP (optional). |
| `path`         | string                                   | No       | The path to access on the HTTP server (optional). |
| `scheme`       | string                                   | No       | The scheme used for connecting to the host. Defaults to HTTP (optional). |
| `http_headers` | `KubernetesLifecycleHTTPGetHeader` list  | No       | Custom headers to set in the request (optional). |

### KubernetesLifecycleHTTPGetHeader

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | HTTP header name.  |
| `value`      | string    | Yes      | HTTP header value. |

### KubernetesLifecycleTcpSocket

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `port`       | `int`     | Yes      | The number of the port to access on the container. |
| `host`       | string    | No       | The host name to connect to, defaults to the pod IP (optional). |

## Configure pod DNS settings

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6562) in GitLab Runner 13.7.

Use the following options to configure the [DNS settings](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-dns-config)
of the pods.

| Option       | Type                        | Required | Description |
|--------------|-----------------------------|----------|-------------|
| `nameservers`| `string` list               | No       | A list of IP addresses that will be used as DNS servers for the pod. |
| `options`    | `KubernetesDNSConfigOption` | No       | A optional list of objects where each object may have a name property (required) and a value property (optional). |
| `searches`   | `string` lists               | No       | A list of DNS search domains for hostname lookup in the pod.

Example configuration in the `config.toml` file:

```toml
concurrent = 1
check_interval = 30
[[runners]]
  name = "myRunner"
  url = "https://gitlab.example.com"
  token = "__REDACTED__"
  executor = "kubernetes"
  [runners.kubernetes]
    image = "alpine:latest"
    [runners.kubernetes.dns_config]
      nameservers = [
        "1.2.3.4",
      ]
      searches = [
        "ns1.svc.cluster-domain.example",
        "my.dns.search.suffix",
      ]

      [[runners.kubernetes.dns_config.options]]
        name = "ndots"
        value = "2"

      [[runners.kubernetes.dns_config.options]]
        name = "edns0"
```

### KubernetesDNSConfigOption

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | Configuration option name.  |
| `value`      | `*string` | No       | Configuration option value. |

## Specify container capabilities

You can specify the [Kubernetes capabilities](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container)
to use in the container.

To specify the container capabilities, use the `cap_add` and `cap_drop` options in the `config.toml`. Container runtimes can also
define a default list of capabilities, like those in [Docker](https://github.com/moby/moby/blob/19.03/oci/defaults.go#L14-L32)
or the [container](https://github.com/containerd/containerd/blob/v1.4.0/oci/spec.go#L93-L110).

There is a [list of capabilities](#default-list-of-dropped-capabilities) that the runner drops by default.
Capabilities that you list in `cap_add` option are excluded from being dropped.

Example configuration in the `config.toml` file:

```toml
concurrent = 1
check_interval = 30
[[runners]]
  name = "myRunner"
  url = "gitlab.example.com"
  executor = "kubernetes"
  [runners.kubernetes]
    # ...
    cap_add = ["SYS_TIME", "IPC_LOCK"]
    cap_drop = ["SYS_ADMIN"]
    # ...
```

When you specify the capabilities:

- User-defined `cap_drop` has priority over user-defined `cap_add`. If
   you define the same capability in both settings, only the capability from `cap_drop` is passed
   to the container.

- Remove the `CAP_` prefix from capability identifiers passed to the container configuration.
  For example, if you want to add or drop the `CAP_SYS_TIME` capability,
  in the configuration file, enter the string, `SYS_TIME`.

- The owner of the Kubernetes cluster
  [can define a PodSecurityPolicy](https://kubernetes.io/docs/concepts/security/pod-security-policy/#capabilities),
  where specific capabilities are allowed, restricted, or added by default. These rules take precedence over any user-defined configuration.

### Default list of dropped capabilities

GitLab Runner drops the following capabilities by default.

User-defined `cap_add` has priority over the default list of dropped capabilities.
If you want to add the capability that is dropped by default, add it to `cap_add`.

<!-- kubernetes_default_cap_drop_list_start -->
- `NET_RAW`

<!-- kubernetes_default_cap_drop_list_end -->

## Set the RuntimeClass

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26646) in GitLab Runner 14.9.

Use `runtime_class_name` to set the [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) for each job container.

If you specify a RuntimeClass name and it's not configured in the cluster or the feature is not supported, the executor fails to create jobs.

```toml
concurrent = 1
check_interval = 30
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"
    [runners.kubernetes]
      runtime_class_name = "myclass"
```

## Using Docker in builds

When you use Docker in your builds, there are several considerations
you should be aware of.

### Exposed `/var/run/docker.sock`

There are certain risks if you use the `runners.kubernetes.volumes.host_path` option
to expose the `/var/run/docker.sock` of your host into your build container.
The node's containers are accessible from the build container, and
depending on if you are running builds in the same cluster as your production
containers, it might not be wise to do that.

### Using `docker:dind`

If you run the `docker:dind`, also called the `docker-in-docker` image,
containers must run in privileged mode. This may have potential risks
and cause additional issues.

The Docker daemon runs as a separate container in the pod because it is started as a `service`,
typically in the `.gitlab-ci.yml`. Containers in pods only share volumes assigned to them and
an IP address, that they use to communicate with each other with `localhost`. The `docker:dind`
container does not share `/var/run/docker.sock` and the `docker` binary tries to use it by default.

To configure the client use TCP to contact the Docker daemon,
in the other container, include the environment variables of
the build container:

- `DOCKER_HOST=tcp://<hostname>:2375` for no TLS connection.
- `DOCKER_HOST=tcp://<hostname>:2376` for TLS connection.

For `hostname` set the value to:

- `localhost` for GitLab Runner 12.7 and earlier, and Kubernetes 1.6 and earlier.
- `docker` for GitLab Runner 12.8 and later, and Kubernetes 1.7 and later.

In Docker 19.03 and later, TLS is enabled by
default but you must map certificates to your client.
You can enable non-TLS connection for DIND or
mount certificates. For more information, see
[**Use Docker In Docker Workflow with Docker executor**](https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#use-docker-in-docker-workflow-with-docker-executor).

### Prevent host kernel exposure

If you use `docker:dind` or `/var/run/docker.sock`, the Docker daemon
has access to the underlying kernel of the host machine. This means that any
`limits` set in the pod do not work when Docker images are built.
The Docker daemon reports the full capacity of the node, regardless of
limits imposed on the Docker build containers spawned by Kubernetes.

If you run build containers in privileged mode, or if `/var/run/docker.sock` is exposed,
the host kernel may become exposed to build containers. To minimize exposure, specify a label
in the `node_selector` option. This ensures that the node matches the labels before any containers
can be deployed to the node. For example, if you specify the label `role=ci`, the build containers
only run on nodes labeled `role=ci`, and all other production services run on other nodes.

To further separate build containers, you can use node
[taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).
Taints prevent other pods from scheduling on the same nodes as the
build pods, without extra configuration for the other pods.

### Use kaniko to build Docker images

You can use [kaniko](https://github.com/GoogleContainerTools/kaniko) to build Docker images inside a Kubernetes cluster.

Kaniko works without the Docker daemon and builds images without privileged access.

For more information, see [Building images with kaniko and GitLab CI/CD](https://docs.gitlab.com/ee/ci/docker/using_kaniko.html).

There is a known issue when using kaniko to build _multi-stage_ `Dockerfiles`. If a pipeline job includes an
`after_script` section, when the `after_script` section is executed it fails with the following error message. The job still completes successfully.

```shell
OCI runtime exec failed: exec failed: container_linux.go:380: starting container process caused: chdir to cwd
("/workspace") set in config.json failed: no such file or directory: unknown
```

The section fails because kaniko deletes its container's `WORKDIR` when building multi-stage `Dockerfile`s. This prevents
`kubectl exec` (and the analogous SDK API) from attaching to the container.

Two workarounds exists for this issue:

- Add `--ignore-path /workspace` to the kaniko executor invocation.
- Add `mkdir -p /workspace` to the job's `script` _after_ the kaniko executor invocation.

For more information, see [issue 30769](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/30769#note_1452088669).

### Restrict Docker images and services

> Added for the Kubernetes executor in GitLab Runner 14.2.

You can restrict the Docker images that are used to run your jobs.
To do this, you specify wildcard patterns. For example, to allow images
from your private Docker registry only:

```toml
[[runners]]
  (...)
  executor = "kubernetes"
  [runners.kubernetes]
    (...)
    allowed_images = ["my.registry.tld:5000/*:*"]
    allowed_services = ["my.registry.tld:5000/*:*"]
```

Or, to restrict to a specific list of images from this registry:

```toml
[[runners]]
  (...)
  executor = "kubernetes"
  [runners.kubernetes]
    (...)
    allowed_images = ["my.registry.tld:5000/ruby:*", "my.registry.tld:5000/node:*"]
    allowed_services = ["postgres:9.4", "postgres:latest"]
```

### Restrict Docker pull policies

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26753) in GitLab 15.1.

In the `.gitlab-ci.yml` file, you can specify a pull policy. This policy determines how
a CI/CD job should fetch images.

To restrict which pull policies can be used in the `.gitlab-ci.yml` file, you can use `allowed_pull_policies`.

For example, to allow only the `always` and `if-not-present` pull policies:

```toml
[[runners]]
  (...)
  executor = "kubernetes"
  [runners.kubernetes]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- If you don't specify `allowed_pull_policies`, the default is the value in the `pull_policy` keyword.
- If you don't specify `pull_policy`, the cluster's image [default pull policy](https://kubernetes.io/docs/concepts/containers/images/#updating-images) is used.
- The existing [`pull_policy` keyword](../executors/kubernetes.md#set-a-pull-policy) must not include a pull policy
  that is not specified in `allowed_pull_policies`. If it does, the job returns an error.

## Job execution

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1775) in GitLab Runner 12.9.
> - [Behind a feature flag `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`](../configuration/feature-flags.md#available-feature-flags), enabled by default.
> - [Using attach by default](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/10341) in GitLab Runner 14.0.

GitLab Runner uses `kube attach` instead of `kube exec` by default. This should avoid problems like when a [job is marked successful midway](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119)
in environments with an unstable network.

If you experience problems after updating to GitLab Runner 14.0, toggle the feature flag [`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`](../configuration/feature-flags.md#available-feature-flags)
to `true` and [submit an issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/new).

Follow [issue #27976](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27976) for progress on legacy execution strategy removal.

### Container entrypoint known issues

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3095) in GitLab Runner 14.5.
> - [Updated](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3212) in GitLab Runner 15.1.

NOTE:
In GitLab 14.5 to 15.0, GitLab Runner uses the entrypoint defined in a Docker image when used with the Kubernetes executor with `kube attach`.
In GitLab 15.1 and later, the entrypoint defined in a Docker image is used with the Kubernetes executor when `FF_KUBERNETES_HONOR_ENTRYPOINT` is set.

The container entry point has the following known issues:

- If an entrypoint is defined in the Dockerfile for an image, it must open a valid shell. Otherwise, the job hangs.
- [File type CI/CD variables](https://docs.gitlab.com/ee/ci/variables/index.html#use-file-type-cicd-variables)
  are not written to disk when the entrypoint is executed. The file is only accessible
  in the job during script execution.
- The following CI/CD variables are not accessible in the entrypoint. You can use
  [`before_script`](https://docs.gitlab.com/ee/ci/yaml/index.html#beforescript) to make
  any setup changes before running script commands:
  - [CI/CD variables defined in the settings](https://docs.gitlab.com/ee/ci/variables/#define-a-cicd-variable-in-the-ui).
  - [Masked CI/CD variables](https://docs.gitlab.com/ee/ci/variables/#mask-a-cicd-variable).

## Remove old runner pods

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27870) in GitLab Runner 14.6.

Sometimes old runner pods are not cleared. This can happen when the runner manager is incorrectly shut down.

To handle this situation, you can use the GitLab Runner Pod Cleanup application to schedule cleanup of old pods. For more information, see:

- The GitLab Runner Pod Cleanup project [README](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pod-cleanup/-/blob/main/readme.md).
- GitLab Runner Pod Cleanup [documentation](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pod-cleanup/-/blob/main/docs/README.md).

## Troubleshooting

The following errors are commonly encountered when using the Kubernetes executor.

### `Job failed (system failure): timed out waiting for pod to start`

If the cluster cannot schedule the build pod before the timeout defined by `poll_timeout`, the build pod returns an error. The [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime) should be able to delete it.

To fix this issue, increase the `poll_timeout` value in your `config.toml` file.

### `context deadline exceeded`

The `context deadline exceeded` errors in job logs usually indicate that the Kubernetes API client hit a timeout for a given cluster API request.

Check the [metrics of the `kube-apiserver` cluster component](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/) for any signs of:

- Increased response latencies.
- Error rates for common create or delete operations over pods, secrets, ConfigMaps, and other core (v1) resources.

Logs for timeout-driven errors from the `kube-apiserver` operations may appear as:

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

In some cases, the `kube-apiserver` error response might provide additional details of its sub-components failing (such as the Kubernetes cluster's `etcdserver`):

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

These `kube-apiserver` service failures can occur during the creation of the build pod and also during cleanup attempts after completion:

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

### Dial tcp xxx.xx.x.x:xxx: i/o timeout

This is a Kubernetes error that generally indicates the Kubernetes API server is unreachable by the runner manager.
To resolve this issue:

- If you use network security policies, grant access to the Kubernetes API, typically on port 443 or port 6443, or both.
- Ensure that the Kubernetes API is running.

### Connection refused when attempting to communicate with the Kubernetes API

When GitLab Runner makes a request to the Kubernetes API and it fails,
it is likely because
[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)
is overloaded and can't accept or process API requests.

### `Error cleaning up pod` and `Job failed (system failure): prepare environment: waiting for pod running`

The following errors occur when Kubernetes fails to schedule the job pod in a timely manner.
GitLab Runner waits for the pod to be ready, but it fails and then tries to clean up the pod, which can also fail.

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

To troubleshoot, check the Kubernetes primary node and all nodes that run a
[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)
instance. Ensure they have all of the resources needed to manage the target number
of pods that you hope to scale up to on the cluster.

To change the time GitLab Runner waits for a pod to reach its `Ready` status, use the
[`poll_timeout`](#other-configtoml-settings) setting.

To better understand how pods are scheduled or why they might not get scheduled
on time, [read about the Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).

### `request did not complete within requested timeout`

The message `request did not complete within requested timeout` observed during build pod creation indicates that a configured [admission control webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) on the Kubernetes cluster is timing out.

Admission control webhooks are a cluster-level administrative control intercept for all API requests they're scoped for, and can cause failures if they do not execute in time.

Admission control webhooks support filters that can finely control which API requests and namespace sources it intercepts. If the Kubernetes API calls from GitLab Runner do not need to pass through an admission control webhook then you may alter the [webhook's selector/filter configuration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector) to ignore the GitLab Runner namespace, or apply exclusion labels/annotations over the GitLab Runner pod by configuring `podAnnotations` or `podLabels` in the [GitLab Runner Helm Chart `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500).

For example, to avoid [DataDog Admission Controller webhook](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator) from intercepting API requests made by the GitLab Runner manager pod, the following can be added:

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

To list a Kubernetes cluster's admission control webhooks, run:

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

The following forms of logs can be observed when an admission control webhook times out:

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

A failure from an admission control webhook may instead appear as:

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

### `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

If using the `alpine` flavor of the [helper image](../configuration/advanced-configuration.md#helper-image),
there can be [DNS issues](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129) related to Alpine's `musl`'s DNS resolver.

Using the `helper_image_flavor = "ubuntu"` option should resolve this.

### `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?`

This error can occur when [using Docker-in-Docker](#using-dockerdind) if attempts are made to access the DIND service before it has had time to fully start up. For a more detailed explanation, see [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215).

### `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443`

This error can happen when [using Docker-in-Docker](#using-dockerdind) if the DIND Maximum Transmission Unit (MTU) is larger than the Kubernetes overlay network. DIND uses a default MTU of 1500, which is too large to route across the default overlay network. The DIND MTU can be changed within the service definition:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

### `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows`

When you run your CI/CD job, you might receive an error like the following:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

This issue occurs when you [use node selectors](#specify-the-node-to-execute-builds) to run builds on nodes with different operating systems and architectures.

To fix the issue, configure `nodeSelector` so that the runner manager pod is always scheduled on a Linux node. For example, your [`values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) should contain the following:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

### Build pods are assigned the worker node's IAM role instead of Runner IAM role

This issue happens when the worker node IAM role does not have the permission to assume the correct role. To fix this, add the `sts:AssumeRole` permission to the trust relationship of the worker node's IAM role:

```json
{
    "Effect": "Allow",
    "Principal": {
        "AWS": "arn:aws:iam::<AWS_ACCOUNT_NUMBER>:role/<IAM_ROLE_NAME>"
    },
    "Action": "sts:AssumeRole"
}
```

### `Preparation failed: failed to pull image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

This issue happens if you specified a `pull_policy` in your `.gitlab-ci.yml` but there is no policy configured in the Runner's config file. To fix this, add `allowed_pull_policies` to your config according to [Restrict Docker pull policies](#restrict-docker-pull-policies).

### Background processes cause jobs to hang and timeout

Background processes started during job execution can [prevent the build job from exiting](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880). To avoid this you can:

- Double fork the process. For example, `command_to_run < /dev/null &> /dev/null &`.
- Kill the process before exiting the job script.

### Cache-related `permission denied` errors

Files and folders that are generated in your job have certain UNIX ownerships and permissions.
When your files and folders are archived or extracted, UNIX details are retained.
However, the files and folders may mismatch with the `USER` configurations of
[helper images](../configuration/advanced-configuration.md#helper-image).

If you encounter permission-related errors in the `Creating cache ...` step,
you can:

- As a solution, investigate whether the source data is modified,
  for example in the job script that creates the cached files.
- As a workaround, add matching [chown](https://linux.die.net/man/1/chown) and
  [chmod](https://linux.die.net/man/1/chmod) commands.
  to your [(`before_`/`after_`)`script:` directives](https://docs.gitlab.com/ee/ci/yaml/index.html#default).

### Apparently redundant shell process in build container with init system

The process tree might include a shell process when either:

- `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` is `false` and `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` is `true`.
- The `ENTRYPOINT` of the build image is an init system (like `tini-init` or `dumb-init`).

```shell
UID    PID   PPID  C STIME TTY          TIME CMD
root     1      0  0 21:58 ?        00:00:00 /scripts-37474587-5556589047/dumb-init -- sh -c if [ -x /usr/local/bin/bash ]; then .exec /usr/local/bin/bash  elif [ -x /usr/bin/bash ]; then .exec /usr/bin/bash  elif [ -x /bin/bash ]; then .exec /bin/bash  elif [ -x /usr/local/bin/sh ]; then .exec /usr/local/bin/sh  elif [ -x /usr/bin/sh ]; then .exec /usr/bin/sh  elif [ -x /bin/sh ]; then .exec /bin/sh  elif [ -x /busybox/sh ]; then .exec /busybox/sh  else .echo shell not found .exit 1 fi
root     7      1  0 21:58 ?        00:00:00 /usr/bin/bash <---------------- WHAT IS THIS???
root    26      1  0 21:58 ?        00:00:00 sh -c (/scripts-37474587-5556589047/detect_shell_script /scripts-37474587-5556589047/step_script 2>&1 | tee -a /logs-37474587-5556589047/output.log) &
root    27     26  0 21:58 ?        00:00:00  \_ /usr/bin/bash /scripts-37474587-5556589047/step_script
root    32     27  0 21:58 ?        00:00:00  |   \_ /usr/bin/bash /scripts-37474587-5556589047/step_script
root    37     32  0 21:58 ?        00:00:00  |       \_ ps -ef --forest
root    28     26  0 21:58 ?        00:00:00  \_ tee -a /logs-37474587-5556589047/output.log
```

This shell process, which might be `sh`, `bash` or `busybox`, with a `PPID` of 1 and a `PID` of 6 or 7, is the shell
started by the shell detection script run by the init system (`PID` 1 above). The process is not redundant, and is the typical
operation when the build container runs with an init system.

### Runner pod fails to run job results and timesout despite successful registration

After the runner pod registers with GitLab, it attempts to run a job but does not and the job eventually times out. The following errors are reported:

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

In this case, the runner might receive the error,

> HTTP 204 No content response code when connecting to the `jobs/request` API.

To troubleshoot this issue, manually send a POST request to the API to
validate if the TCP connection is hanging. If the TCP connection is hanging,
the runner might not be able to request CI job payloads.

### `failed to reserve container name` for init-permissions container when `gcs-fuse-csi-driver` is used

The `gcs-fuse-csi-driver` `csi` driver [does not support mounting volumes for the init container](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38). This can cause failures starting the init container when using this driver. Features [introduced in Kubernetes 1.28](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/) must be supported in the driver's project to resolve this bug.
