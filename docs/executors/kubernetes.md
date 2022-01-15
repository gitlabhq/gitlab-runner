---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments
---

# The Kubernetes executor for GitLab Runner **(FREE)**

GitLab Runner can use Kubernetes to run builds on a Kubernetes cluster. This is
possible with the use of the Kubernetes executor.

The Kubernetes executor, when used with GitLab CI, connects to the Kubernetes
API in the cluster creating a Pod for each GitLab CI Job. This Pod is made
up of, at the very least, a build container, a helper container, and an additional container for each
`service` defined in the `.gitlab-ci.yml` or `config.toml` files. The names for these containers
are as follows:

- The build container is `build`
- The helper container is `helper`
- The services containers are `svc-X` where `X` is `[0-9]+`

Note that when services and containers are running in the same Kubernetes
pod, they are all sharing the same localhost address. The following restrictions
are then applicable:

- Since GitLab Runner 12.8 and Kubernetes 1.7, the services are accessible via their DNS names. If you are using an older version you will have to use `localhost`.
- You cannot use several services using the same port (e.g., you cannot have two
  `mysql` services at the same time).

## Workflow

The Kubernetes executor divides the build into multiple steps:

1. **Prepare**: Create the Pod against the Kubernetes Cluster.
   This creates the containers required for the build and services to run.
1. **Pre-build**: Clone, restore cache and download artifacts from previous
   stages. This is run on a special container as part of the Pod.
1. **Build**: User build.
1. **Post-build**: Create cache, upload artifacts to GitLab. This also uses
   the special container as part of the Pod.

## Connecting to the Kubernetes API

The following options are provided, which allow you to connect to the Kubernetes API:

- `host`: Optional Kubernetes apiserver host URL (auto-discovery attempted if not specified)
- `cert_file`: Optional Kubernetes apiserver user auth certificate
- `key_file`: Optional Kubernetes apiserver user auth private key
- `ca_file`: Optional Kubernetes apiserver ca certificate

The user account provided must have permission to create, list and attach to Pods in
the specified namespace in order to function.

If you are running GitLab Runner within the Kubernetes cluster you can omit
all of the above fields to have GitLab Runner auto-discover the Kubernetes API. This
is the recommended approach.

If you are running it externally to the Cluster then you will need to set each
of these settings and make sure that GitLab Runner has access to the Kubernetes API
on the cluster.

## Kubernetes executor interaction diagram

The diagram below depicts the interaction with a GitLab Runner hosted on a Kubernetes cluster and the Kubernetes API. The Kubernetes API is the mechanism that is used by GitLab Runner on Kubernetes to create pods on the cluster. The interaction depicted in this diagram is valid on any Kubernetes cluster, whether that's a turnkey solution hosted on the major public cloud providers or a self-managed Kubernetes installation.

NOTE:
The GitLab Runner [Helm chart](../install/kubernetes.md) has been tested on Google Kubernetes Engine and Azure Container Service.

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
    P->>+P: Execute job
    Note over P: CI build job = Prepare + Pre-build + Build + Post-build
    P->>+G: Job logs
```

## The available `config.toml` settings

The following settings help to define the behavior of GitLab Runner within Kubernetes.

### CPU requests and limits

| Setting | Description |
|---------|-------------|
| `cpu_limit` | The CPU allocation given to build containers. |
| `cpu_limit_overwrite_max_allowed` | The max amount the CPU allocation can be written to for build containers. When empty, it disables the cpu limit overwrite feature. |
| `cpu_request` | The CPU allocation requested for build containers. |
| `cpu_request_overwrite_max_allowed` | The max amount the CPU allocation request can be written to for build containers. When empty, it disables the cpu request overwrite feature. |
| `helper_cpu_limit` | The CPU allocation given to build helper containers. |
| `helper_cpu_limit_overwrite_max_allowed` | The max amount the CPU allocation can be written to for helper containers. When empty, it disables the cpu limit overwrite feature. |
| `helper_cpu_request` | The CPU allocation requested for build helper containers. |
| `helper_cpu_request_overwrite_max_allowed` | The max amount the CPU allocation request can be written to for helper containers. When empty, it disables the cpu request overwrite feature. |
| `service_cpu_limit` | The CPU allocation given to build service containers. |
| `service_cpu_limit_overwrite_max_allowed` | The max amount the CPU allocation can be written to for service containers. When empty, it disables the cpu limit overwrite feature. |
| `service_cpu_request` | The CPU allocation requested for build service containers. |
| `service_cpu_request_overwrite_max_allowed` | The max amount the CPU allocation request can be written to for service containers. When empty, it disables the cpu request overwrite feature. |

### Memory requests and limits

| Setting | Description |
|---------|-------------|
| `memory_limit` | The amount of memory allocated to build containers. |
| `memory_limit_overwrite_max_allowed` | The max amount the memory allocation can be written to for build containers. When empty, it disables the memory limit overwrite feature. |
| `memory_request` | The amount of memory requested from build containers. |
| `memory_request_overwrite_max_allowed` | The max amount the memory allocation request can be written to for build containers. When empty, it disables the memory request overwrite feature. |
| `helper_memory_limit` | The amount of memory allocated to build helper containers. |
| `helper_memory_limit_overwrite_max_allowed` | The max amount the memory allocation can be written to for helper containers. When empty, it disables the memory limit overwrite feature. |
| `helper_memory_request` | The amount of memory requested for build helper containers. |
| `helper_memory_request_overwrite_max_allowed` | The max amount the memory allocation request can be written to for helper containers. When empty, it disables the memory request overwrite feature. |
| `service_memory_limit` | The amount of memory allocated to build service containers. |
| `service_memory_limit_overwrite_max_allowed` | The max amount the memory allocation can be written to for service containers. When empty, it disables the memory limit overwrite feature. |
| `service_memory_request` | The amount of memory requested for build service containers. |
| `service_memory_request_overwrite_max_allowed` | The max amount the memory allocation request can be written to for service containers. When empty, it disables the memory request overwrite feature. |

### Storage requests and limits

| Setting | Description |
|---------|-------------|
| `ephemeral_storage_limit` | The ephemeral storage limit for build containers. |
| `ephemeral_storage_limit_overwrite_max_allowed` | The max amount the ephemeral storage limit for build containers can be overwritten. When empty, it disables the ephemeral storage limit overwrite feature. |
| `ephemeral_storage_request` | The ephemeral storage request given to build containers. |
| `ephemeral_storage_request_overwrite_max_allowed` | The max amount the ephemeral storage request can be overwritten by for build containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `helper_ephemeral_storage_limit` | The ephemeral storage limit given to helper containers. |
| `helper_ephemeral_storage_limit_overwrite_max_allowed` | The max amount the ephemeral storage limit can be overwritten by for helper containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `helper_ephemeral_storage_request` | The ephemeral storage request given to helper containers. |
| `helper_ephemeral_storage_request_overwrite_max_allowed` | The max amount the ephemeral storage request can be overwritten by for helper containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `service_ephemeral_storage_limit` | The ephemeral storage limit given to service containers. |
| `service_ephemeral_storage_limit_overwrite_max_allowed` | The max amount the ephemeral storage limit can be overwritten by for service containers. When empty, it disables the ephemeral storage request overwrite feature. |
| `service_ephemeral_storage_request` | The ephemeral storage request given to service containers. |
| `service_ephemeral_storage_request_overwrite_max_allowed` | The max amount the ephemeral storage request can be overwritten by for service containers. When empty, it disables the ephemeral storage request overwrite feature. |

### Other `config.toml` settings

| Setting | Description |
|---------|-------------|
| `affinity` | Specify affinity rules that determine which node runs the build. Read more about [using affinity](#using-affinity). |
| `allow_privilege_escalation` | Run all containers with the `allowPrivilegeEscalation` flag enabled. When empty, it does not define the `allowPrivilegeEscalation` flag in the container `SecurityContext` and allows Kubernetes to use the default [privilege escalation](https://kubernetes.io/docs/concepts/policy/pod-security-policy/#privilege-escalation) behavior. |
| `allowed_images` | Wildcard list of images that can be specified in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`). See [Restrict Docker images and services](../configuration/advanced-configuration.md#restricting-docker-images-and-services). |
| `allowed_services` | Wildcard list of services that can be specified in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`). See [Restrict Docker images and services](../configuration/advanced-configuration.md#restricting-docker-images-and-services). |
| `bearer_token` | Default bearer token used to launch build pods. |
| `bearer_token_overwrite_allowed` | Boolean to allow projects to specify a bearer token that will be used to create the build pod. |
| `cap_add` | Specify Linux capabilities that should be added to the job pod containers. [Read more about capabilities configuration in Kubernetes executor](#capabilities-configuration). |
| `cap_drop` | Specify Linux capabilities that should be dropped from the job pod containers. [Read more about capabilities configuration in Kubernetes executor](#capabilities-configuration). |
| `cleanup_grace_period_seconds` | When a job completes, the duration in seconds that the pod has to terminate gracefully. After this period, the processes are forcibly halted with a kill signal. Ignored if `terminationGracePeriodSeconds` is specified. |
| `helper_image` | (Advanced) [Override the default helper image](../configuration/advanced-configuration.md#helper-image) used to clone repos and upload artifacts. |
| `helper_image_flavor` | Sets the helper image flavor (`alpine`, `alpine3.12`, `alpine3.13`, `alpine3.14`, `alpine3.15`, or `ubuntu`). Defaults to `alpine`. Using `alpine` is the same as `alpine3.12`. |
| `host_aliases` | List of additional host name aliases that will be added to all containers. [Read more about using extra host aliases](#adding-extra-host-aliases). |
| `image_pull_secrets` | An array of items containing the Kubernetes `docker-registry` secret names used to authenticate Docker image pulling from private registries. |
| `namespace` | Namespace in which to run Kubernetes Pods. |
| `namespace_overwrite_allowed` | Regular expression to validate the contents of the namespace overwrite environment variable (documented below). When empty, it disables the namespace overwrite feature. |
| `node_selector` | A `table` of `key=value` pairs in the format of `string=string` (`string:string` in the case of environment variables). Setting this limits the creation of pods to Kubernetes nodes matching all the `key=value` pairs. [Read more about using node selectors](#using-node-selectors). |
| `node_tolerations` | A `table` of `"key=value" = "Effect"` pairs in the format of `string=string:string`. Setting this allows pods to schedule to nodes with all or a subset of tolerated taints. Only one toleration can be supplied through environment variable configuration. The `key`, `value`, and `effect` match with the corresponding field names in Kubernetes pod toleration configuration. |
| `pod_annotations` | A `table` of `key=value` pairs in the format of `string=string`. This is the list of annotations to be added to each build pod created by the Runner. The value of these can include environment variables for expansion. Pod annotations can be overwritten in each build. |
| `pod_annotations_overwrite_allowed` | Regular expression to validate the contents of the pod annotations overwrite environment variable. When empty, it disables the pod annotations overwrite feature. |
| `pod_labels` | A set of labels to be added to each build pod created by the runner. The value of these can include environment variables for expansion. |
| `pod_security_context` | Configured through the configuration file, this sets a pod security context for the build pod. [Read more about security context](#using-security-context). |
| `build_container_security_context` | Sets a container security context for the build container. [Read more about security context](#using-security-context). |
| `helper_container_security_context` | Sets a container security context for the helper container. [Read more about security context](#using-security-context). |
| `service_container_security_context` | Sets a container security context for the service containers. [Read more about security context](#using-security-context). |
| `pod_termination_grace_period_seconds` | Pod-level setting which determines the duration in seconds which the pod has to terminate gracefully. After this, the processes are forcibly halted with a kill signal. Ignored if `terminationGracePeriodSeconds` is specified. |
| `poll_interval` | How frequently, in seconds, the runner will poll the Kubernetes pod it has just created to check its status (default = 3). |
| `poll_timeout` | The amount of time, in seconds, that needs to pass before the runner will time out attempting to connect to the container it has just created. Useful for queueing more builds that the cluster can handle at a time (default = 180). |
| `privileged` | Run containers with the privileged flag. |
| `pull_policy` | Specify the image pull policy: `never`, `if-not-present`, `always`. If not set, the cluster's image [default pull policy](https://kubernetes.io/docs/concepts/containers/images/#updating-images) is used. For more information and instructions on how to set multiple pull policies, see [using pull policies](#using-pull-policies). See also [`if-not-present`, `never` security considerations](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy). |
| `service_account` | Default service account job/executor pods use to talk to Kubernetes API. |
| `service_account_overwrite_allowed` | Regular expression to validate the contents of the service account overwrite environment variable. When empty, it disables the service account overwrite feature. |
| `services` | [Since GitLab Runner 12.5](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4470), list of [services](https://docs.gitlab.com/ee/ci/services/) attached to the build container using the [sidecar pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/sidecar). Read more about [using services](#using-services). |
| `terminationGracePeriodSeconds` | Duration after the processes running in the pod are sent a termination signal and the time when the processes are forcibly halted with a kill signal. [Deprecated in favour of `cleanup_grace_period_seconds` and `pod_termination_grace_period_seconds`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28165). |
| `volumes` | Configured through the configuration file, the list of volumes that will be mounted in the build container. [Read more about using volumes](#using-volumes). |
| `dns_policy` | Specify the DNS policy that should be used when constructing the pod: `none`, `default`, `cluster-first`, `cluster-first-with-host-net`. The Kubernetes default (`cluster-first`) will be used if not set. |
| `dns_config` | Specify the DNS configuration that should be used when constructing the pod. [Read more about using pod's DNS config](#pods-dns-config). |

### Configuring executor Service Account

You can set the `KUBERNETES_SERVICE_ACCOUNT` environment variable or use `--kubernetes-service-account` flag.

### Overwriting Kubernetes Namespace

Additionally, Kubernetes namespace can be overwritten on `.gitlab-ci.yml` file, by using the variable
`KUBERNETES_NAMESPACE_OVERWRITE`.

This approach allows you to create a new isolated namespace dedicated for CI purposes, and deploy a custom
set of Pods. The `Pods` spawned by the runner will take place on the overwritten namespace, for simple
and straight forward access between containers during the CI stages.

``` yaml
variables:
  KUBERNETES_NAMESPACE_OVERWRITE: ci-${CI_COMMIT_REF_SLUG}
```

Furthermore, to ensure only designated namespaces will be used during CI runs, set the configuration
`namespace_overwrite_allowed` with an appropriate regular expression. When left empty the overwrite behavior is
disabled.

When you overwrite the Kubernetes namespace, make sure that:

- In the `values.yml` file for GitLab Runner Helm charts, this value is set: `rbac.clusterWideAccess: true`.
- The runner has these permissions in the core API group:

  | Resource   | Permissions |
  |------------|-------------|
  | pods/exec  | create, patch, delete |
  | pods       | get, list, watch, create, patch, delete |
  | secrets    | get, list, watch, create, patch, delete |

This can be achieved by setting `rbac.create: true` or by specifying a service account `rbac.serviceAccountName: <service_account_name>` with the above permissions in the `values.yml` file.

### Overwriting Kubernetes Default Service Account

Additionally, the Kubernetes service account can be overwritten in the `.gitlab-ci.yml` file by using the variable
`KUBERNETES_SERVICE_ACCOUNT_OVERWRITE`.

This approach allows you to specify a service account that is attached to the namespace, which is useful when dealing
with complex RBAC configurations.

``` yaml
variables:
  KUBERNETES_SERVICE_ACCOUNT_OVERWRITE: ci-service-account
```

To ensure only designated service accounts will be used during CI runs, set the configuration
`service_account_overwrite_allowed` or set the environment variable `KUBERNETES_SERVICE_ACCOUNT_OVERWRITE_ALLOWED`
with an appropriate regular expression. When left empty the overwrite behavior is disabled.

### Setting Bearer Token to be Used When Making Kubernetes API calls

In conjunction with setting the namespace and service account as mentioned above, you may set the
bearer token used when making API calls to create the build pods. This will allow project owners to
use project secret variables to specify a bearer token. When specifying the bearer token, you must
set the `Host` configuration setting.

``` yaml
variables:
  KUBERNETES_BEARER_TOKEN: thebearertokenfromanothernamespace
```

### Overwriting pod annotations

Additionally, Kubernetes pod annotations can be overwritten on the `.gitlab-ci.yml` file, by using `KUBERNETES_POD_ANNOTATIONS_*` for variables and `key=value` for the value. The pod annotations will be overwritten to the `key=value`. Multiple annotations can be applied. For example:

```yaml
variables:
  KUBERNETES_POD_ANNOTATIONS_1: "Key1=Val1"
  KUBERNETES_POD_ANNOTATIONS_2: "Key2=Val2"
  KUBERNETES_POD_ANNOTATIONS_3: "Key3=Val3"
```

NOTE:
You must specify [`pod_annotations_overwrite_allowed`](#the-available-configtoml-settings) to override pod annotations via the `.gitlab-ci.yml` file.

### Overwriting Container Resources

Additionally, Kubernetes CPU and memory allocations for requests and
limits for the build, helper and service containers can be overwritten
on the `.gitlab-ci.yml` file with the following variables:

``` yaml
 variables:
   KUBERNETES_CPU_REQUEST: 3
   KUBERNETES_CPU_LIMIT: 5
   KUBERNETES_MEMORY_REQUEST: 2Gi
   KUBERNETES_MEMORY_LIMIT: 4Gi
   KUBERNETES_EPHEMERAL_STORAGE_REQUEST: 512Mi
   KUBERNETES_EPHEMERAL_STORAGE_LIMIT: 1Gi

   KUBERNETES_HELPER_CPU_REQUEST: 3
   KUBERNETES_HELPER_CPU_LIMIT: 5
   KUBERNETES_HELPER_MEMORY_REQUEST: 2Gi
   KUBERNETES_HELPER_MEMORY_LIMIT: 4Gi
   KUBERNETES_HELPER_EPHEMERAL_STORAGE_REQUEST: 512Mi
   KUBERNETES_HELPER_EPHEMERAL_STORAGE_LIMIT: 1Gi

   KUBERNETES_SERVICE_CPU_REQUEST: 3
   KUBERNETES_SERVICE_CPU_LIMIT: 5
   KUBERNETES_SERVICE_MEMORY_REQUEST: 2Gi
   KUBERNETES_SERVICE_MEMORY_LIMIT: 4Gi
   KUBERNETES_SERVICE_EPHEMERAL_STORAGE_REQUEST: 512Mi
   KUBERNETES_SERVICE_EPHEMERAL_STORAGE_LIMIT: 1Gi
```

The values for these variables are restricted to the [max overwrite](#the-available-configtoml-settings)
setting for that resource. If the max overwrite has not been set for a resource, the variable is ignored.

## Define settings in the configuration TOML

Each of the settings can be defined in the `config.toml` file.

Here is an example `config.toml`:

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
    [runners.kubernetes.node_selector]
      gitlab = "true"
    [runners.kubernetes.node_tolerations]
      "node-role.kubernetes.io/master" = "NoSchedule"
      "custom.toleration=value" = "NoSchedule"
      "empty.value=" = "PreferNoSchedule"
      "onlyKey" = ""
```

## Using the cache with the Kubernetes executor

When the cache is used with the Kubernetes executor, a specific volume called `/cache` is mounted on the pod. 
The cache volume can be configured in the `config.toml` file by using the `cache_dir` setting.

During the job's execution, if the cached data is needed, the runner checks to see if cached data is available (if a compressed file is available on the cache volume).

- If available, the compressed file is extracted into the build folder and can then be used in the job.
- If not available, the cached data is downloaded from the configured storage and saved into the `cache dir` as a compressed file.
  The compressed file is then extracted into the `build` folder.

## Using volumes

As described earlier, volumes can be mounted in the build container.
At this time _hostPath_, _PVC_, _configMap_, _secret_, and _CSI_ volume types
are supported. Users can configure any number of volumes for each of
mentioned types.

Here is an example configuration:

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

### Host Path volumes

[_HostPath_ volume](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath) configuration instructs Kubernetes to mount
a specified host path inside of the container. The volume can be configured with
following options:

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes    | The name of the volume. |
| `mount_path` | string    | Yes    | Path inside of container where the volume should be mounted. |
| `sub_path`   | string    | No     | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `host_path`  | string    | No     | Host's path that should be mounted as volume. If not specified then set to the same path as `mount_path`. |
| `read_only`  | boolean   | No     | Sets the volume in read-only mode (defaults to false). |

### PVC volumes

[_PVC_ volume](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim) configuration instructs Kubernetes to use a _PersistentVolumeClaim_
that is defined in Kubernetes cluster and mount it inside of the container. The volume
can be configured with following options:

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | The name of the volume and at the same time the name of _PersistentVolumeClaim_ that should be used. |
| `mount_path` | string    | Yes      | Path inside of container where the volume should be mounted. |
| `read_only`  | boolean   | No       | Sets the volume in read-only mode (defaults to false). |
| `sub_path`   | string    | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |

### ConfigMap volumes

_ConfigMap_ volume configuration instructs Kubernetes to use a [_configMap_](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/)
that is defined in Kubernetes cluster and mount it inside of the container.

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | The name of the volume and at the same time the name of _configMap_ that should be used. |
| `mount_path` | string    | Yes      | Path inside of container where the volume should be mounted. |
| `read_only`  | boolean   | No       | Sets the volume in read-only mode (defaults to false). |
| `sub_path`   | string    | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `items`      | `map[string]string` | no   | Key-to-path mapping for keys from the _configMap_ that should be used. |

When using _configMap_ volume, each key from selected _configMap_ will be changed into a file
stored inside of the selected mount path. By default all keys are present, _configMap's_ key
is used as file's name and value is stored as file's content. The default behavior can be
changed with `items` option.

`items` option is defining a mapping between key that should be used and path (relative
to volume's mount path) where _configMap's_ value should be saved. When using `items` option
**only selected keys** will be added to the volumes and all other will be skipped.

> **Notice**: If a non-existing key will be used then job will fail on Pod creation stage.

### Secret volumes

[_Secret_ volume](https://kubernetes.io/docs/concepts/storage/volumes/#secret) configuration instructs Kubernetes to use
a _secret_ that is defined in Kubernetes cluster and mount it inside of the container.

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | The name of the volume and at the same time the name of _secret_ that should be used. |
| `mount_path` | string    | Yes      | Path inside of container where the volume should be mounted. |
| `read_only`  | boolean   | No       | Sets the volume in read-only mode (defaults to false). |
| `sub_path`   | string    | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `items`      | `map[string]string` | No   | Key-to-path mapping for keys from the _configMap_ that should be used. |

When using _secret_ volume each key from selected _secret_ will be changed into a file
stored inside of the selected mount path. By default all keys are present, _secret's_ key
is used as file's name and value is stored as file's content. The default behavior can be
changed with `items` option.

`items` option is defining a mapping between key that should be used and path (relative
to volume's mount path) where _secret's_ value should be saved. When using `items` option
**only selected keys** will be added to the volumes and all other will be skipped.

> **Notice**: If a non-existing key will be used then job will fail on Pod creation stage.

### Empty Dir volumes

[_emptyDir_ volume](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) configuration instructs Kubernetes to mount an empty directory inside of the container.

| Option       | Type    | Required | Description |
|--------------|---------|----------|-------------|
| `name`       | string  | Yes      | The name of the volume. |
| `mount_path` | string  | Yes      | Path inside of container where the volume should be mounted. |
| `sub_path`   | string  | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `medium`     | string  | No       | "Memory" will provide a tmpfs, otherwise it defaults to the node disk storage (defaults to ""). |

### CSI volumes

[_CSI_ volume](https://kubernetes.io/docs/concepts/storage/volumes/#csi) configuration instructs Kubernetes to use a custom CSI driver to mount an arbitrary storage system inside of the container.

| Option              | Type                | Required | Description |
|---------------------|---------------------|----------|-------------|
| `name`              | string              | Yes      | The name of the volume. |
| `mount_path`        | string              | Yes      | Path inside of container where the volume should be mounted. |
| `driver`            | string              | Yes      | A string value that specifies the name of the volume driver to use. |
| `fs_type`           | string              | No       | A string value that specifies the name of the filesystem type (Ex. "ext4", "xfs", "ntfs".). |
| `volume_attributes` | `map[string]string` | No       | Key-value pair mapping for attributes of the CSI volume. |
| `sub_path`          | string              | No       | Mount a [sub-path](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath) within the volume instead of the root. |
| `read_only`         | boolean             | No       | Sets the volume in read-only mode (defaults to false). |

## Custom builds directory mount

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2862) in GitLab Runner 13.12.

To store the builds directory for the job, define custom volume mounts to the
configured `builds_dir` (`/builds` by default).
To use [PVC volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/),
be aware that depending on the
[access mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes),
you might be limited to running jobs on 1 node.

Here is an example configuration:

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

## Using Security Context

[Pod security context](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) configuration instructs executor to set a pod security policy on the build pod.

| Option               | Type       | Required | Description |
|----------------------|------------|----------|-------------|
| `fs_group`           | `int`      | No       | A special supplemental group that applies to all containers in a pod. |
| `run_as_group`       | `int`      | No       | The GID to run the entry point of the container process. |
| `run_as_non_root`    | boolean    | No       | Indicates that the container must run as a non-root user. |
| `run_as_user`        | `int`      | No       | The UID to run the entry point of the container process. |
| `supplemental_groups`| `int` list | No       | A list of groups applied to the first process run in each container, in addition to the container's primary GID. |

Assigning a security context to pods provides security to your Kubernetes cluster. For this to work you'll need to provide a helper
image that conforms to the policy you set here.

NOTE:
If no additional requirements other than `nonroot` environment is required, the [GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421) and [GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433) images can be used in place of building own helper image.
One caveat is that the images are designed so that they can work with any user ID. It's important that this user ID is part of the root group.
Being part of the root group doesn't give it any specific privileges.

[Read more about the helper image](../configuration/advanced-configuration.md#helper-image).
Example of building your own helper image:

```Dockerfile
ARG tag
FROM gitlab/gitlab-runner-helper:${tag}
RUN addgroup -g 59417 -S nonroot && \
    adduser -u 59417 -S nonroot -G nonroot
WORKDIR /home/nonroot
USER 59417:59417
```

This example creates a user and group called `nonroot` and sets the image to run as that user.

Example of setting pod security context in your `config.toml`:

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

### Container security context

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/3116) in GitLab Runner 14.5.

Use the [container security context](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) configuration to configure the executor to set a container security policy on the build, helper, or service pods.

| Option                | Type        | Required | Description |
|-----------------------|-------------|----------|-------------|
| `run_as_group`        | int         | No       | The GID to run the entry point of the container process. |
| `run_as_non_root`     | boolean     | No       | Indicates that the container must run as a non-root user. |
| `run_as_user`         | int         | No       | The UID to run the entry point of the container process. |
| `capabilities.add`    | string list | No       | The capabilities to add when running the container. |
| `capabilities.drop`   | string list | No       | The capabilities to drop when running the container. |

The example below sets a pod security context, then overrides `run_as_user` and `run_as_group` for both the build and helper containers. In the example, all service containers would inherit `run_as_user` and `run_as_group` from the pod security context.

Example of setting pod security context in your `config.toml`:

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

## Using services

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4470) in GitLab Runner 12.5.
> - [Introduced support for `alias`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4829) in GitLab Runner 12.9
> - [Introduced support for `command` and `entrypoint`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173) in GitLab Runner 13.6.

Define a list of [services](https://docs.gitlab.com/ee/ci/services/).

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

## Using pull policies

Use the `pull_policy` parameter to specify a single or multiple pull policies.

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

The policy applies to the build image, helper image, and any services. It controls how an image is fetched and updated.

To determine when to use which policy, see
[the documentation on Kubernetes pull policies](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy).

The GitLab naming convention is slightly different than the Kubernetes one.

| Runner pull policy | Kubernetes pull policy | Description |
|--------------------|------------------------|-------------|
| _blank_ | _blank_ | Uses the default policy, as specified by Kubernetes. |
| `if-not-present` | `IfNotPresent` | The image is pulled only if it is not already present on the node that executes the job. See [security considerations](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy). |
| `always` | `Always` | The image is pulled every time the job is executed. |
| `never` | `Never` | The image is never pulled and requires the node to already have it. |

When multiple policies are defined, each policy is attempted until the image is obtained successfully.
For example, when you use `[ always, if-not-present ]`, you can use the policy `if-not-present` if `always` fails due to a temporary registry problem.

Beware of the [security considerations](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy) when using `if-not-present`.

## Using node selectors

The `node_selector` option can be used to specify which node in a Kubernetes cluster you want builds to be executed on.
It is a table of `key=value` pairs in the format of `string=string` (`string:string` in the case of environment variables).

Runner additionally uses the information provided to determine the OS and architecture for the build. This ensures that
the correct [helper image](../configuration/advanced-configuration.md#helper-image) is used. By default, the OS and
architecture is assumed to be `linux/amd64`.

By using certain labels, builds can be scheduled on nodes with different operating systems and architectures:

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

Kubernetes for Windows has certain [limitations](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/#supported-functionality-and-limitations),
so if process-isolation is used, you must additionally provide the specific windows build version with the
[`node.kubernetes.io/windows-build`](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesiowindows-build) label.

```toml
  [[runners]]
    name = "myRunner"
    url = "gitlab.example.com"
    executor = "kubernetes"

    # The FF_USE_POWERSHELL_PATH_RESOLVER feature flag has to be enabled for PowerShell
    # to resolve paths for Windows correctly when Runner is operating in a Linux environment
    # but targeting Windows nodes.
    environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

    [runners.kubernetes.node_selector]
      "kubernetes.io/arch" = "amd64"
      "kubernetes.io/os" = "windows"
      "node.kubernetes.io/windows-build" = "10.0.19041"
```

## Adding extra host aliases

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2818) in GitLab Runner 13.7.

This feature is available in Kubernetes 1.7+.

[Host aliases](https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/) configuration
instructs Kubernetes to add entries to `/etc/hosts` file inside of the container. The host aliases can be configured with the following options:

| Option       | Type          | Required | Description |
|--------------|---------------|----------|-------------|
| `IP`         | string        | Yes      | The IP address you want to attach hosts too. |
| `Hostnames`  | `string` list | Yes      | A list of host name aliases that will be attached to the IP. |

Here is an example configuration:

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

## Using Affinity

### Node Affinity

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2324) in GitLab Runner 13.4.

Define a list of [node affinities](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) to be added to a pod specification at build time.

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

### Pod affinity and anti-affinity

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2324) in GitLab Runner 14.3.

Use these affinities to constrain which nodes [your pod is eligible](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) to be scheduled on based on the labels on other pods.

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

## Container lifecycle hooks

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3630) in GitLab Runner 14.2.

Use [container lifecycle hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/) to run code configured for a handler when the corresponding lifecycle hook is executed.
You can configure two types of hooks: `PreStop` and `PostStart`. Each of them allows only one type of handler to be set.

Here is an example configuration:

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

Each lifecycle hook is configured through the following settings:

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

## Pod's DNS Config

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6562) in GitLab Runner 13.7.

[Pod's DNS Config](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-dns-config) gives users more control on the DNS settings of the created build Pods.

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

| Option       | Type                        | Required | Description |
|--------------|-----------------------------|----------|-------------|
| `nameservers`| `string` list               | No       | A list of IP addresses that will be used as DNS servers for the pod. |
| `options`    | `KubernetesDNSConfigOption` | No       | A optional list of objects where each object may have a name property (required) and a value property (optional). |
| `searches`   | `string` lists               | No       | A list of DNS search domains for hostname lookup in the pod. |

### KubernetesDNSConfigOption

| Option       | Type      | Required | Description |
|--------------|-----------|----------|-------------|
| `name`       | string    | Yes      | Configuration option name.  |
| `value`      | `*string` | No       | Configuration option value. |

## Capabilities configuration

[Kubernetes allows](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container)
to configure different [Linux capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html) that should be
added or dropped from a container.

GitLab Runner supports configuration of capabilities with the `cap_add` and `cap_drop` settings in the `[runners.kubernetes]`
section of the configuration file. By default, GitLab Runner provides
a [list of capabilities](#default-list-of-dropped-capabilities) that should be dropped.

Because the default list of dropped capabilities can intersect with user-defined capabilities, the following rules are
applied to determine the final list:

1. User-defined `cap_drop` has priority over user-defined `cap_add`. If
   you define the same capability in both settings, only the one from `cap_drop` is passed
   to the container.

1. User-defined `cap_add` has priority over the default list of dropped capabilities.
   If you want to add the capability that is dropped by default, explicitly add it to
   `cap_add`.

The final list of capabilities is added to all containers in the job's pod.

A few notes:

- Remove the `CAP_` prefix from capability identifiers passed to the container configuration.
  For example, if you want to add or drop the `CAP_SYS_TIME` capability,
  in the configuration file, set a `SYS_TIME` string for `cap_add` or `cap_drop`.

- The owner of the Kubernetes cluster
  [can define a PodSecurityPolicy](https://kubernetes.io/docs/concepts/policy/pod-security-policy/#capabilities),
  where specific capabilities are allowed, restricted, or added by default.
  These rules take precedence over any user-defined configuration.

  Container runtimes can also define a default list of capabilities that you can add,
  like those seen in
  [Docker](https://github.com/moby/moby/blob/19.03/oci/defaults.go#L14-L32)
  or [containerd](https://github.com/containerd/containerd/blob/v1.4.0/oci/spec.go#L93-L110).

### Default list of dropped capabilities

GitLab Runner tries to drop these capabilities by default. If any of them are required for the job
to be executed properly, you should explicitly add the capability with the `cap_add` setting:

<!-- kubernetes_default_cap_drop_list_start -->
- `NET_RAW`

<!-- kubernetes_default_cap_drop_list_end -->

### Example configuration

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

## Using Docker in your builds

There are a couple of caveats when using Docker in your builds while running on
a Kubernetes cluster. Most of these issues are already discussed in the
[**Using Docker Build**](https://docs.gitlab.com/ee/ci/docker/using_docker_build.html)
section of the GitLab CI
documentation but it is worthwhile to revisit them here as you might run into
some slightly different things when running this on your cluster.

### Exposing `/var/run/docker.sock`

Exposing your host's `/var/run/docker.sock` into your build container, using the
`runners.kubernetes.volumes.host_path` option, brings the same risks with it as
always. That node's containers are accessible from the build container and
depending if you are running builds in the same cluster as your production
containers it might not be wise to do that.

### Using `docker:dind`

Running the `docker:dind` also known as the `docker-in-docker` image is also
possible but sadly needs the containers to be run in privileged mode.
If you're willing to take that risk other problems will arise that might not
seem as straight forward at first glance. Because the Docker daemon is started
as a `service` usually in your `.gitlab-ci.yaml` it will be run as a separate
container in your Pod. Basically containers in Pods only share volumes assigned
to them and an IP address by which they can reach each other using `localhost`.
`/var/run/docker.sock` is not shared by the `docker:dind` container and the `docker`
binary tries to use it by default.

To overwrite this and make the client use TCP to contact the Docker daemon,
in the other container, be sure to include the environment variables of
the build container:

- `DOCKER_HOST=tcp://<hostname>:2375` for no TLS connection.
- `DOCKER_HOST=tcp://<hostname>:2376` for TLS connection.

If you are using GitLab Runner 12.7 or earlier and Kubernetes 1.6 or earlier, the value of
`hostname` must be set to `localhost`. Otherwise it should be set to `docker`.

Make sure to configure those properly. As of Docker 19.03, TLS is enabled by
default but it requires mapping
certificates to your client. You can enable non-TLS connection for DIND or
mount certificates as described in
[**Use Docker In Docker Workflow with Docker executor**](https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#use-docker-in-docker-workflow-with-docker-executor)

### Resource separation

In both the `docker:dind` and `/var/run/docker.sock` cases the Docker daemon
has access to the underlying kernel of the host machine. This means that any
`limits` that had been set in the Pod will not work when building Docker images.
The Docker daemon will report the full capacity of the node regardless of
the limits imposed on the Docker build containers spawned by Kubernetes.

One way to help minimize the exposure of the host's kernel to any build container
when running in privileged mode or by exposing `/var/run/docker.sock` is to use
the `node_selector` option to set one or more labels that have to match a node
before any containers are deployed to it. For example build containers may only run
on nodes that are labeled with `role=ci` while running all other production services
on other nodes. Further separation of build containers can be achieved using node
[taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).
This will disallow other pods from scheduling on the same nodes as the
build pods without extra configuration for the other pods.

### Using kaniko

Another approach for building Docker images inside a Kubernetes cluster is using [kaniko](https://github.com/GoogleContainerTools/kaniko).
kaniko:

- Allows you to build images without privileged access.
- Works without the Docker daemon.

For more information, see [Building images with kaniko and GitLab CI/CD](https://docs.gitlab.com/ee/ci/docker/using_kaniko.html).

## Job execution

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1775) in GitLab Runner 12.9.
> - [Behind a feature flag `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`](../configuration/feature-flags.md#available-feature-flags), enabled by default.
> - [Using attach by default](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/10341) in GitLab Runner 14.0.

GitLab Runner uses `kube attach` instead of `kube exec` by default. This should avoid problems like when a [job is marked successful midway](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119)
in environments with an unstable network.

If you experience problems after updating to GitLab Runner 14.0, toggle the feature flag [`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`](../configuration/feature-flags.md#available-feature-flags)
to `true` and [submit an issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/new).

Follow [issue #27976](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27976) for progress on legacy execution strategy removal.

### Container entrypoint

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3095) in GitLab Runner 14.5.

GitLab Runner honors the entrypoint defined in a Docker image when used with the Kubernetes executor with `kube attach`.

NOTE:
If an entrypoint is defined in the Dockerfile for an image, it must open a valid shell. Otherwise, the CI job hangs.

## Pod cleanup

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27870) in GitLab Runner 14.6.

Sometimes old runner pods are not cleared. This can happen when Runner Manager is incorrectly shut down.

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

### `request did not complete within requested timeout`

The message `request did not complete within requested timeout` observed during build pod creation indicates that a configured [admission control webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) on the Kubernetes cluster is timing out.

Admission control webhooks are a cluster-level administrative control intercept for all API requests they're scoped for, and can cause failures if they do not execute in time.

Admission control webhooks support filters that can finely control which API requests and namespace sources it intercepts. If the Kubernetes API calls from GitLab Runner do not need to pass through an admission control webhook then you may alter the [webhook's selector/filter configuration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector) to ignore the GitLab Runner namespace, or apply exclusion labels/annotations over the GitLab Runner pod by configuring `podAnnotations` or `podLabels` in the [GitLab Runner Helm Chart `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500).

For example, to avoid [DataDog Admission Controller webhook](https://docs.datadoghq.com/agent/cluster_agent/admission_controller/) from intercepting API requests made by the GitLab Runner manager pod, the following can be added:

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
