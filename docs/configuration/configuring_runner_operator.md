---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Configuring GitLab Runner on OpenShift

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, Self-managed

This document explains how to configure GitLab Runner on OpenShift.

## Passing properties to GitLab Runner Operator

When creating a `Runner`, you can configure it by setting properties in its `spec`.
For example, you can specify the GitLab URL where the runner is registered,
or the name of the secret that contains the registration token:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
 name: dev
spec:
 gitlabUrl: https://gitlab.example.com
 token: gitlab-runner-secret # Name of the secret containing the Runner token
```

Read about all the available properties in [Operator properties](#operator-properties).

## Operator properties

The following properties can be passed to the Operator.

Some properties are only available with more recent versions of the Operator.

| Setting          | Operator | Description                                                                                                                       |
|------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------|
| `gitlabUrl`      | all      | The fully qualified domain name for the GitLab instance, for example, `https://gitlab.example.com`.                               |
| `token`          | all      | Name of `Secret` containing the `runner-registration-token` key used to register the runner.                                      |
| `tags`           | all      | List of comma-separated tags to be applied to the runner.                                                                         |
| `concurrent`     | all      | Limits how many jobs can run concurrently. The maximum number is all defined runners. 0 does not mean unlimited. Default is `10`. |
| `interval`       | all      | Defines the number of seconds between checks for new jobs. Default is `30`.                                                       |
| `locked`         | 1.8      | Defines if the runner should be locked to a project. Default is `false`.                                                          |
| `runUntagged`    | 1.8      | Defines if jobs without tags should be run. Default is `true` if no tags were specified. Otherwise, it's `false`.                 |
| `protected`      | 1.8      | Defines if the runner should run jobs on protected branches only. Default is `false`.                                             |
| `cloneURL`       | all      | Overwrite the URL for the GitLab instance. Used only if the runner can't connect to the GitLab URL.                               |
| `env`            | all      | Name of `ConfigMap` containing key-value pairs that are injected as environment variables in the Runner pod.                      |
| `runnerImage`    | 1.7      | Overwrites the default GitLab Runner image. Default is the Runner image the operator was bundled with.                            |
| `helperImage`    | all      | Overwrites the default GitLab Runner helper image.                                                                                |
| `buildImage`     | all      | The default Docker image to use for builds when none is specified.                                                                |
| `cacheType`      | all      | Type of cache used for Runner artifacts. One of: `gcs`, `s3`, `azure`.                                                            |
| `cachePath`      | all      | Defines the cache path on the file system.                                                                                        |
| `cacheShared`    | all      | Enable sharing of cache between runners.                                                                                          |
| `s3`             | all      | Options used to set up S3 cache. Refer to [Cache properties](#cache-properties).                                                  |
| `gcs`            | all      | Options used to set up `gcs` cache. Refer to [Cache properties](#cache-properties).                                               |
| `azure`          | all      | Options used to set up Azure cache. Refer to [Cache properties](#cache-properties).                                               |
| `ca`             | all      | Name of TLS secret containing the custom certificate authority (CA) certificates.                                                 |
| `serviceaccount` | all      | Use to override service account used to run the Runner pod.                                                                       |
| `config`         | all      | Use to provide a custom `ConfigMap` with a [configuration template](../register/index.md#register-with-a-configuration-template). |

## Cache properties

### S3 cache

| Setting       | Operator | Description                                                                                                   |
|---------------|----------|---------------------------------------------------------------------------------------------------------------|
| `server`      | all      | The S3 server address.                                                                                        |
| `credentials` | all      | Name of the `Secret` containing the `accesskey` and `secretkey` properties used to access the object storage. |
| `bucket`      | all      | Name of the bucket in which the cache is stored.                                                              |
| `location`    | all      | Name of the S3 region in which the cache is stored.                                                              |
| `insecure`    | all      | Use insecure connections or `HTTP`.                                                                           |

### `gcs` cache

| Setting           | Operator | Description                                                                                                     |
|-------------------|----------|-----------------------------------------------------------------------------------------------------------------|
| `credentials`     | all      | Name of the `Secret` containing the `access-id` and `private-key` properties used to access the object storage. |
| `bucket`          | all      | Name of the bucket in which the cache is stored.                                                                |
| `credentialsFile` | all      | Takes the `gcs` credentials file, `keys.json`.                                                                  |

### Azure cache

| Setting         | Operator | Description                                                                                                      |
|-----------------|----------|------------------------------------------------------------------------------------------------------------------|
| `credentials`   | all      | Name of the `Secret` containing the `accountName` and `privateKey` properties used to access the object storage. |
| `container`     | all      | Name of the Azure container in which the cache is stored.                                                        |
| `storageDomain` | all      | The domain name of the Azure blob storage.                                                                       |

## Configure a proxy environment

To create a proxy environment:

1. Edit the `custom-env.yaml` file. For example:

   ```yaml
   apiVersion: v1
   data:
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
   ```

1. Update OpenShift to apply the changes.

   ```shell
   oc apply -f custom-env.yaml
   ```

1. Update your [`gitlab-runner.yml`](../install/operator.md#install-gitlab-runner) file.

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret # Name of the secret containing the Runner token
     env: custom-env
   ```

If the proxy can't reach the Kubernetes API, you might see an error in your CI/CD job:

```shell
ERROR: Job failed (system failure): prepare environment: setting up credentials: Post https://172.21.0.1:443/api/v1/namespaces/<KUBERNETES_NAMESPACE>/secrets: net/http: TLS handshake timeout. Check https://docs.gitlab.com/runner/shells/index.html#shell-profile-loading for more information
```

To resolve this error, add the IP address of the Kubernetes API to `NO_PROXY` configuration in the `custom-env.yaml` file:

```yaml
   apiVersion: v1
   data:
     NO_PROXY: 172.21.0.1
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
```

You can verify the IP address of the Kubernetes API by running:

```shell
oc get services --namespace default --field-selector='metadata.name=kubernetes' | grep -v NAME | awk '{print $3}'
```

## Customize `config.toml` with a configuration template

You can customize the runner's `config.toml` file by using the [configuration template](../register/index.md#register-with-a-configuration-template).

1. Create a custom configuration template file. For example, let's instruct our runner to mount an `EmptyDir` volume and
   set the `cpu_limit`. Create the `custom-config.toml` file:

   ```toml
   [[runners]]
     [runners.kubernetes]
       cpu_limit = "500m"
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "empty-dir"
           mount_path = "/path/to/empty_dir"
           medium = "Memory"
   ```

1. Create a `ConfigMap` named `custom-config-toml` from our `custom-config.toml` file:

   ```shell
    oc create configmap custom-config-toml --from-file config.toml=custom-config.toml
   ```

1. Set the `config` property of the `Runner`:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     config: custom-config-toml
   ```

Because of a [known issue](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/issues/229), you
must use environment variables instead of configuration templates to modify the following settings:

| Setting                             | Environment variable             | Default value |
|-------------------------------------|----------------------------------|---------------|
| `runners.request_concurrency`       | `RUNNER_REQUEST_CONCURRENCY`     | `1`           |
| `runners.output_limit`              | `RUNNER_OUTPUT_LIMIT`            | `4096`        |
| `kubernetes.runner.poll_timeout`    | `KUBERNETES_POLL_TIMEOUT`        | `180`         |

## Configure a custom TLS cert

1. To set a custom TLS cert, create a secret with key `tls.crt`. In this example, the file is named `custom-tls-ca-secret.yaml`:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
       name: custom-tls-ca
   type: Opaque
   stringData:
       tls.crt: |
           -----BEGIN CERTIFICATE-----
           MIIEczCCA1ugAwIBAgIBADANBgkqhkiG9w0BAQQFAD..AkGA1UEBhMCR0Ix
           .....
           7vQMfXdGsRrXNGRGnX+vWDZ3/zWI0joDtCkNnqEpVn..HoX
           -----END CERTIFICATE-----
   ```

1. Create the secret:

   ```shell
   oc apply -f custom-tls-ca-secret.yaml
   ```

1. Set the `ca` key in the `runner.yaml` to the same name as the name of our secret:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     ca: custom-tls-ca
   ```

## Configure the CPU and memory size of runner pods

To set [CPU limits](../executors/kubernetes/index.md#cpu-requests-and-limits) and [memory limits](../executors/kubernetes/index.md#memory-requests-and-limits) in a custom `config.toml` file, follow the instructions in [this topic](#customize-configtoml-with-a-configuration-template).

## Configure job concurrency per runner based on cluster resources

Set the `concurrent` property of the `Runner` resource:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  concurrent: 2
```

Job concurrency is dictated by the requirements of the project.

1. Start by trying to determine the compute and memory resources required to execute a CI job.
1. Calculate how many times that job would be able to execute given the resources in the cluster.

If you set a high concurrency value, the Kubernetes executor processes the jobs as soon as it can.
However, the Kubernetes cluster's scheduler capacity determines when the job is scheduled.

## Service account for the GitLab Runner manager

For a fresh installation, GitLab Runner creates a Kubernetes `ServiceAccount` named
`gitlab-runner-app-sa` for the runner manager pod if these RBAC role binding
resources don't exist:

- `gitlab-runner-app-rolebinding`
- `gitlab-runner-rolebinding`

If one of the role bindings exists, GitLab resolves the role and service account
from the `subjects` and `roleRef` defined in the role binding.

If both role bindings exist, `gitlab-runner-app-rolebinding` takes precedence over
`gitlab-runner-rolebinding`.

## Troubleshooting

### Root vs non-root

The GitLab Runner Operator and the GitLab Runner pod run as non-root users.
As a result, the build image used in the job must run as a non-root user to be able to complete successfully.
This ensures that jobs can run successfully with the least permission.

To make this work, make sure that the build image used for CI/CD jobs:

- Runs as non-root
- Does not write to restricted filesystems

Most container filesystems on an OpenShift cluster are read-only, except:

- Mounted volumes
- `/var/tmp`
- `/tmp`
- Other volumes mounted on root filesystems as `tmpfs`

#### Overriding the `HOME` environment variable

If creating a custom build image or [overriding environment variables](#configure-a-proxy-environment), ensure that the `HOME` environment variables is not set to `/` which would be read-only.
Especially if your jobs would need to write files to the home directory.
You could create a directory under `/home` for example `/home/ci` and set `ENV HOME=/home/ci` in your `Dockerfile`.

For the runner pods [it's expected that `HOME` would be set to `/home/gitlab-runner`](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L14).
If this variable is changed, the new location must have the [proper permissions](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L38).
These guidelines are also documented in the [Red Hat Container Platform documentation](https://docs.openshift.com/container-platform/4.7/openshift_images/create-images.html#support-arbitrary-user-ids).

### Overriding `locked` variable

When you register a runner token, if you set the `locked` variable to `true`, the error
`Runner configuration other than name, description, and exector is reserved and cannot be specified`
appears.

```yaml
  locked: true # REQUIRED
  tags: ""
  runUntagged: false
  protected: false
  maximumTimeout: 0
```

For more information, see [issue 472](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/472#note_1483346437).

#### Watch out for security context constraints

By default, when installed in a new OpenShift project, the GitLab Runner Operator runs as non-root.
Some projects, like the `default` project, are exceptions where all service accounts have `anyuid` access.
In that case, the user of the image is `root`. You can check this by running the `whoami` inside any container shell, for example, a job.
Read more about security context constraints in [Red Hat Container Platform documentation](https://docs.openshift.com/container-platform/4.7/authentication/managing-security-context-constraints.html).

#### Run as `anyuid` security context constraints

WARNING:
Running jobs as root or writing to root filesystems can expose your system to security risks.

To run a CI/CD job as the root user or write to root filesystems,
set the `anyuid` security context constraints on the `gitlab-runner-app-sa` service account.
The GitLab Runner container uses this service account.

In OpenShift 4.3.8 and earlier:

```shell
oc adm policy add-scc-to-user anyuid -z gitlab-runner-app-sa -n <runner_namespace>

# Check that the anyiud SCC is set:
oc get scc anyuid -o yaml
```

In OpenShift 4.3.8 and later:

```shell
oc create -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scc-anyuid
  namespace: <runner_namespace>
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - anyuid
  resources:
  - securitycontextconstraints
  verbs:
  - use
EOF

oc create -f - <<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: sa-to-scc-anyuid
  namespace: <runner_namespace>
subjects:
  - kind: ServiceAccount
    name: gitlab-runner-app-sa
roleRef:
  kind: Role
  name: scc-anyuid
  apiGroup: rbac.authorization.k8s.io
EOF
```

#### Configure SETFCAP

If you use Red Hat OpenShift Container Platform (RHOCP) 4.11 or later, you may get the following error message:

```shell
error reading allowed ID mappings:error reading subuid mappings for user
```

Some jobs (for example, `buildah`) need the `SETFCAP` capability granted to run correctly. To fix this issue:

1. Add the SETFCAP capability to the security context constraints that GitLab Runner is using (replace the `gitlab-scc` with the security context constraints assigned to your GitLab Runner pod):

   ```shell
   oc patch scc gitlab-scc --type merge -p '{"allowedCapabilities":["SETFCAP"]}'
   ```

1. Update your `config.toml` and add the `SETFCAP` capability under the `kubernetes` section:

   ```yaml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.pod_security_context]
       [runners.kubernetes.build_container_security_context]
       [runners.kubernetes.build_container_security_context.capabilities]
         add = ["SETFCAP"]
   ```

1. Create a `ConfigMap` using this `config.toml` in the namespace where GitLab Runner is deployed:

   ```shell
   oc create configmap custom-config-toml --from-file config.toml=config.toml
   ```

1. Modify the runner you want to fix, adding the `config:` parameter to point to the recently created `ConfigMap` (replace my-runner with the correct runner pod name).

   ```shell
   oc patch runner my-runner --type merge -p '{"spec": {"config": "custom-config-toml"}}'
   ```

For more information, see the [Red Hat documentation](https://access.redhat.com/solutions/7016013).

### Using FIPS Compliant GitLab Runner

NOTE:
For Operator, you can change only the helper image. You can't change the GitLab Runner image yet.
[Issue 28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814) tracks this feature.

To use a [FIPS compliant GitLab Runner helper](../install/index.md#fips-compliant-gitlab-runner), change the helper image as follows:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
 name: dev
spec:
 gitlabUrl: https://gitlab.example.com
 token: gitlab-runner-secret
 helperImage: gitlab/gitlab-runner-helper:ubi-fips
 concurrent: 2
```

#### Register GitLab Runner by using a self-signed certificate

To use self-signed certificate with GitLab self-managed, create a secret that
contains the CA certificate you used to sign the private certificates.

The name of the secret is then provided as the CA in the Runner spec section:

```yaml
KIND:     Runner
VERSION:  apps.gitlab.com/v1beta2

FIELD:    ca <string>

DESCRIPTION:
     Name of tls secret containing the custom certificate authority (CA)
     certificates
```

The secret can be created using the following command:

```shell
oc create secret generic mySecret --from-file=tls.crt=myCert.pem -o yaml
```

#### Register GitLab Runner with an external URL that points to an IP address

If the runner cannot match the self-signed certificate with the hostname, you might get an error message.
This issue occurs when you configure GitLab self-managed to use an IP address (like ###.##.##.##) instead of a hostname:

```shell
[31;1mERROR: Registering runner... failed               [0;m  [31;1mrunner[0;m=A5abcdEF [31;1mstatus[0;m=couldn't execute POST against https://###.##.##.##/api/v4/runners:
Post https://###.##.##.##/api/v4/runners: x509: cannot validate certificate for ###.##.##.## because it doesn't contain any IP SANs
[31;1mPANIC: Failed to register the runner. You may be having network problems.[0;m
```

To fix this issue:

1. On the GitLab self-managed server, modify the `openssl` to add the IP address to the `subjectAltName` parameter:

   ```shell
   # vim /etc/pki/tls/openssl.cnf

   [ v3_ca ]
   subjectAltName=IP:169.57.64.36 <---- Add this line. 169.57.64.36 is your GitLab server IP.
    ```

1. Then re-generate a self-signed CA with the commands below:

   ```shell
   # cd /etc/gitlab/ssl
   # openssl req -x509 -nodes -days 3650 -newkey rsa:4096 -keyout /etc/gitlab/ssl/169.57.64.36.key -out /etc/gitlab/ssl/169.57.64.36.crt
   # openssl dhparam -out /etc/gitlab/ssl/dhparam.pem 4096
   # gitlab-ctl restart
   ```

1. Use this new certificate to generate a new secret.
