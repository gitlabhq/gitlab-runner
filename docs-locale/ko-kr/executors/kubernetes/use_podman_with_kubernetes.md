---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Kubernetes에서 GitLab Runner와 함께 Podman 사용
---

Podman은 컨테이너를 개발, 관리, 실행하기 위한 오픈 소스 [Open Container Initiative](https://opencontainers.org/) (OCI) 도구입니다.

Podman은 루트 사용자 없이 또는 [privileged](../../security/_index.md#usage-of-docker-executor) 권한 상승 없이 CI 작업에서 컨테이너 이미지를 빌드할 수 있는 구성을 제공합니다.

이 문서는 OpenShift 및 비 OpenShift Kubernetes 클러스터에서 GitLab 러너와 함께 Podman을 구성하는 방법에 대한 정보를 다룹니다. 이 구성은 루트 사용자 및 비 루트 사용자로 설정된 컨테이너 이미지에 적용됩니다.

## 비 OpenShift Kubernetes 클러스터에서 Podman 실행 {#run-podman-on-non-openshift-kubernetes-cluster}

### `--privileged` 플래그를 `true`로 설정하여 비 루트 사용자로 Podman 실행 {#run-podman-as-a-non-root-user-with-the---privileged-flag-set-to-true}

> [!warning]
> `--privileged` 플래그를 `true`로 설정하여 Podman을 실행하면 컨테이너 엔진이 추가 보안 제어 여부와 관계없이 컨테이너를 시작합니다.

비 루트 컨테이너 프로세스를 사용하여 Podman을 비 루트 사용자로 실행하려면:

1. `.gitlab-ci.yml` 파일에서 다음 샘플 코드를 사용하여 Podman으로 컨테이너 이미지를 만듭니다:

   ```yaml
   variables:
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

   또한 기능 플래그를 활성화하여 환경의 러너 동작을 조정할 수 있습니다. 자세한 내용은 [이용 가능한 기능 플래그](../../configuration/feature-flags.md#available-feature-flags)를 참조하세요.

1. 기본값 `user_id`을 `1000`로 설정하고 `config.toml` 파일에 다음 구성을 추가합니다:

   ```ini
       [runners.kubernetes.pod_security_context]
         run_as_user = 1000
       [runners.kubernetes.build_container_security_context]
         run_as_user = 1000
   ```

1. `config.toml` 파일에 다음 러너 구성을 추가합니다:

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
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "repo"
           mount_path = "/my_custom_dir"
       [runners.kubernetes.pod_security_context]
         run_as_user = 1000
       [runners.kubernetes.build_container_security_context]
         run_as_user = 1000
   ```

작업이 예상대로 통과하면 작업 로그는 다음 예제와 같아야 합니다:

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

### `--privileged` 플래그를 `false`로 설정하여 루트 사용자로 Podman 실행 {#run-podman-as-a-root-user-with-the---privileged-flag-set-to-false}

전제 조건:

- 컨테이너 내에서 `fuse-overlayfs`을 사용할 수 있는 권한입니다.

다음 단계는 [Kubernetes 내에서 Podman을 사용하는 방법](https://www.redhat.com/en/blog/podman-inside-kubernetes)의 "권한 없는 Podman(privileged 플래그 제외)" 섹션에서 영감을 받았습니다.

권한 없는 Podman을 실행할 때 시스템 구성을 약간 조정하여 privileged 플래그를 제거할 수 있습니다. 컨테이너가 `/dev/fuse`에 접근해야 컨테이너 내에서 `fuse-overlayfs`를 사용할 수 있습니다.

또한 Kubernetes 클러스터가 실행 중인 호스트에서 SELinux를 비활성화해야 합니다. SELinux는 컨테이너화된 프로세스가 컨테이너 내에서 필요한 파일 시스템을 마운트하는 것을 방지합니다.

이를 수행하려면:

1. 작업 Pod에서 사용할 수 있는 디바이스 플러그인을 만듭니다. 예를 들면:

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

1. `config.toml`을 구성하여 클러스터에 GitLab 러너를 설치합니다.

   - 작업 Pod를 `root` 사용자로 실행하되 `--privileged` 플래그를 `false`로 설정합니다:

     ```toml
     allow_privilege_escalation = false
     [runners.kubernetes.pod_security_context]
       run_as_non_root = false
     [runners.kubernetes.build_container_security_context]
       run_as_user = 0
       run_as_group = 0
     ```

   - [`pod_spec` 기능](_index.md#overwrite-generated-pod-specifications)을 사용하여 작업 Pod에 리소스 제한을 설정합니다. `pod_spec`을 사용하려면 `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` 기능 플래그를 `true`로 설정합니다.

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

   `config.toml`은 다음과 같이 표시되어야 합니다:

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

1. Podman으로 이미지를 빌드하는 작업을 실행합니다.

   ```yaml
   variables:
     FF_USE_ADVANCED_POD_SPEC_CONFIGURATION: "true"

   podman-privileged-test:
     image: quay.io/podman/stable
     before_script:
       - podman info
       - id
     script:
       - podman build . -t playground-bis:testing
   ```

   또한 기능 플래그를 활성화하여 환경의 러너 동작을 조정할 수 있습니다. 자세한 내용은 [이용 가능한 기능 플래그](../../configuration/feature-flags.md#available-feature-flags)를 참조하세요.

작업이 `podman build`을 실행하고 성공적으로 완료되어야 합니다.

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

## OpenShift에서 비 루트 사용자로 Podman 실행 {#run-podman-as-a-non-root-user-on-openshift}

권한 없는 Podman을 권한 없는 컨테이너로 실행하려면 RedHat 문서 [GitLab 러너로서 OpenShift에서 Podman을 사용하여 컨테이너 이미지 빌드](https://developers.redhat.com/articles/2024/10/01/build-container-images-openshift-using-podman-gitlab-runner)의 단계를 따릅니다.

## 문제 해결 {#troubleshooting}

### `git`는 비 루트 사용자로 작업을 실행할 때 `/.gitconfig`에 구성을 저장할 수 없습니다 {#git-cannot-save-the-configuration-in-gitconfig-when-you-run-the-job-as-a-non-root-user}

루트로 작업을 실행하지 않으므로 `git`은 `/.gitconfig`에 구성을 저장할 수 없습니다. 결과적으로 다음 오류가 나타날 수 있습니다:

```shell
Getting source from Git repository
00:00
error: could not lock config file //.gitconfig: Permission denied
```

이 오류를 방지하려면:

1. `emptyDir` 볼륨을 `/my_custom_dir`에 마운트합니다.
1. `HOME` 환경 변수를 `/my_custom_dir` 경로로 설정합니다.
