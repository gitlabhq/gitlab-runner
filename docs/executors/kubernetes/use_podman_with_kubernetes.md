---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Use Podman with GitLab Runner on Kubernetes

Podman is an open-source [Open Container Initiative](https://opencontainers.org/) (OCI) tool for developing, managing, and running containers.

Podman provides configurations that let you build container images in a CI job, without a root user or [privileged](../../security/index.md#usage-of-docker-executor) escalation on the host.

You can configure Podman to use it with GitLab Runner on Kubernetes.

## Run Podman as a non-root user with privileges

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
Running with gitlab-runner development version (HEAD)
  on investigation REDACTED, system ID: s_b188029b2abb
  feature flags: FF_USE_POWERSHELL_PATH_RESOLVER:true, FF_SCRIPT_SECTIONS:true, FF_PRINT_POD_EVENTS:true
Preparing the "kubernetes" executor
00:00
WARNING: Namespace is empty, therefore assuming 'default'.
Using Kubernetes namespace: default
Using Kubernetes executor with image quay.io/podman/stable ...
Using attach strategy to execute scripts...
Preparing environment
00:03
Using FF_USE_POD_ACTIVE_DEADLINE_SECONDS, the Pod activeDeadlineSeconds will be set to the job timeout: 10m0s...
Subscribing to Kubernetes Pod events...
Type     Reason      Message
Normal   Scheduled   Successfully assigned default/runner-REDACTED-project-REDACTED-concurrent-0-l5aszavi to colima
Normal   Pulled   Container image "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest" already present on machine
Normal   Created   Created container init-permissions
Normal   Started   Started container init-permissions
Normal   Pulling   Pulling image "quay.io/podman/stable"
Normal   Pulled   Successfully pulled image "quay.io/podman/stable" in 429ms (429ms including waiting). Image size: 713244641 bytes.
Normal   Created   Created container build
Normal   Started   Started container build
Normal   Pulled   Container image "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest" already present on machine
Normal   Created   Created container helper
Normal   Started   Started container helper
Running on runner-REDACTED-project-REDACTED-concurrent-0-l5aszavi via ratchade-MBP...
Getting source from Git repository
00:02
Fetching changes with git depth set to 5...
Initialized empty Git repository in /my_custom_dir/ra-group2/playground-bis/.git/
Created fresh repository.
Checking out 433efc98 as detached HEAD (ref is dind-test)...
Skipping Git submodules setup
Executing "step_script" stage of the job script
00:14
$ podman info
host:
  arch: arm64
  buildahVersion: 1.37.3
  cgroupControllers:
  - cpuset
  - cpu
  - io
  - memory
  - hugetlb
  - pids
  - rdma
  - misc
  cgroupManager: cgroupfs
  cgroupVersion: v2
  conmon:
    package: conmon-2.1.12-2.fc40.aarch64
    path: /usr/bin/conmon
    version: 'conmon version 2.1.12, commit: '
  cpuUtilization:
    idlePercent: 98.22
    systemPercent: 0.77
    userPercent: 1.01
  cpus: 2
  databaseBackend: sqlite
  distribution:
    distribution: fedora
    variant: container
    version: "40"
  eventLogger: file
  freeLocks: 2048
  hostname: runner-idukxkzgd-project-25452826-concurrent-0-l5aszavi
  idMappings:
    gidmap:
    - container_id: 0
      host_id: 1000
      size: 1
    - container_id: 1
      host_id: 1
      size: 999
    - container_id: 1000
      host_id: 1001
      size: 64535
    uidmap:
    - container_id: 0
      host_id: 1000
      size: 1
    - container_id: 1
      host_id: 1
      size: 999
    - container_id: 1000
      host_id: 1001
      size: 64535
  kernel: 6.8.0-39-generic
  linkmode: dynamic
  logDriver: k8s-file
  memFree: 550629376
  memTotal: 2051272704
  networkBackend: netavark
  networkBackendInfo:
    backend: netavark
    dns:
      package: aardvark-dns-1.12.2-2.fc40.aarch64
      path: /usr/libexec/podman/aardvark-dns
      version: aardvark-dns 1.12.2
    package: netavark-1.12.2-1.fc40.aarch64
    path: /usr/libexec/podman/netavark
    version: netavark 1.12.2
  ociRuntime:
    name: crun
    package: crun-1.17-1.fc40.aarch64
    path: /usr/bin/crun
    version: |-
      crun version 1.17
      commit: 000fa0d4eeed8938301f3bcf8206405315bc1017
      rundir: /tmp/storage-run-1000/crun
      spec: 1.0.0
      +SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +CRIU +LIBKRUN +WASM:wasmedge +YAJL
  os: linux
  pasta:
    executable: /usr/bin/pasta
    package: passt-0^20240906.g6b38f07-1.fc40.aarch64
    version: |
      pasta 0^20240906.g6b38f07-1.fc40.aarch64-pasta
      Copyright Red Hat
      GNU General Public License, version 2 or later
        <https://www.gnu.org/licenses/old-licenses/gpl-2.0.html>
      This is free software: you are free to change and redistribute it.
      There is NO WARRANTY, to the extent permitted by law.
  remoteSocket:
    exists: false
    path: /tmp/storage-run-1000/podman/podman.sock
  rootlessNetworkCmd: pasta
  security:
    apparmorEnabled: false
    capabilities: CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID,CAP_SYS_CHROOT
    rootless: true
    seccompEnabled: true
    seccompProfilePath: /usr/share/containers/seccomp.json
    selinuxEnabled: false
  serviceIsRemote: false
  slirp4netns:
    executable: ""
    package: ""
    version: ""
  swapFree: 0
  swapTotal: 0
  uptime: 7h 48m 56.00s (Approximately 0.29 days)
  variant: v8
plugins:
  authorization: null
  log:
  - k8s-file
  - none
  - passthrough
  - journald
  network:
  - bridge
  - macvlan
  - ipvlan
  volume:
  - local
registries:
  search:
  - registry.fedoraproject.org
  - registry.access.redhat.com
  - docker.io
store:
  configFile: /my_custom_dir/.config/containers/storage.conf
  containerStore:
    number: 0
    paused: 0
    running: 0
    stopped: 0
  graphDriverName: overlay
  graphOptions: {}
  graphRoot: /my_custom_dir/.local/share/containers/storage
  graphRootAllocated: 61285326848
  graphRootUsed: 4142067712
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "true"
    Supports d_type: "true"
    Supports shifting: "false"
    Supports volatile: "true"
    Using metacopy: "false"
  imageCopyTmpDir: /var/tmp
  imageStore:
    number: 0
  runRoot: /tmp/storage-run-1000/containers
  transientStore: false
  volumePath: /my_custom_dir/.local/share/containers/storage/volumes
version:
  APIVersion: 5.2.3
  Built: 1727136000
  BuiltTime: Tue Sep 24 00:00:00 2024
  GitCommit: ""
  GoVersion: go1.22.7
  Os: linux
  OsArch: linux/arm64
  Version: 5.2.3
$ id
uid=1000(podman) gid=1000(podman) groups=1000(podman)
$ podman build . -t playground-bis:testing
STEP 1/6: FROM docker.io/library/golang:1.23.1 AS builder
Trying to pull docker.io/library/golang:1.23.1...
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

## Run Podman as a non-root user without privileges on vanilla Kubernetes

Before you being, make sure you have:

- A Kubernetes cluster using `CRI-O` as runtime engine.
- Permission to use `fuse-overlayfs` inside of the container.

To run rootless Podman without privileges, follow the steps in [Run Podman as a non-root user with privileges](#run-podman-as-a-non-root-user-with-privileges).

## Run Podman as a non-root user on OpenShift

To run rootless Podman without privileged containers, follow the steps in the RedHat article [Build container images in OpenShift using Podman as a GitLab Runner](https://developers.redhat.com/articles/2024/10/01/build-container-images-openshift-using-podman-gitlab-runner).

## Troubleshooting

### `git` cannot save the configuration in `/.gitconfig` when you run the job as a root user

Because you are not running the job as root, `git` cannot save the configuration in `/.gitconfig`. As a result, you might encounter the following error:

```shell
Getting source from Git repository
00:00
error: could not lock config file //.gitconfig: Permission denied
```

To prevent this error:

1. Mount an `emptyDir` volume on `/my_custom_dir`.
1. Set the `HOME` environment variable in the `/my_custom_dir` path.
