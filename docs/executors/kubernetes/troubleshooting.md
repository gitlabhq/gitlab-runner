---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Troubleshooting the Kubernetes executor
---

The following errors are commonly encountered when using the Kubernetes executor.

## `Job failed (system failure): timed out waiting for pod to start`

If the cluster cannot schedule the build pod before the timeout defined by `poll_timeout`, the build pod returns an error. The [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime) should be able to delete it.

To fix this issue, increase the `poll_timeout` value in your `config.toml` file.

## `context deadline exceeded`

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

## `Dial tcp xxx.xx.x.x:xxx: i/o timeout`

This is a Kubernetes error that generally indicates the Kubernetes API server is unreachable by the runner manager.
To resolve this issue:

- If you use network security policies, grant access to the Kubernetes API, typically on port 443 or port 6443, or both.
- Ensure that the Kubernetes API is running.

## Connection refused when attempting to communicate with the Kubernetes API

When GitLab Runner makes a request to the Kubernetes API and it fails,
it is likely because
[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)
is overloaded and can't accept or process API requests.

## `Error cleaning up pod` and `Job failed (system failure): prepare environment: waiting for pod running`

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
[`poll_timeout`](_index.md#other-configtoml-settings) setting.

To better understand how pods are scheduled or why they might not get scheduled
on time, [read about the Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).

## `request did not complete within requested timeout`

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

## Error `Could not resolve host: example.com`

If using the `alpine` flavor of the [helper image](../../configuration/advanced-configuration.md#helper-image),
there can be [DNS issues](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129) related to Alpine's `musl`'s DNS resolver.
The error might look similar to:

- `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

Use the `helper_image_flavor = "ubuntu"` option to resolve this issue.

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?`

This error can occur when [using Docker-in-Docker](_index.md#using-dockerdind) if attempts are made to access the DIND service before it has had time to fully start up. For a more detailed explanation, see [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215).

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443`

This error can happen when [using Docker-in-Docker](_index.md#using-dockerdind) if the DIND Maximum Transmission Unit (MTU) is larger than the Kubernetes overlay network. DIND uses a default MTU of 1500, which is too large to route across the default overlay network. The DIND MTU can be changed within the service definition:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows`

When you run your CI/CD job, you might receive an error like the following:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

This issue occurs when you [use node selectors](_index.md#specify-the-node-to-execute-builds) to run builds on nodes with different operating systems and architectures.

To fix the issue, configure `nodeSelector` so that the runner manager pod is always scheduled on a Linux node. For example, your [`values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) should contain the following:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## Build pods are assigned the worker node's IAM role instead of Runner IAM role

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

## Error: `pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies`

This issue happens if you specified a `pull_policy` in your `.gitlab-ci.yml` but there is no policy
configured in the Runner's configuration file. The error might look similar to:

- `Preparation failed: invalid pull policy for image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

To fix this issue, add `allowed_pull_policies` to your configuration according to
[restrict Docker pull policies](_index.md#restrict-docker-pull-policies).

## Background processes cause jobs to hang and timeout

Background processes started during job execution can [prevent the build job from exiting](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880). To avoid this you can:

- Double fork the process. For example, `command_to_run < /dev/null &> /dev/null &`.
- Kill the process before exiting the job script.

## Cache-related `permission denied` errors

Files and folders that are generated in your job have certain UNIX ownerships and permissions.
When your files and folders are archived or extracted, UNIX details are retained.
However, the files and folders may mismatch with the `USER` configurations of
[helper images](../../configuration/advanced-configuration.md#helper-image).

If you encounter permission-related errors in the `Creating cache ...` step,
you can:

- As a solution, investigate whether the source data is modified,
  for example in the job script that creates the cached files.
- As a workaround, add matching [chown](https://linux.die.net/man/1/chown) and
  [chmod](https://linux.die.net/man/1/chmod) commands.
  to your [(`before_`/`after_`)`script:` directives](https://docs.gitlab.com/ci/yaml/#default).

## Apparently redundant shell process in build container with init system

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

## Runner pod fails to run job results and times out despite successful registration

After the runner pod registers with GitLab, it attempts to run a job but does not and the job eventually times out. The following errors are reported:

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

In this case, the runner might receive the error,

```plaintext
HTTP 204 No content response code when connecting to the `jobs/request` API.
```

To troubleshoot this issue, manually send a POST request to the API to
validate if the TCP connection is hanging. If the TCP connection is hanging,
the runner might not be able to request CI job payloads.

## `failed to reserve container name` for init-permissions container when `gcs-fuse-csi-driver` is used

The `gcs-fuse-csi-driver` `csi` driver [does not support mounting volumes for the init container](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38). This can cause failures starting the init container when using this driver. Features [introduced in Kubernetes 1.28](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/) must be supported in the driver's project to resolve this bug.

## Error: `only read-only root filesystem container is allowed`

In clusters with admission policies that force containers to run on read-only mounted root filesystems,
this error might appear when:

- You install GitLab Runner.
- GitLab Runner tries to schedule a build pod.

These admission policies are usually enforced by an admission controller like
[Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/) or [Kyverno](https://kyverno.io/).
For example, a policy forcing containers to run on read-only root filesystems is
the [`readOnlyRootFilesystem`](https://open-policy-agent.github.io/gatekeeper-library/website/validation/read-only-root-filesystem/) Gatekeeper policy.

To resolve this issue:

- All pods that are deployed to the cluster must adhere to the admission policies by setting
  `securityContext.readOnlyRootFilesystem` to `true` for their containers so the
  admission controller does not block the pod.
- The containers must run successfully and be able to write to the filesystem
  even though the root file system is mounted read-only.

### For GitLab Runner

If GitLab Runner is deployed with the [GitLab Runner Helm chart](../../install/kubernetes.md),
you must update the GitLab chart configuration to have:

- A proper `securityContext` value:

  ```yaml
  <...>
  securityContext:
    readOnlyRootFilesystem: true
  <...>
  ```

- A writable file system mounted where the pod can write:

  ```yaml
  <...>
  volumeMounts:
  - name: tmp-dir
    mountPath: /tmp
  volumes:
  - name: tmp-dir
    emptyDir:
      medium: "Memory"
  <...>
  ```

### For the build pod

To make the build pod run on a read-only root file system,
configure the different containers' security contexts in `config.toml`.
You can set the GitLab chart variable `runners.config`, which is passed to the build pod:

```yaml
runners:
  config: |
   <...>
   [[runners]]
     [runners.kubernetes.build_container_security_context]
       read_only_root_filesystem = true
     [runners.kubernetes.init_permissions_container_security_context]
       read_only_root_filesystem = true
     [runners.kubernetes.helper_container_security_context,omitempty]
       read_only_root_filesystem = true
     # This section is only needed if jobs with services are used
     [runners.kubernetes.service_container_security_context,omitempty]
       read_only_root_filesystem = true
   <...>
```

To make the build pod and its containers run successfully on a read-only
file system, you must have writable filesystems in locations where the build pod can write.
At a minimum, these locations are the build and home directories.
Ensure the build process has write access to other locations if necessary.

The home directory must generally be writable so programs can store
their configuration and other data they need for successful execution.
The `git` binary is one example of a program that expects to be able to
write to the home directory.

To make the home directory writable regardless of its path in different
container images:

1. Mount a volume on a stable path (regardless of which build image you use).
1. Change the home directory by setting the environment variable `$HOME` globally for all builds.

You can configure the build pod and its containers in `config.toml` by
updating the value of the GitLab chart variable `runners.config`.

```yaml
runners:
  config: |
   <...>
   [[runners]]
     environment = ["HOME=/build_home"]
     [[runners.kubernetes.volumes.empty_dir]]
       name = "repo"
       mount_path = "/builds"
     [[runners.kubernetes.volumes.empty_dir]]
       name = "build-home"
       mount_path = "/build_home"
   <...>
```

{{< alert type="note" >}}

Instead of `emptyDir`, you can use any other
[supported volume types](_index.md#configure-volume-types).
Because all files that are not explicitly handled and stored as build
artefacts are usually ephemeral, `emptyDir` works for most cases.

{{< /alert >}}

## AWS EKS: Error cleaning up pod: pods "runner-**" not found or status is "Failed"

The Amazon EKS zone rebalancing feature balances the availability zones in an autoscaling group. This feature might stop a node in one availability zone and create it in another.

Runner jobs cannot be stopped and moved to another node. Disable this feature for runner jobs to resolve this error.

## Services not supported with Windows containers

When attempting to use [services](https://docs.gitlab.com/ci/services/) on Windows nodes,
they might fail with the following error:

- `ERROR: Job failed (system failure): prepare environment: admission webhook "windows.common-webhooks.networking.gke.io" denied the request: spec.hostAliases: Invalid value: []v1.HostAlias{v1.HostAlias{IP:"127.0.0.1", Hostnames:[]string{"<your windows image>"}}}: Windows does not support this field.`

Depending on the Kubernetes runtime, the error could either be reported or silently ignored.
For example, GKE does report the error.

Services are implemented using `hostAlias` in Kubernetes executor, which is not supported in Windows containers.
