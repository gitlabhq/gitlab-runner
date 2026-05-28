---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Kubernetes 실행기 문제 해결
---

Kubernetes 실행기를 사용할 때 일반적으로 발생하는 오류는 다음과 같습니다.

## `Job failed (system failure): timed out waiting for pod to start` {#job-failed-system-failure-timed-out-waiting-for-pod-to-start}

클러스터가 `poll_timeout`로 정의된 타임아웃 전에 빌드 포드를 스케줄할 수 없으면 빌드 포드는 오류를 반환합니다. [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime)가 포드를 삭제할 수 있어야 합니다.

이 문제를 해결하려면 `config.toml` 파일에서 `poll_timeout` 값을 증가시킵니다.

## `context deadline exceeded` {#context-deadline-exceeded}

`context deadline exceeded` 오류는 일반적으로 Kubernetes API 클라이언트가 지정된 클러스터 API 요청에 대해 타임아웃되었음을 나타냅니다.

[`kube-apiserver` 클러스터 구성 요소의 메트릭](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/)을 확인하여 다음 징후가 있는지 확인하세요:

- 응답 지연 시간 증가
- 포드, 시크릿, ConfigMaps 및 기타 코어(v1) 리소스에 대한 일반적인 생성 또는 삭제 작업의 오류율입니다.

`kube-apiserver` 작업에서 타임아웃 관련 오류에 대한 로그는 다음과 같이 나타날 수 있습니다:

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

경우에 따라 `kube-apiserver` 오류 응답은 하위 구성 요소 장애(예: Kubernetes 클러스터의 `etcdserver`)에 대한 추가 세부 정보를 제공할 수 있습니다:

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

이러한 `kube-apiserver` 서비스 장애는 빌드 포드 생성 중 및 완료 후 정리 시도 중에 발생할 수 있습니다:

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

## `Dial tcp xxx.xx.x.x:xxx: i/o timeout` {#dial-tcp-xxxxxxxxxx-io-timeout}

이것은 일반적으로 Kubernetes API 서버에 러너 관리자가 도달할 수 없음을 나타내는 Kubernetes 오류입니다. 이 문제를 해결하려면:

- 네트워크 보안 정책을 사용하는 경우 Kubernetes API에 대한 액세스 권한을 부여합니다. 일반적으로 포트 443 또는 포트 6443 또는 둘 다입니다.
- Kubernetes API가 실행 중인지 확인합니다.

## Kubernetes API와 통신을 시도할 때 연결이 거부됨 {#connection-refused-when-attempting-to-communicate-with-the-kubernetes-api}

GitLab 러너가 Kubernetes API에 요청을 하고 실패하면 [`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)가 과부하 상태이고 API 요청을 수락하거나 처리할 수 없을 가능성이 높습니다.

## `Error cleaning up pod` 및 `Job failed (system failure): prepare environment: waiting for pod running` {#error-cleaning-up-pod-and-job-failed-system-failure-prepare-environment-waiting-for-pod-running}

다음 오류는 Kubernetes가 작업 포드를 적시에 스케줄하지 못할 때 발생합니다. GitLab 러너는 포드가 준비될 때까지 기다리지만 실패한 후 포드를 정리하려고 시도하며, 이것도 실패할 수 있습니다.

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

문제를 해결하려면 Kubernetes 기본 노드와 [`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver) 인스턴스를 실행하는 모든 노드를 확인합니다. 클러스터에서 규모를 확대하려는 대상 포드 수를 관리하는 데 필요한 모든 리소스가 있는지 확인합니다.

GitLab 러너가 포드가 `Ready` 상태에 도달할 때까지 기다리는 시간을 변경하려면 [`poll_timeout`](_index.md#other-configtoml-settings) 설정을 사용합니다.

준비 스테이지가 총 실행 시간(포드 스케줄링 포함)을 제한하려면 [`prepare_timeout`](../../configuration/advanced-configuration.md#prepare-stage-timeout) 설정을 사용합니다.

포드가 스케줄되는 방법 또는 시간 내에 스케줄되지 않는 이유를 더 잘 이해하려면 [Kubernetes Scheduler에 대해 읽어보세요](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).

## `request did not complete within requested timeout` {#request-did-not-complete-within-requested-timeout}

`request did not complete within requested timeout` 메시지는 빌드 포드 생성 중에 Kubernetes 클러스터에서 구성된 [허용 제어 웹후크](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)가 타임아웃되고 있음을 나타냅니다.

허용 제어 웹후크는 범위가 지정된 모든 API 요청에 대한 클러스터 수준의 관리 제어 인터셉트이며, 제때 실행되지 않으면 오류를 유발할 수 있습니다.

허용 제어 웹후크는 API 요청 및 네임스페이스 원본을 세밀하게 제어할 수 있는 필터를 지원합니다. GitLab 러너의 Kubernetes API 호출이 허용 제어 웹후크를 통과할 필요가 없으면 [웹후크의 선택자/필터 구성](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector)을 변경하여 GitLab 러너 네임스페이스를 무시하거나, `podAnnotations` 또는 `podLabels`를 구성하여 GitLab 러너 포드에 제외 레이블/주석을 적용할 수 있습니다. [GitLab 러너 Helm Chart `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500)

예를 들어 [DataDog 허용 제어 웹후크](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator)가 GitLab 러너 관리자 포드가 수행한 API 요청을 가로채지 않도록 하려면 다음을 추가할 수 있습니다:

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

Kubernetes 클러스터의 허용 제어 웹후크를 나열하려면 다음을 실행합니다:

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

허용 제어 웹후크가 타임아웃될 때 관찰할 수 있는 로그의 형식은 다음과 같습니다:

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

허용 제어 웹후크 장애는 대신 다음과 같이 나타날 수 있습니다:

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

## 오류 `Could not resolve host: example.com` {#error-could-not-resolve-host-examplecom}

`alpine` 형식의 [도우미 이미지](../../configuration/advanced-configuration.md#helper-image)를 사용하는 경우 Alpine의 `musl`의 DNS 리졸버와 관련된 [DNS 문제](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129)가 있을 수 있습니다. 오류는 다음과 유사할 수 있습니다:

- `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

`helper_image_flavor = "ubuntu"` 옵션을 사용하여 이 문제를 해결합니다.

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?` {#docker-cannot-connect-to-the-docker-daemon-at-tcpdocker2375-is-the-docker-daemon-running}

이 오류는 [Docker-in-Docker를 사용](_index.md#using-dockerdind)할 때 DIND 서비스가 완전히 시작될 때까지 기다릴 시간이 충분하지 않은 경우에 발생할 수 있습니다. 더 자세한 설명은 [이 이슈](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215)를 참조하세요.

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443` {#curl-35-openssl-ssl_connect-ssl_error_syscall-in-connection-to-githubcom443}

이 오류는 [Docker-in-Docker를 사용](_index.md#using-dockerdind)할 때 DIND 최대 전송 단위(MTU)가 Kubernetes 오버레이 네트워크보다 큰 경우에 발생할 수 있습니다. DIND는 기본 MTU 1500을 사용하며, 이는 기본 오버레이 네트워크 전체에서 라우팅하기에는 너무 큽니다. DIND MTU는 서비스 정의 내에서 변경할 수 있습니다:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows` {#mountvolumesetup-failed-for-volume-kube-api-access-xxxxx--chown-is-not-supported-by-windows}

CI/CD 작업을 실행할 때 다음과 같은 오류가 나타날 수 있습니다:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

이 문제는 [노드 선택자를 사용](_index.md#specify-the-node-to-execute-builds)하여 서로 다른 운영 체제 및 아키텍처가 있는 노드에서 빌드를 실행할 때 발생합니다.

문제를 해결하려면 `nodeSelector`를 구성하여 러너 관리자 포드가 항상 Linux 노드에서 스케줄되도록 합니다. 예를 들어 [`values.yaml` 파일](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)에는 다음이 포함되어야 합니다:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## 빌드 포드에 러너 IAM 역할 대신 워커 노드의 IAM 역할이 할당됨 {#build-pods-are-assigned-the-worker-nodes-iam-role-instead-of-runner-iam-role}

이 문제는 워커 노드 IAM 역할이 올바른 역할을 가정할 수 있는 권한이 없을 때 발생합니다. 이를 해결하려면 `sts:AssumeRole` 권한을 워커 노드의 IAM 역할의 신뢰 관계에 추가합니다:

```json
{
    "Effect": "Allow",
    "Principal": {
        "AWS": "arn:aws:iam::<AWS_ACCOUNT_NUMBER>:role/<IAM_ROLE_NAME>"
    },
    "Action": "sts:AssumeRole"
}
```

## 오류: `pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies` {#error-pull_policy-always-defined-in-gitlab-pipeline-config-is-not-one-of-the-allowed_pull_policies}

이 문제는 `.gitlab-ci.yml`에서 `pull_policy`를 지정했지만 러너 구성 파일에 구성된 정책이 없을 때 발생합니다. 오류는 다음과 유사할 수 있습니다:

- `Preparation failed: invalid pull policy for image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

이 문제를 해결하려면 `allowed_pull_policies`를 [Docker 가져오기 정책 제한](_index.md#restrict-docker-pull-policies)에 따라 구성에 추가합니다.

## 백그라운드 프로세스로 인해 작업이 중단되고 타임아웃됨 {#background-processes-cause-jobs-to-hang-and-timeout}

작업 실행 중에 시작된 백그라운드 프로세스는 [빌드 작업이 종료되는 것을 방지](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880)할 수 있습니다. 이를 피하려면 다음을 수행할 수 있습니다:

- 프로세스를 이중 포크합니다. 예를 들어, `command_to_run < /dev/null &> /dev/null &`.
- 작업 스크립트를 종료하기 전에 프로세스를 중지합니다.

## 캐시 관련 `permission denied` 오류 {#cache-related-permission-denied-errors}

작업에서 생성된 파일 및 폴더는 특정 UNIX 소유권 및 권한을 가집니다. 파일 및 폴더를 보관하거나 추출할 때 UNIX 세부 정보가 유지됩니다. 그러나 파일 및 폴더는 [도우미 이미지](../../configuration/advanced-configuration.md#helper-image)의 `USER` 구성과 일치하지 않을 수 있습니다.

`Creating cache ...` 단계에서 권한 관련 오류가 발생하면 다음을 수행할 수 있습니다:

- 솔루션으로 원본 데이터가 수정되는지 조사합니다. 예를 들어 캐시된 파일을 생성하는 작업 스크립트에서입니다.
- 해결 방법으로 일치하는 [chown](https://linux.die.net/man/1/chown) 및 [chmod](https://linux.die.net/man/1/chmod) 명령을 추가합니다. [(`before_`/`after_`)`script:` 지시문](https://docs.gitlab.com/ci/yaml/#default)에 추가합니다.

## init 시스템을 사용하는 빌드 컨테이너의 겉으로는 중복된 셸 프로세스 {#apparently-redundant-shell-process-in-build-container-with-init-system}

프로세스 트리는 다음 중 하나일 때 셸 프로세스를 포함할 수 있습니다:

- `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`은 `false`이고 `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR`은 `true`입니다.
- 빌드 이미지의 `ENTRYPOINT`은 init 시스템(`tini-init` 또는 `dumb-init` 등)입니다.

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

`sh`, `bash` 또는 `busybox`일 수 있는 이 셸 프로세스는 `PPID`가 1이고 `PID`가 6 또는 7인 경우 init 시스템(위의 `PID` 1)에서 실행하는 셸 감지 스크립트로 시작된 셸입니다. 이 프로세스는 중복되지 않으며 빌드 컨테이너가 init 시스템으로 실행될 때의 일반적인 작동입니다.

## 러너 포드가 성공적인 등록에도 불구하고 작업 결과를 실행하지 못하고 타임아웃됨 {#runner-pod-fails-to-run-job-results-and-times-out-despite-successful-registration}

러너 포드가 GitLab에 등록한 후 작업을 실행하려고 시도하지만 실패하고 결국 작업이 타임아웃됩니다. 다음 오류가 보고됩니다:

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

이 경우 러너는 다음 오류를 받을 수 있습니다.

```plaintext
HTTP 204 No content response code when connecting to the `jobs/request` API.
```

이 문제를 해결하려면 TCP 연결이 중단되는지 확인하기 위해 API에 POST 요청을 수동으로 보냅니다. TCP 연결이 중단된 경우 러너는 CI 작업 페이로드를 요청할 수 없습니다.

## `failed to reserve container name` init-permissions 컨테이너용(때 `gcs-fuse-csi-driver`가 사용됨) {#failed-to-reserve-container-name-for-init-permissions-container-when-gcs-fuse-csi-driver-is-used}

`gcs-fuse-csi-driver` `csi` 드라이버는 [init 컨테이너의 볼륨 마운트를 지원하지 않습니다](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38). 이것은 이 드라이버를 사용할 때 init 컨테이너 시작 실패를 유발할 수 있습니다. 이 버그를 해결하려면 [Kubernetes 1.28에 도입된](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/) 기능이 드라이버의 프로젝트에서 지원되어야 합니다.

## 오류: `only read-only root filesystem container is allowed` {#error-only-read-only-root-filesystem-container-is-allowed}

읽기 전용 마운트된 루트 파일 시스템에서 실행하도록 강제하는 허용 정책이 있는 클러스터에서 이 오류는 다음과 같은 경우에 나타날 수 있습니다:

- GitLab 러너를 설치합니다.
- GitLab 러너는 빌드 포드를 스케줄하려고 합니다.

이러한 허용 정책은 일반적으로 [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/) 또는 [Kyverno](https://kyverno.io/)와 같은 허용 제어자에 의해 시행됩니다. 예를 들어 컨테이너가 읽기 전용 루트 파일 시스템에서 실행되도록 강제하는 정책은 [`readOnlyRootFilesystem`](https://open-policy-agent.github.io/gatekeeper-library/website/validation/read-only-root-filesystem/) Gatekeeper 정책입니다.

이 문제를 해결하려면:

- 클러스터에 배포된 모든 포드는 `securityContext.readOnlyRootFilesystem`를 `true`로 설정하여 허용 정책을 준수해야 합니다. 이렇게 하면 허용 제어자가 포드를 차단하지 않습니다.
- 루트 파일 시스템이 읽기 전용으로 마운트되더라도 컨테이너는 성공적으로 실행되고 파일 시스템에 쓸 수 있어야 합니다.

### GitLab 러너 {#for-gitlab-runner}

GitLab 러너가 [GitLab 러너 Helm 차트](../../install/kubernetes.md)로 배포된 경우 GitLab 차트 구성을 다음과 같이 업데이트해야 합니다:

- 적절한 `securityContext` 값:

  ```yaml
  <...>
  securityContext:
    readOnlyRootFilesystem: true
  <...>
  ```

- 포드가 쓸 수 있는 쓰기 가능한 파일 시스템이 마운트됨:

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

### 빌드 포드 {#for-the-build-pod}

빌드 포드가 읽기 전용 루트 파일 시스템에서 실행되도록 하려면 `config.toml`에서 다양한 컨테이너의 보안 컨텍스트를 구성합니다. GitLab 차트 변수 `runners.config`를 설정할 수 있습니다. 이는 빌드 포드에 전달됩니다:

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

빌드 포드와 해당 컨테이너가 읽기 전용 파일 시스템에서 성공적으로 실행되도록 하려면 빌드 포드가 쓸 수 있는 위치에 쓰기 가능한 파일 시스템이 있어야 합니다. 최소한 이러한 위치는 빌드 및 홈 디렉토리입니다. 필요한 경우 빌드 프로세스가 다른 위치에 쓰기 액세스 권한이 있는지 확인합니다.

홈 디렉토리는 일반적으로 프로그램이 구성 및 성공적인 실행에 필요한 다른 데이터를 저장할 수 있도록 쓰기 가능해야 합니다. `git` 바이너리는 홈 디렉토리에 쓸 수 있기를 원하는 프로그램의 한 예입니다.

다양한 컨테이너 이미지의 경로에 관계없이 홈 디렉토리를 쓰기 가능하게 만들려면:

1. 안정적인 경로에 볼륨을 마운트합니다(사용하는 빌드 이미지에 관계없이).
1. 환경 변수 `$HOME`를 설정하여 홈 디렉토리를 변경합니다(모든 빌드에 대해 전역).

`config.toml`에서 빌드 포드 및 해당 컨테이너를 구성할 수 있습니다. GitLab 차트 변수 `runners.config`의 값을 업데이트합니다.

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

> [!note]
> `emptyDir` 대신 [지원되는 볼륨 유형](_index.md#configure-volume-types)을 사용할 수 있습니다. 명시적으로 처리되지 않고 빌드 아티팩트로 저장되는 모든 파일이 일반적으로 일시적이므로 `emptyDir`은 대부분의 경우에 작동합니다.

## AWS EKS: 포드 정리 오류: 포드 "runner-\*\*"를 찾을 수 없거나 상태가 "Failed" {#aws-eks-error-cleaning-up-pod-pods-runner--not-found-or-status-is-failed}

Amazon EKS 영역 재조정 기능은 자동 크기 조정 그룹의 가용 영역을 균형 있게 유지합니다. 이 기능은 한 가용 영역의 노드를 중지하고 다른 가용 영역에 생성할 수 있습니다.

러너 작업은 중지할 수 없으며 다른 노드로 이동할 수 없습니다. 러너 작업에 대해 이 기능을 비활성화하여 이 오류를 해결합니다.

## Windows 컨테이너에서 지원되지 않는 서비스 {#services-not-supported-with-windows-containers}

Windows 노드에서 [서비스](https://docs.gitlab.com/ci/services/)를 사용하려고 할 때 다음 오류로 실패할 수 있습니다:

- `ERROR: Job failed (system failure): prepare environment: admission webhook "windows.common-webhooks.networking.gke.io" denied the request: spec.hostAliases: Invalid value: []v1.HostAlias{v1.HostAlias{IP:"127.0.0.1", Hostnames:[]string{"<your windows image>"}}}: Windows does not support this field.`

Kubernetes 런타임에 따라 오류는 보고되거나 자동으로 무시될 수 있습니다. 예를 들어 GKE는 오류를 보고합니다.

서비스는 Kubernetes 실행기에서 `hostAlias`을(를) 사용하여 구현되며 Windows 컨테이너에서 지원되지 않습니다.
