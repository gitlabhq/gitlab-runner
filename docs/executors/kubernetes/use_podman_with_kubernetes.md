---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Use Podman with GitLab Runner on Kubernetes
---

Podman is an open-source [Open Container Initiative](https://opencontainers.org/) (OCI) tool for developing, managing, and running containers.

Podman provides configurations that let you build container images in a CI job, without a root user or [privileged](../../security/_index.md#usage-of-docker-executor) escalation on the host.

This document covers information about how to configure Podman to use it with GitLab Runner on OpenShift and non-OpenShift Kubernetes clusters.
The configuration applies to container images set as a root and non-root user.

## Run Podman on non-OpenShift Kubernetes cluster

### Run Podman as a non-root user with the `--privileged` flag set to `true`

> [!warning]
> When you run Podman with the `--privileged` flag set to `true`, the container engine launches the container with or without any additional security controls.

To run Podman as a non-root user with non-root container processes:

1. Create a container image with Podman using the following sample code in your `.gitlab-ci.yml` file:

   ```yaml
   variables:
     FF_USE_POWERSHELL_PATH_RESOLVER: "true"
     FF_RETRIEVE_POD_WARNING_EVENTS: "true"
     FF_PRINT_POD_EVENTS: "true"
     FF_SCRIPT_SECTIONS: "true"
     CI_DEBUG_SERVICES: "true"
     GIT_DEPTH: 5
     HOME: /my_custom_dir
     DOCKER_HOST: tcp://docker:2375

   podman-privileged-test:
     image: quay.io/podman/stable
     before_script:
       - podman info
       - id
     script:
       - podman build . -t playground-bis:testing
   ```

1. Set the default `user_id` to `1000` by adding the following configurations to your `config.toml` file:

   ```ini
       [runners.kubernetes.pod_security_context]
         run_as_user = 1000
       [runners.kubernetes.build_container_security_context]
         run_as_user = 1000
   ```

1. Add the following runner configurations to your `config.toml` file:

   ```toml
   listen_address = ":9252"
   concurrent = 3
   check_interval = 1
   log_level = "debug"
   log_format = "runner"
   connection_max_age = "15m0s"
   shutdown_timeout = 0

   [session_server]
     session_timeout = 1800

   [[runners]]
     name = "investigation"
     limit = 50
     url = "https://gitlab.com/"
     id = 0
     token = "glrt-REDACTED"
     token_obtained_at = 2024-09-30T14:38:04.623237Z
     executor = "kubernetes"
     builds_dir = "/my_custom_dir"
     shell = "bash"
     [runners.kubernetes]
       host = ""
       bearer_token_overwrite_allowed = false
       image = ""
       namespace = ""
       namespace_overwrite_allowed = ""
       namespace_per_job = false
       privileged = true
       node_selector_overwrite_allowed = ".*"
       node_tolerations_overwrite_allowed = ""
       pod_labels_overwrite_allowed = ""
       service_account_overwrite_allowed = ""
       pod_annotations_overwrite_allowed = ""
       [runners.kubernetes.pod_labels]
         user = "ratchade"
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "repo"
           mount_path = "/my_custom_dir"
       [runners.kubernetes.pod_security_context]
         run_as_user = 1000
       [runners.kubernetes.build_container_security_context]
         run_as_user = 1000
       [[runners.kubernetes.services]]
         name = ""
       [runners.kubernetes.dns_config]
   ```

If the jobs pass as expected, the job log should look like in the following example:

```shell
...

$ podman build . -t playground-bis:testing
STEP 1/6: FROM docker.io/library/golang:1.24.4 AS builder
Trying to pull docker.io/library/golang:1.24.4...
Getting image source signatures
Copying blob sha256:6564e0d9b89ebe3e93013c7d7fbf4d560c5831ed61448167899654bf22c6dc59
Copying blob sha256:2b238499ec52e0d6be479f948c76ba0bc3cc282f612d5a6a4b5ef52ff45f6b2c
Copying blob sha256:6d11c181ebb38ef30f2681a42f02030bc6fdcfbe9d5248270ee065eb7302b500
Copying blob sha256:600c2555aee6a6bed84df8b8e456b2d705602757d42f5009a41b03abceff02f8
Copying blob sha256:41b754d079e82fafdf15447cfc188868092eaf1cf4a3f96c9d90ab1b7db91230
Copying blob sha256:a355a3cac949bed5cda9c62103ceb0f004727cedcd2a17d7c9836aea1a452fda
Copying blob sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1
Copying config sha256:723e5b94e776fd1a0d4e9bb860400f02acbe62cdac487f114f5bd6303d76fbd9
Writing manifest to image destination
STEP 2/6: WORKDIR "/workspace"
--> 32b9a99335a7
STEP 3/6: COPY . .
--> 3de77f571048
STEP 4/6: RUN go build -v main.go
internal/unsafeheader
internal/goarch
internal/cpu
internal/abi
internal/bytealg
internal/byteorder
internal/chacha8rand
internal/coverage/rtcov
internal/godebugs
internal/goexperiment
internal/goos
internal/profilerecord
internal/runtime/atomic
internal/runtime/syscall
internal/stringslite
internal/runtime/exithook
runtime/internal/math
runtime/internal/sys
cmp
internal/itoa
internal/race
runtime
math/bits
math
unicode/utf8
sync/atomic
unicode
internal/asan
internal/msan
internal/reflectlite
iter
sync
slices
errors
internal/bisect
strconv
io
internal/oserror
path
internal/godebug
syscall
reflect
time
io/fs
internal/filepathlite
internal/syscall/unix
internal/poll
internal/fmtsort
internal/syscall/execenv
internal/testlog
os
fmt
command-line-arguments
--> 6340b6cccaa9
STEP 5/6: RUN ls -halF
total 2.2M
drwxr-xr-x 1 root root 4.0K Oct  3 15:14 ./
dr-xr-xr-x 1 root root 4.0K Oct  3 15:14 ../
drwxrwxrwx 6 root root 4.0K Oct  3 15:14 .git/
-rw-rw-rw- 1 root root  690 Oct  3 15:14 .gitlab-ci.yml
-rw-rw-rw- 1 root root 1.8K Oct  3 15:14 Dockerfile
-rw-rw-rw- 1 root root   74 Oct  3 15:14 Dockerfile_multistage
-rw-rw-rw- 1 root root   18 Oct  3 15:14 README.md
-rw-rw-rw- 1 root root   51 Oct  3 15:14 go.mod
-rw-rw-rw- 1 root root  258 Oct  3 15:14 long-script-with-cleanup.sh
-rwxr-xr-x 1 root root 2.1M Oct  3 15:14 main*
-rw-rw-rw- 1 root root  157 Oct  3 15:14 main.go
-rw-rw-rw- 1 root root  333 Oct  3 15:14 string_output.sh
drwxrwxrwx 2 root root 4.0K Oct  3 15:14 test/
--> e3cce3e2b16a
STEP 6/6: CMD ["exec", "main"]
COMMIT playground-bis:testing
--> 2bf7283ee21d
Successfully tagged localhost/playground-bis:testing
2bf7283ee21dd86134fbda06a5835af4b68fe3dc6a3525b96587e14c40d7f1a3
Cleaning up project directory and file based variables
00:01
Job succeeded
```

### Run Podman as a root user with the `--privileged` flag set to `false`

Prerequisites:

- Permission to use `fuse-overlayfs` inside the container.

The following steps are inspired from the "Rootless Podman without the privileged flag" section
of [How to use Podman inside of Kubernetes](https://www.redhat.com/en/blog/podman-inside-kubernetes).

When running rootless Podman, you can remove the privileged flag by making a few adjustments
to your system configuration. The container needs access to `/dev/fuse` to use `fuse-overlayfs`
inside the container.

You must also disable SELinux on the host running the Kubernetes cluster.
SELinux prevents containerized processes from mounting the required file
systems inside a container.

To achieve this:

1. Create a device plugin that can be used by the job Pod, for example:

   ```yaml
   apiVersion: apps/v1
   kind: DaemonSet
   metadata:
     name: fuse-device-plugin-daemonset
     namespace: kube-system
   spec:
     selector:
       matchLabels:
         name: fuse-device-plugin-ds
     template:
       metadata:
         labels:
           name: fuse-device-plugin-ds
       spec:
         hostNetwork: true
         containers:
           - image: soolaugust/fuse-device-plugin:v1.0
             name: fuse-device-plugin-ctr
             securityContext:
               allowPrivilegeEscalation: false
               capabilities:
                 drop: ["ALL"]
             volumeMounts:
               - name: device-plugin
                 mountPath: /var/lib/kubelet/device-plugins
         volumes:
           - name: device-plugin
             hostPath:
               path: /var/lib/kubelet/device-plugins
   ```

1. Configure the `config.toml` to install GitLab Runner on the cluster.

   - Set the job Pod to run as a `root` user with the `--privileged` flag set to `false`:

     ```toml
     allow_privilege_escalation = false
     [runners.kubernetes.pod_security_context]
       run_as_non_root = false
     [runners.kubernetes.build_container_security_context]
       run_as_user = 0
       run_as_group = 0
     ```

   - Set a resource limit to the job Pod by using the [`pod_spec` feature](_index.md#overwrite-generated-pod-specifications).
     To use `pod_spec`, set the `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` feature flag to `true`.

     ```toml
     [[runners.kubernetes.pod_spec]]
       name = "device-fuse"
       patch_type = "strategic"
       patch = '''
         containers:
           - name: build
             resources:
               limits:
                 github.com/fuse: 1
       '''
     ```

   The `config.toml` should look similar to:

   ```toml
   [[runners]]
     [runners.kubernetes]
       host = ""
       bearer_token_overwrite_allowed = false
       pod_termination_grace_period_seconds = 0
       namespace = ""
       namespace_overwrite_allowed = ""
       pod_labels_overwrite_allowed = ""
       service_account_overwrite_allowed = ""
       pod_annotations_overwrite_allowed = ""
       node_selector_overwrite_allowed = ".*"
       allow_privilege_escalation = false
       [runners.kubernetes.pod_security_context]
         run_as_non_root = false
       [runners.kubernetes.build_container_security_context]
         run_as_user = 0
         run_as_group = 0
       [[runners.kubernetes.services]]
       [runners.kubernetes.dns_config]
       [runners.kubernetes.pod_labels]
       [[runners.kubernetes.pod_spec]]
         name = "device-fuse"
         patch_type = "strategic"
         patch = '''
           containers:
             - name: build
               resources:
                 limits:
                   github.com/fuse: 1
         '''
   ```

1. Run the job to build an image with Podman.

   ```yaml
   variables:
     FF_USE_POWERSHELL_PATH_RESOLVER: "true"
     FF_RETRIEVE_POD_WARNING_EVENTS: "true"
     FF_PRINT_POD_EVENTS: "true"
     FF_SCRIPT_SECTIONS: "true"
     CI_DEBUG_SERVICES: "true"
     GIT_DEPTH: 5
     FF_USE_ADVANCED_POD_SPEC_CONFIGURATION: "true"

   podman-privileged-test:
     image: quay.io/podman/stable
     before_script:
       - podman info
       - id
     script:
       - podman build . -t playground-bis:testing
   ```

The job runs `podman build`, which should complete successfully.

```shell
...

$ podman build . -t playground-bis:testing
time="2024-11-06T16:57:41Z" level=warning msg="Using cgroups-v1 which is deprecated in favor of cgroups-v2 with Podman v5 and will be removed in a future version. Set environment variable `PODMAN_IGNORE_CGROUPSV1_WARNING` to hide this warning."
time="2024-11-06T16:57:41Z" level=warning msg="Using cgroups-v1 which is deprecated in favor of cgroups-v2 with Podman v5 and will be removed in a future version. Set environment variable `PODMAN_IGNORE_CGROUPSV1_WARNING` to hide this warning."
STEP 1/6: FROM docker.io/library/golang:1.24.4 AS builder
Trying to pull docker.io/library/golang:1.24.4...
Getting image source signatures
Copying blob sha256:32d3574b34bd65a6cf89a80e5bd939574c7a9bd3efbaa4881292aaca16d3d0dc
Copying blob sha256:a47cff7f31e941e78bf63ca19f0811b675283e2c00ddea10c57f78d93b2bc343
Copying blob sha256:cdd62bf39133c498a16f7a7b1b6555ba43d02b2511c508fa4c0a9b1975ffe20e
Copying blob sha256:1eb015951d08f558e9805d427f6d30728b0cd94d5c9b9538cd4f7df57598664a
Copying blob sha256:a173f2aee8e962ea19db1e418ae84a0c9f71480b51f768a19332dfa83d7722a5
Copying blob sha256:e7bff916ab0c126c9d943f0c481a905f402e00f206a89248f257ef90beaabbd8
Copying blob sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1
Copying config sha256:8027d6b1a7f0702ed8a4174fd022be03f87e35c7a7fa00afb2bf4178b22080d4
Writing manifest to image destination
STEP 2/6: WORKDIR "/workspace"
--> 94b34d00b2cb
STEP 3/6: COPY . .
--> b807785fe549
STEP 4/6: RUN go build -v main.go
internal/goarch
internal/unsafeheader
internal/cpu
internal/abi
internal/bytealg
internal/byteorder
internal/chacha8rand
internal/coverage/rtcov
internal/godebugs
internal/goexperiment
internal/goos
internal/profilerecord
internal/runtime/atomic
internal/runtime/syscall
internal/runtime/exithook
internal/stringslite
runtime/internal/math
runtime/internal/sys
cmp
internal/itoa
internal/race
runtime
math/bits
math
unicode/utf8
sync/atomic
unicode
internal/asan
internal/msan
iter
internal/reflectlite
sync
slices
internal/bisect
errors
strconv
io
internal/oserror
path
internal/godebug
reflect
syscall
time
io/fs
internal/fmtsort
internal/filepathlite
internal/syscall/unix
internal/syscall/execenv
internal/testlog
internal/poll
os
fmt
command-line-arguments
--> 5c4fa8b22a3e
STEP 5/6: RUN ls -halF
total 2.1M
drwxr-xr-x  4 root root   18 Nov  6 16:58 ./
dr-xr-xr-x 19 root root    6 Nov  6 16:58 ../
drwxrwxrwx  6 root root  128 Nov  6 16:57 .git/
-rw-rw-rw-  1 root root  743 Nov  6 16:57 .gitlab-ci.yml
-rw-rw-rw-  1 root root 1.8K Nov  6 16:57 Dockerfile
-rw-rw-rw-  1 root root   74 Nov  6 16:57 Dockerfile_multistage
-rw-rw-rw-  1 root root   18 Nov  6 16:57 README.md
-rw-rw-rw-  1 root root   51 Nov  6 16:57 go.mod
-rw-rw-rw-  1 root root  258 Nov  6 16:57 long-script-with-cleanup.sh
-rwxr-xr-x  1 root root 2.1M Nov  6 16:58 main*
-rw-rw-rw-  1 root root  157 Nov  6 16:57 main.go
-rw-rw-rw-  1 root root  333 Nov  6 16:57 string_output.sh
drwxrwxrwx  2 root root   87 Nov  6 16:57 test/
--> 57bb3eb7e929
STEP 6/6: CMD ["exec", "main"]
COMMIT playground-bis:testing
--> 2cc55d032ba8
Successfully tagged localhost/playground-bis:testing
2cc55d032ba852e05c513e4067b55c10fd697c65e07ffe2aae104e8531702274
Cleaning up project directory and file based variables
00:00
Job succeeded
```

## Run Podman as a non-root user on OpenShift

To run rootless Podman without privileged containers, follow the steps in the RedHat article [Build container images in OpenShift using Podman as a GitLab Runner](https://developers.redhat.com/articles/2024/10/01/build-container-images-openshift-using-podman-gitlab-runner).

## Troubleshooting

### `git` cannot save the configuration in `/.gitconfig` when you run the job as a non-root user

Because you are not running the job as root, `git` cannot save the configuration in `/.gitconfig`. As a result, you might encounter the following error:

```shell
Getting source from Git repository
00:00
error: could not lock config file //.gitconfig: Permission denied
```

To prevent this error:

1. Mount an `emptyDir` volume on `/my_custom_dir`.
1. Set the `HOME` environment variable to the `/my_custom_dir` path.
